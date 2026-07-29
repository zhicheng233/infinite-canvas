import axios from "axios";
import { isLoggedIn } from "./ai-proxy";
import { API_BASE } from "./client";

import { notifyCreditBalanceChanged } from "@/constant/credits";
import { dataUrlToFile, getDataUrlByteSize } from "@/lib/image-utils";
import { getMediaBlob, uploadMediaFile, type UploadedFile } from "@/services/file-storage";
import { getImageBlob, imageToDataUrl } from "@/services/image-storage";
import { uploadTempImage, uploadTempMedia } from "@/services/api/temp-media";
import { normalizeBinghuoRatio, normalizeBinghuoReferenceMode, normalizeBinghuoResolution } from "@/lib/binghuo-video";
import { boolConfig, buildSeedancePromptText, isSeedanceVideoConfig, normalizeSeedanceDuration, normalizeSeedanceRatio, normalizeSeedanceResolution, seedanceVideoReferenceError, SEEDANCE_REFERENCE_LIMITS } from "@/lib/seedance-video";
import { buildApiUrl, buildProxyApiUrl, modelOptionName, normalizeVideoDurationForModel, readLocalAiCredentials, resolveModelRequestConfig, videoRouteForModel, type AiConfig } from "@/stores/use-config-store";
import type { ReferenceImage } from "@/types/image";
import type { ReferenceAudio, ReferenceVideo } from "@/types/media";

type VideoResponse = { id?: string; task_id?: string; status?: string; url?: string; video_url?: string; result_url?: string; output?: string[]; video?: { url?: string } | null; error?: { message?: string } };
type ApiVideoResponse = VideoResponse | { code?: number; data?: VideoResponse | null; msg?: string };
type XAIVideoTask = {
    id?: string;
    request_id?: string;
    status?: string;
    progress?: number;
    error?: { code?: string; message?: string } | null;
    video?: { url?: string } | null;
};
type NewApiVideoTask = {
    task_id?: string;
    id?: string;
    status?: string;
    state?: string;
    task_status?: string;
    success?: boolean;
    message?: string;
    url?: string;
    video_url?: string;
    result_url?: string;
    output?: string[];
    format?: string;
    original_watermarked_video_url?: string;
    metadata?: {
        original_watermarked_video_url?: string;
        result_url?: string;
        result_urls?: string[];
        url?: string;
        video_url?: string;
    } | null;
    video?: { url?: string; duration?: number } | null;
    error?: string | { code?: string | number; message?: string } | null;
    data?: NewApiVideoTask | null;
};
type SeedanceTask = {
    id: string;
    status?: "queued" | "running" | "succeeded" | "failed" | "cancelled" | "expired";
    error?: { code?: string; message?: string } | null;
    content?: { video_url?: string; last_frame_url?: string } | null;
};
type ApiEnvelope<T> = T | { code?: number | string; ok?: boolean; data?: T | null; msg?: string; message?: string; error?: string | { message?: string }; error_detail?: string };
type RequestOptions = { signal?: AbortSignal };

class ExplicitVideoDownloadError extends Error {}

export type VideoGenerationResult = { blob?: Blob; url?: string; mimeType?: string };
export type VideoGenerationTask = { id: string; provider: "openai" | "seedance" | "xai" | "newapi" | "yijia" | "binghuo"; model: string; channelId?: number; channelModelId?: number };
export type VideoGenerationTaskState = { status: "pending" } | { status: "completed"; result: VideoGenerationResult } | { status: "failed"; error: string };
type VideoTaskRouting = Pick<VideoGenerationTask, "model" | "channelId" | "channelModelId">;

function aiApiUrl(config: AiConfig, path: string, routing?: VideoTaskRouting) {
    if (isLoggedIn()) {
        const model = routing?.model || config.model || config.videoModel;
        return buildProxyApiUrl(API_BASE, config, model, path, {
            channelId: routing?.channelId,
            channelModelId: routing?.channelModelId,
            routingModel: modelOptionName(model),
            routingVideoRoute: videoRouteForModel(config, model),
        });
    }
    return buildApiUrl(readLocalAiCredentials().baseUrl, path);
}

function resolvedTaskRouting(headers: unknown) {
    const values = (headers || {}) as Record<string, unknown>;
    const channelId = Number(values["x-resolved-channel-id"]);
    const channelModelId = Number(values["x-resolved-channel-model-id"]);
    return channelId > 0 && channelModelId > 0 ? { channelId, channelModelId } : {};
}

function aiHeaders(config: AiConfig, contentType?: string) {
    const headers: Record<string, string> = {};
    if (isLoggedIn()) {
        const token = typeof window !== "undefined" ? localStorage.getItem("infinite-canvas:auth_token") : null;
        if (token) headers["Authorization"] = "Bearer " + token;
    } else {
        headers["Authorization"] = "Bearer " + readLocalAiCredentials().apiKey;
    }
    if (contentType) headers["Content-Type"] = contentType;
    return headers;
}

export async function requestVideoGeneration(config: AiConfig, prompt: string, references: ReferenceImage[] = [], videoReferences: ReferenceVideo[] = [], audioReferences: ReferenceAudio[] = [], options?: RequestOptions): Promise<VideoGenerationResult> {
    const task = await createVideoGenerationTask(config, prompt, references, videoReferences, audioReferences, options);
    const delayMs = task.provider === "seedance" ? 5000 : 2500;
    const maxAttempts = task.provider === "seedance" ? 180 : 240;
    for (let attempt = 0; attempt < maxAttempts; attempt += 1) {
        if (options?.signal?.aborted) throw new DOMException("Aborted", "AbortError");
        const state = await pollVideoGenerationTask(config, task, options);
        if (state.status === "completed") return state.result;
        if (state.status === "failed") throw new Error(state.error);
        if (attempt === maxAttempts - 1) throw new Error(`${task.provider === "seedance" ? "Seedance " : ""}视频生成超时，请稍后重试`);
        await delay(delayMs, options?.signal);
    }
    throw new Error("视频生成超时，请稍后重试");
}

export async function createVideoGenerationTask(config: AiConfig, prompt: string, references: ReferenceImage[] = [], videoReferences: ReferenceVideo[] = [], audioReferences: ReferenceAudio[] = [], options?: RequestOptions): Promise<VideoGenerationTask> {
    const selectedModel = (config.model || config.videoModel).trim();
    const requestConfig = resolveModelRequestConfig(config, selectedModel);
    const localBaseUrl = isLoggedIn() ? "" : readLocalAiCredentials().baseUrl;
    assertVideoConfig(requestConfig, requestConfig.model);
    const configuredRoute = videoRouteForModel(requestConfig, selectedModel);
    if (configuredRoute !== "auto") {
        if (configuredRoute !== "seedance" && configuredRoute !== "binghuo" && (videoReferences.length || audioReferences.length)) {
            throw new Error("当前视频模型暂不支持参考视频或参考音频，请仅保留参考图片");
        }
        if (configuredRoute === "openai") return createOpenAIVideoTask(requestConfig, selectedModel, prompt, references, options);
        if (configuredRoute === "veo_json") return createVeoJsonVideoTask(requestConfig, selectedModel, prompt, references, options);
        if (configuredRoute === "waninter") return createWaninterVideoTask(requestConfig, selectedModel, prompt, references, options);
        if (configuredRoute === "yijia") return createYijiaVideoTask(requestConfig, selectedModel, prompt, references, options);
        if (configuredRoute === "xai") return createXAIVideoTask(requestConfig, selectedModel, prompt, references, options);
        if (configuredRoute === "newapi") return createNewApiVideoTask(requestConfig, selectedModel, prompt, references, options);
        if (configuredRoute === "seedance") return createSeedanceTask(requestConfig, selectedModel, prompt, references, videoReferences, audioReferences, options);
        if (configuredRoute === "binghuo") return createBinghuoVideoTask(requestConfig, selectedModel, prompt, references, videoReferences, audioReferences, options);
    }
    if (isSeedanceVideoConfig(requestConfig)) {
        return createSeedanceTask(requestConfig, selectedModel, prompt, references, videoReferences, audioReferences, options);
    }
    if (isXAIVideoModel(selectedModel, localBaseUrl)) {
        if (videoReferences.length || audioReferences.length) {
            throw new Error("当前视频模型暂不支持参考视频或参考音频，请仅保留参考图片");
        }
        return createXAIVideoTask(requestConfig, selectedModel, prompt, references, options);
    }
    if (isNewApiVideoGenerationModel(selectedModel, localBaseUrl)) {
        if (videoReferences.length || audioReferences.length) {
            throw new Error("当前视频模型暂不支持参考视频或参考音频，请仅保留参考图片");
        }
        return createNewApiVideoTask(requestConfig, selectedModel, prompt, references, options);
    }
    if (videoReferences.length || audioReferences.length) {
        throw new Error("当前视频接口不支持参考视频或参考音频，请切换到 Seedance 2.0 / 火山 Agent Plan 模型，或移除参考素材");
    }
    return createOpenAIVideoTask(requestConfig, selectedModel, prompt, references, options);
}

export async function pollVideoGenerationTask(config: AiConfig, task: VideoGenerationTask, options?: RequestOptions): Promise<VideoGenerationTaskState> {
    const requestConfig = resolveModelRequestConfig(config, task.model);
    assertVideoConfig(requestConfig, requestConfig.model);
    if (task.provider === "seedance") return pollSeedanceTask(requestConfig, task, options);
    if (task.provider === "xai") return pollXAIVideoTask(requestConfig, task, options);
    if (task.provider === "newapi" || task.provider === "binghuo") return pollNewApiVideoTask(requestConfig, task, options);
    if (task.provider === "yijia") return pollYijiaVideoTask(requestConfig, task, options);
    return pollOpenAIVideoTask(requestConfig, task, options);
}

export async function storeGeneratedVideo(result: VideoGenerationResult): Promise<UploadedFile> {
    if (result.blob) return uploadMediaFile(await normalizeVideoBlob(result.blob), "video");
    if (result.url) return { url: result.url, storageKey: "", bytes: 0, mimeType: result.mimeType || "video/mp4" };
    throw new Error("视频接口没有返回可播放的视频");
}

async function createOpenAIVideoTask(config: AiConfig, model: string, prompt: string, references: ReferenceImage[], options?: RequestOptions): Promise<VideoGenerationTask> {
    if (references.length > 1) throw new Error("当前视频模型只支持单张参考图");
    const body: Record<string, unknown> = {
        model: modelOptionName(model),
        prompt,
        input_reference: "",
        size: normalizeVideoSize(config.size) || "1280x720",
    };
    if (references[0]) body.input_reference = await resolveYijiaInputReference(references[0]);
    try {
        const response = await axios.post<ApiVideoResponse>(aiApiUrl(config, "/videos", { model }), body, { headers: aiHeaders(config, "application/json"), signal: options?.signal });
        const created = unwrapVideoResponse(response.data);
        const taskId = created.id || created.task_id;
        if (!taskId) throw new Error("视频接口没有返回任务 ID");
        notifyCreditBalanceChanged();
        return { id: taskId, provider: "openai", model, ...resolvedTaskRouting(response.headers) };
    } catch (error) {
        throw new Error(readAxiosError(error, "视频任务创建失败"));
    }
}

async function createXAIVideoTask(config: AiConfig, model: string, prompt: string, references: ReferenceImage[], options?: RequestOptions): Promise<VideoGenerationTask> {
    const payload: Record<string, unknown> = {
        model: modelOptionName(model),
    };
    const seconds = normalizeVideoSecondsForModel(config, model, config.videoSeconds);
    if (seconds) payload.duration = Number(seconds);
    const ratio = normalizeXAIVideoAspectRatio(config.size);
    if (ratio) payload.aspect_ratio = ratio;
    const resolution = normalizeVideoResolution(config.vquality);
    if (resolution) payload.resolution = resolution;
    const imageUrls = await Promise.all(
        references.map(async (image) => {
            const dataUrl = await imageToDataUrl(image);
            return dataUrl || image.url || "";
        }),
    ).then((items) => items.filter(Boolean) as string[]);
    if (imageUrls.length === 1) {
        payload.prompt = { image: imageUrls[0], text: prompt };
    } else {
        payload.prompt = prompt;
        if (imageUrls.length > 1) payload.reference_images = imageUrls.slice(0, 7).map((url) => ({ url }));
    }
    try {
        const response = await axios.post<ApiEnvelope<XAIVideoTask>>(aiApiUrl(config, "/videos/generations", { model }), payload, { headers: aiHeaders(config, "application/json"), signal: options?.signal });
        const created = unwrapXAIVideoTask(response.data);
        const requestId = created.request_id || created.id;
        if (!requestId) throw new Error("视频接口没有返回任务 ID");
        notifyCreditBalanceChanged();
        return { id: requestId, provider: "xai", model, ...resolvedTaskRouting(response.headers) };
    } catch (error) {
        throw new Error(readAxiosError(error, "视频任务创建失败"));
    }
}

async function createVeoJsonVideoTask(config: AiConfig, model: string, prompt: string, references: ReferenceImage[], options?: RequestOptions): Promise<VideoGenerationTask> {
    const payload: Record<string, unknown> = {
        model: modelOptionName(model),
        prompt,
        duration: Number(normalizeVideoSecondsForModel(config, model, config.videoSeconds)),
    };
    const aspectRatio = normalizeVideoAspectRatio(config.size);
    if (aspectRatio) payload.aspect_ratio = aspectRatio;
    const imageUrls = await Promise.all(references.slice(0, 7).map((image) => resolveVeoIngredientImage(image))).then((items) => items.filter(Boolean) as string[]);
    if (imageUrls.length) payload.Ingredients_images = imageUrls;
    try {
        const response = await axios.post<ApiVideoResponse>(aiApiUrl(config, "/videos", { model }), payload, { headers: aiHeaders(config, "application/json"), signal: options?.signal });
        const created = unwrapVideoResponse(response.data);
        const taskId = created.id || created.task_id;
        if (!taskId) throw new Error("视频接口没有返回任务 ID");
        notifyCreditBalanceChanged();
        return { id: taskId, provider: "openai", model, ...resolvedTaskRouting(response.headers) };
    } catch (error) {
        throw new Error(readAxiosError(error, "视频任务创建失败"));
    }
}

async function createWaninterVideoTask(config: AiConfig, model: string, prompt: string, references: ReferenceImage[], options?: RequestOptions): Promise<VideoGenerationTask> {
    const seconds = normalizeVideoSecondsForModel(config, model, config.videoSeconds);
    const payload: Record<string, unknown> = {
        model: modelOptionName(model),
        prompt,
        seconds,
        duration: Number(seconds),
        size: normalizeVideoSize(config.size) || "1280x720",
        aspect_ratio: normalizeVideoAspectRatio(config.size),
        resolution: normalizeVideoResolution(config.vquality),
    };
    if (boolConfig(config.videoGenerateAudio, false)) payload.generate_audio = true;
    const imageUrls = await Promise.all(references.slice(0, 7).map((image) => resolveVeoIngredientImage(image))).then((items) => items.filter(Boolean) as string[]);
    if (imageUrls.length) {
        if (isWaninterVeoStyleModel(model)) {
            payload.Ingredients_images = imageUrls;
        } else {
            payload.images = imageUrls;
        }
    }
    try {
        const response = await axios.post<ApiVideoResponse>(aiApiUrl(config, "/videos", { model }), payload, { headers: aiHeaders(config, "application/json"), signal: options?.signal });
        const created = unwrapVideoResponse(response.data);
        const taskId = created.id || created.task_id;
        if (!taskId) throw new Error("瑙嗛鎺ュ彛娌℃湁杩斿洖浠诲姟 ID");
        notifyCreditBalanceChanged();
        return { id: taskId, provider: "openai", model, ...resolvedTaskRouting(response.headers) };
    } catch (error) {
        throw new Error(readAxiosError(error, "瑙嗛浠诲姟鍒涘缓澶辫触"));
    }
}

async function createYijiaVideoTask(config: AiConfig, model: string, prompt: string, references: ReferenceImage[], options?: RequestOptions): Promise<VideoGenerationTask> {
    const payload: Record<string, unknown> = {
        model: modelOptionName(model),
        prompt,
        input_reference: "",
        size: normalizeVideoSize(config.size) || "1280x720",
    };
    if (references.length > 1) throw new Error("当前视频模型只支持单张参考图");
    if (references[0]) payload.input_reference = await resolveYijiaInputReference(references[0]);
    try {
        const response = await axios.post<ApiVideoResponse>(aiApiUrl(config, "/videos", { model }), payload, { headers: aiHeaders(config, "application/json"), signal: options?.signal });
        const created = unwrapVideoResponse(response.data);
        const taskId = created.id || created.task_id;
        if (!taskId) throw new Error("视频接口没有返回任务 ID");
        notifyCreditBalanceChanged();
        return { id: taskId, provider: "yijia", model, ...resolvedTaskRouting(response.headers) };
    } catch (error) {
        throw new Error(readAxiosError(error, "视频任务创建失败"));
    }
}

async function createNewApiVideoTask(config: AiConfig, model: string, prompt: string, references: ReferenceImage[], options?: RequestOptions): Promise<VideoGenerationTask> {
    const payload: Record<string, unknown> = {
        model: modelOptionName(model),
        prompt,
        duration: Number(normalizeVideoSecondsForModel(config, model, config.videoSeconds)),
    };
    const size = normalizeVideoSize(config.size);
    const match = size?.match(/^(\d+)x(\d+)$/);
    if (match) {
        payload.width = Number(match[1]);
        payload.height = Number(match[2]);
    }
    if (references.length > 1) throw new Error("当前视频模型只支持单张参考图");
    if (references[0]) {
        const dataUrl = await imageToDataUrl(references[0]);
        payload.image = dataUrl || references[0].url || "";
    }
    try {
        const response = await axios.post<ApiEnvelope<NewApiVideoTask>>(aiApiUrl(config, "/video/generations", { model }), payload, { headers: aiHeaders(config, "application/json"), signal: options?.signal });
        const created = unwrapNewApiVideoTask(response.data);
        const taskId = created.task_id || created.id;
        if (!taskId) throw new Error("视频接口没有返回任务 ID");
        notifyCreditBalanceChanged();
        return { id: taskId, provider: "newapi", model, ...resolvedTaskRouting(response.headers) };
    } catch (error) {
        throw new Error(readAxiosError(error, "视频任务创建失败"));
    }
}

async function createBinghuoVideoTask(config: AiConfig, model: string, prompt: string, references: ReferenceImage[], videoReferences: ReferenceVideo[], audioReferences: ReferenceAudio[], options?: RequestOptions): Promise<VideoGenerationTask> {
    const mode = normalizeBinghuoReferenceMode(config.videoReferenceMode);
    if (mode === "first_last" && references.length !== 2) throw new Error("首尾帧模式需要恰好两张参考图，并按首帧、尾帧顺序排列");
    const payload: Record<string, unknown> = {
        model: modelOptionName(model),
        prompt,
        duration: Number(normalizeVideoSecondsForModel(config, model, config.videoSeconds)),
        ratio: normalizeBinghuoRatio(config.size),
        resolution: normalizeBinghuoResolution(config.vquality),
        generate_audio: boolConfig(config.videoGenerateAudio, true),
        n: 1,
    };
    const images = await Promise.all(references.slice(0, 9).map(resolveBinghuoImageUrl));
    if (mode === "first_last") {
        payload.start_frame = [images[0]];
        payload.end_frame = [images[1]];
    } else if (images.length) {
        payload.images = images;
    }
    const referenceVideos = await Promise.all(videoReferences.slice(0, 3).map(resolveBinghuoVideoUrl));
    const referenceAudios = await Promise.all(audioReferences.slice(0, 3).map(resolveBinghuoAudioUrl));
    if (referenceVideos.length) payload.reference_videos = referenceVideos;
    if (referenceAudios.length) payload.reference_audios = referenceAudios;
    try {
        const response = await axios.post<ApiEnvelope<NewApiVideoTask>>(aiApiUrl(config, "/video/generations", { model }), payload, { headers: aiHeaders(config, "application/json"), signal: options?.signal });
        const created = unwrapNewApiVideoTask(response.data);
        const taskId = created.task_id || created.id;
        if (!taskId) throw new Error("炳火 API 没有返回任务 ID");
        notifyCreditBalanceChanged();
        return { id: taskId, provider: "binghuo", model, ...resolvedTaskRouting(response.headers) };
    } catch (error) {
        throw new Error(readAxiosError(error, "炳火视频任务创建失败"));
    }
}

async function resolveBinghuoImageUrl(image: ReferenceImage) {
    const direct = String(image.url || image.dataUrl || "").trim();
    if (isPublicMediaUrl(direct)) return direct;
    const dataUrl = await imageToDataUrl(image);
    if (!dataUrl) throw new Error("参考图读取失败，无法上传到临时媒体服务");
    return uploadBinghuoMedia(dataUrlToFile({ ...image, dataUrl }));
}

async function resolveBinghuoVideoUrl(video: ReferenceVideo) {
    if (isPublicMediaUrl(video.url)) return video.url;
    const blob = await localReferenceBlob(video.url, video.storageKey);
    if (!blob) throw new Error("参考视频读取失败，无法上传到临时媒体服务");
    return uploadBinghuoMedia(new File([blob], video.name || "reference.mp4", { type: video.type || blob.type || "video/mp4" }));
}

async function resolveBinghuoAudioUrl(audio: ReferenceAudio) {
    if (isPublicMediaUrl(audio.url)) return audio.url;
    const blob = await localReferenceBlob(audio.url, audio.storageKey);
    if (!blob) throw new Error("参考音频读取失败，无法上传到临时媒体服务");
    return uploadBinghuoMedia(new File([blob], audio.name || "reference.mp3", { type: audio.type || blob.type || "audio/mpeg" }));
}

async function localReferenceBlob(url?: string, storageKey?: string) {
    if (storageKey) {
        const stored = await getMediaBlob(storageKey);
        if (stored) return stored;
    }
    if (url?.startsWith("blob:") || url?.startsWith("data:")) return (await fetch(url)).blob();
    return null;
}

async function uploadBinghuoMedia(file: File) {
    try {
        const result = await uploadTempMedia(file);
        if (!isPublicMediaUrl(result.url)) throw new Error("临时媒体服务未返回公网 HTTP(S) 地址，请配置 PUBLIC_BASE_URL 后重试");
        return result.url;
    } catch (error) {
        if (error instanceof Error && error.message.includes("公网 HTTP(S)")) throw error;
        throw new Error("临时媒体上传失败，请检查媒体大小、类型和 PUBLIC_BASE_URL 配置");
    }
}

async function pollYijiaVideoTask(config: AiConfig, task: VideoGenerationTask, options?: RequestOptions): Promise<VideoGenerationTaskState> {
    return pollOpenAIVideoTask(config, task, options);
}

async function pollOpenAIVideoTask(config: AiConfig, task: VideoGenerationTask, options?: RequestOptions): Promise<VideoGenerationTaskState> {
    try {
        const video = unwrapVideoResponse((await axios.get<ApiVideoResponse>(aiApiUrl(config, `/videos/${task.id}`, task), { headers: aiHeaders(config), signal: options?.signal })).data);
        const status = String(video.status || "").toLowerCase();
        if (status === "completed" || status === "succeeded" || status === "done") {
            const result = await resolveVideoTaskResult(config, video as NewApiVideoTask, options, task);
            if (result) return { status: "completed", result };
            const content = await axios.get<Blob>(aiApiUrl(config, `/videos/${task.id}/content`, task), { headers: aiHeaders(config), responseType: "blob", signal: options?.signal });
            return { status: "completed", result: { blob: await normalizeVideoBlob(content.data) } };
        }
        if (status === "failed" || status === "cancelled" || status === "error") return { status: "failed", error: video.error?.message || "视频生成失败" };
        return { status: "pending" };
    } catch (error) {
        throw new Error(readAxiosError(error, "视频任务查询失败"));
    }
}

async function pollXAIVideoTask(config: AiConfig, task: VideoGenerationTask, options?: RequestOptions): Promise<VideoGenerationTaskState> {
    try {
        const state = unwrapXAIVideoTask((await axios.get<ApiEnvelope<XAIVideoTask>>(aiApiUrl(config, `/videos/${task.id}`, task), { headers: aiHeaders(config), signal: options?.signal })).data);
        const status = String(state.status || "").toLowerCase();
        if (status === "done" || status === "completed" || status === "succeeded") {
            const url = state.video?.url;
            if (!url) return { status: "failed", error: "视频生成成功但没有返回视频地址" };
            return { status: "completed", result: await videoResultFromUrl(config, url, options, task) };
        }
        if (status === "failed" || status === "cancelled" || status === "error") {
            return { status: "failed", error: videoTaskErrorMessage(state.error) || state.message || "视频生成失败" };
        }
        return { status: "pending" };
    } catch (error) {
        throw new Error(readAxiosError(error, "视频任务查询失败"));
    }
}

async function pollNewApiVideoTask(config: AiConfig, task: VideoGenerationTask, options?: RequestOptions): Promise<VideoGenerationTaskState> {
    try {
        const state = unwrapNewApiVideoTask((await axios.get<ApiEnvelope<NewApiVideoTask>>(aiApiUrl(config, `/video/generations/${task.id}`, task), { headers: aiHeaders(config), signal: options?.signal })).data);
        const status = String(state.status || "").toLowerCase();
        if (status === "completed" || status === "succeeded" || status === "done") {
            const result = await resolveVideoTaskResult(config, state, options, task);
            if (result) return { status: "completed", result };
            return { status: "failed", error: "视频生成成功但没有返回可播放的视频地址" };
        }
        if (status === "failed" || status === "cancelled" || status === "error") {
            return { status: "failed", error: videoTaskErrorMessage(state.error) || state.message || "视频生成失败" };
        }
        return { status: "pending" };
    } catch (error) {
        throw new Error(readAxiosError(error, "视频任务查询失败"));
    }
}

async function createSeedanceTask(config: AiConfig, model: string, prompt: string, references: ReferenceImage[], videoReferences: ReferenceVideo[], audioReferences: ReferenceAudio[], options?: RequestOptions): Promise<VideoGenerationTask> {
    if (audioReferences.length && !references.length && !videoReferences.length) {
        throw new Error("Seedance 参考音频不能单独使用，请同时添加参考图或参考视频");
    }
    assertSeedanceVideoReferences(videoReferences);
    assertSeedanceAudioReferences(audioReferences);
    const content = await buildSeedanceContent(config, prompt, references, videoReferences, audioReferences);
    if (!content.length) throw new Error("请输入视频提示词，或连接参考图片/视频/音频");
    const payload = {
        model: modelOptionName(model),
        content,
        ratio: normalizeSeedanceRatio(config.size),
        resolution: normalizeSeedanceResolution(config.vquality, modelOptionName(model)),
        duration: normalizeSeedanceDuration(config.videoSeconds),
        generate_audio: boolConfig(config.videoGenerateAudio, true),
        watermark: boolConfig(config.videoWatermark, false),
    };

    try {
        const response = await axios.post<ApiEnvelope<SeedanceTask>>(seedanceApiUrl(config, model), payload, { headers: aiHeaders(config, "application/json"), signal: options?.signal });
        const created = unwrapSeedanceTask(response.data);
        if (!created.id) throw new Error("Seedance 接口没有返回任务 ID");
        notifyCreditBalanceChanged();
        return { id: created.id, provider: "seedance", model, ...resolvedTaskRouting(response.headers) };
    } catch (error) {
        throw new Error(readAxiosError(error, "Seedance 任务创建失败"));
    }
}

async function pollSeedanceTask(config: AiConfig, task: VideoGenerationTask, options?: RequestOptions): Promise<VideoGenerationTaskState> {
    try {
        const state = unwrapSeedanceTask((await axios.get<ApiEnvelope<SeedanceTask>>(seedanceApiUrl(config, task.model, task.id, task), { headers: aiHeaders(config), signal: options?.signal })).data);
        if (state.status === "succeeded") {
            const url = state.content?.video_url;
            if (!url) return { status: "failed", error: "Seedance 任务成功但没有返回视频 URL" };
            return { status: "completed", result: await videoResultFromUrl(config, url, options, task) };
        }
        if (state.status === "failed" || state.status === "cancelled" || state.status === "expired") return { status: "failed", error: state.error?.message || `Seedance 视频生成${state.status === "expired" ? "超时" : "失败"}` };
        return { status: "pending" };
    } catch (error) {
        throw new Error(readAxiosError(error, "Seedance 任务查询失败"));
    }
}

function assertSeedanceVideoReferences(videoReferences: ReferenceVideo[]) {
    const error = seedanceVideoReferenceError(videoReferences);
    if (error) throw new Error(error);
    let total = 0;
    for (const video of videoReferences) {
        if (!video.durationMs) continue;
        if (video.durationMs < 2000 || video.durationMs > 15000) throw new Error("Seedance 参考视频单个时长需要在 2-15 秒之间");
        total += video.durationMs;
    }
    if (total > 15000) throw new Error("Seedance 参考视频总时长不能超过 15 秒");
}

function assertSeedanceAudioReferences(audioReferences: ReferenceAudio[]) {
    let total = 0;
    for (const audio of audioReferences) {
        if (!audio.durationMs) continue;
        if (audio.durationMs < 2000 || audio.durationMs > 15000) throw new Error("Seedance 参考音频单个时长需要在 2-15 秒之间");
        total += audio.durationMs;
    }
    if (total > 15000) throw new Error("Seedance 参考音频总时长不能超过 15 秒");
}

function seedanceApiUrl(config: AiConfig, model: string, taskId?: string, routing?: VideoTaskRouting) {
    return aiApiUrl(config, `/contents/generations/tasks${taskId ? `/${encodeURIComponent(taskId)}` : ""}`, routing || { model });
}

async function buildSeedanceContent(config: AiConfig, prompt: string, references: ReferenceImage[], videoReferences: ReferenceVideo[], audioReferences: ReferenceAudio[]) {
    const content: Array<Record<string, unknown>> = [];
    const text = buildSeedancePromptText(prompt, references, videoReferences, audioReferences);
    if (text) content.push({ type: "text", text });
    for (const image of references.slice(0, SEEDANCE_REFERENCE_LIMITS.images)) {
        content.push({ type: "image_url", image_url: { url: await resolveSeedanceImageUrl(config, image) }, role: "reference_image" });
    }
    for (const video of videoReferences.slice(0, SEEDANCE_REFERENCE_LIMITS.videos)) {
        content.push({ type: "video_url", video_url: { url: await resolveSeedanceVideoUrl(video) }, role: "reference_video" });
    }
    for (const audio of audioReferences.slice(0, SEEDANCE_REFERENCE_LIMITS.audios)) {
        content.push({ type: "audio_url", audio_url: { url: await resolveSeedanceAudioUrl(audio) }, role: "reference_audio" });
    }
    return content;
}

async function resolveSeedanceImageUrl(config: AiConfig, image: ReferenceImage) {
    const directUrl = image.url || image.dataUrl;
    if (isPublicMediaUrl(directUrl) || directUrl.startsWith("asset://")) return directUrl;
    const dataUrl = await imageToDataUrl(image);
    if (!dataUrl) throw new Error("参考图读取失败，请换一张图片或重新上传");
    return dataUrl;
}

async function resolveSeedanceVideoUrl(video: ReferenceVideo) {
    if (isPublicMediaUrl(video.url) || video.url.startsWith("asset://")) return video.url;
    let blob: Blob | null = null;
    if (video.storageKey) blob = await getMediaBlob(video.storageKey);
    if (!blob && video.url?.startsWith("blob:")) blob = await (await fetch(video.url)).blob();
    if (!blob) throw new Error("参考视频必须是公网 URL、素材 ID，或本地已保存的视频");
    return blobToDataUrl(blob);
}

async function resolveSeedanceAudioUrl(audio: ReferenceAudio) {
    if (isPublicMediaUrl(audio.url) || audio.url.startsWith("asset://")) return audio.url;
    let blob: Blob | null = null;
    if (audio.storageKey) blob = await getMediaBlob(audio.storageKey);
    if (!blob && audio.url?.startsWith("blob:")) blob = await (await fetch(audio.url)).blob();
    if (!blob) throw new Error("参考音频必须是公网 URL、素材 ID，或本地已保存的音频");
    return blobToDataUrl(blob);
}

async function videoResultFromUrl(config: AiConfig, url: string, options?: RequestOptions, task?: VideoGenerationTask): Promise<VideoGenerationResult> {
    try {
        return await fetchVideoResult(config, url, options, task);
    } catch (error) {
        if (axios.isCancel(error) || options?.signal?.aborted) throw error;
        throw error;
    }
}

async function resolveVideoTaskResult(config: AiConfig, state: Partial<NewApiVideoTask>, options?: RequestOptions, task?: VideoGenerationTask) {
    let lastError: unknown;
    for (const url of readVideoTaskUrls(state)) {
        try {
            return await videoResultFromUrl(config, url, options, task);
        } catch (error) {
            lastError = error;
            continue;
        }
    }
    if (lastError) throw lastError;
    return null;
}

async function fetchVideoResult(config: AiConfig, url: string, options?: RequestOptions, task?: VideoGenerationTask): Promise<VideoGenerationResult> {
    const errors: string[] = [];
    const target = toProxyableVideoPath(url);
    if (!target) throw new Error("视频成片地址不是 HTTP(S) URL");
    if (isLoggedIn() && shouldUseBackendVideoProxy(target)) {
        try {
            return { blob: await readAxiosVideoBlob(axios.get<Blob>(aiApiUrl(config, target, task), { headers: aiHeaders(config), signal: options?.signal, responseType: "blob" })) };
        } catch (error) {
            if (axios.isCancel(error) || options?.signal?.aborted) throw error;
            errors.push(errorMessage(error));
        }
    }
    try {
        return { blob: await fetchVideoBlobViaNextProxy(target, options) };
    } catch (error) {
        if (options?.signal?.aborted) throw error;
        if (!shouldUseBackendVideoProxy(target)) {
            try {
                assertDownloadErrorCanFallback(error);
                return { url: target, mimeType: "video/mp4" };
            } catch (fallbackError) {
                if (options?.signal?.aborted) throw fallbackError;
                errors.push(errorMessage(fallbackError));
            }
        } else {
            errors.push(errorMessage(error));
        }
    }
    throw new Error(`视频成片下载失败，请检查上游成片地址或代理配置${errors.length ? `：${errors.filter(Boolean).join("；")}` : ""}`);
}

function toProxyableVideoPath(url: string) {
    try {
        const target = new URL(url);
        if (target.protocol !== "http:" && target.protocol !== "https:") return "";
        return target.toString();
    } catch {
        return "";
    }
}

async function readAxiosVideoBlob(request: Promise<{ data: Blob }>) {
    const response = await request;
    return normalizeVideoBlob(response.data);
}

async function fetchVideoBlobViaNextProxy(url: string, options?: RequestOptions) {
    const target = toProxyableVideoPath(url);
    if (!target) throw new Error("视频成片地址不是 HTTP(S) URL");
    const response = await fetch("/webdav-proxy", {
        method: "POST",
        headers: {
            "x-webdav-target": target,
            "x-webdav-method": "GET",
        },
        signal: options?.signal,
    });
    const blob = await response.blob();
    if (!response.ok) throw new Error(`Next.js 代理下载失败（${response.status}）`);
    return normalizeVideoBlob(blob);
}

function shouldUseBackendVideoProxy(url: string) {
    try {
        return /\/v1\/videos\/[^/]+\/content$/i.test(new URL(url).pathname);
    } catch {
        return false;
    }
}

function assertDownloadErrorCanFallback(error: unknown) {
    if (error instanceof ExplicitVideoDownloadError) throw error;
}

function assertVideoConfig(config: AiConfig, model: string) {
    if (!model) throw new Error("请先配置视频模型");
    if (isLoggedIn()) return;
    const local = readLocalAiCredentials();
    if (!local.baseUrl.trim()) throw new Error("请先配置 Base URL");
    if (!local.apiKey.trim()) throw new Error("请先配置 API Key");
}

function normalizeVideoSecondsForModel(config: AiConfig, model: string, value: string) {
    return normalizeVideoDurationForModel(config, model, value);
}

function normalizeVideoSize(value: string) {
    if (value === "auto") return null;
    const size = value || "1280x720";
    if (/^\d+x\d+$/.test(size)) return size;
    return ["9:16", "2:3", "3:4"].includes(size) ? "720x1280" : "1280x720";
}

function normalizeVideoResolution(value: string) {
    if (value === "low") return "480p";
    if (value === "auto" || value === "high" || value === "medium") return "720p";
    const resolution = value.replace(/p$/i, "") || "720";
    return `${resolution}p`;
}

function normalizeXAIVideoAspectRatio(value: string) {
    if (!value || value === "auto") return undefined;
    if (/^\d+:\d+$/.test(value)) return value;
    const normalized = normalizeVideoSize(value);
    const match = normalized?.match(/^(\d+)x(\d+)$/);
    if (!match) return undefined;
    const width = Number(match[1]);
    const height = Number(match[2]);
    if (!width || !height) return undefined;
    const candidates = ["1:1", "16:9", "9:16", "4:3", "3:4", "3:2", "2:3"] as const;
    return candidates.reduce((best, item) => (Math.abs(ratioValue(item) - width / height) < Math.abs(ratioValue(best) - width / height) ? item : best), candidates[0]);
}

function normalizeVideoAspectRatio(value: string) {
    if (!value || value === "auto") return "16:9";
    if (/^\d+:\d+$/.test(value)) return value;
    const normalized = normalizeVideoSize(value);
    const match = normalized?.match(/^(\d+)x(\d+)$/);
    if (!match) return "16:9";
    const width = Number(match[1]);
    const height = Number(match[2]);
    if (!width || !height) return "16:9";
    const candidates = ["1:1", "16:9", "9:16", "4:3", "3:4", "3:2", "2:3"] as const;
    return candidates.reduce((best, item) => (Math.abs(ratioValue(item) - width / height) < Math.abs(ratioValue(best) - width / height) ? item : best), candidates[0]);
}

async function resolveVeoIngredientImage(image: ReferenceImage) {
    const directUrl = String(image.url || "").trim();
    if (isPublicMediaUrl(directUrl)) return directUrl;
    const uploaded = await uploadReferenceImageAsTempUrl(image);
    if (uploaded) return uploaded;
    const dataUrl = await imageToDataUrl(image);
    if (!dataUrl) return "";
    if (getDataUrlByteSize(dataUrl) <= 500 * 1024) return dataUrl;
    const compressed = await compressImageDataUrl(dataUrl, image.storageKey);
    if (getDataUrlByteSize(compressed) <= 500 * 1024) return compressed;
    throw new Error(`VEO 输入图片 data base64 数据不能超过 500KB，当前约 ${(getDataUrlByteSize(compressed) / 1024).toFixed(1)}KB`);
}

async function resolveYijiaInputReference(image: ReferenceImage) {
    const directUrl = String(image.url || "").trim();
    if (isPublicMediaUrl(directUrl)) return directUrl;
    const uploaded = await uploadReferenceImageAsTempUrl(image);
    if (uploaded) return uploaded;
    return imageToDataUrl(image);
}

async function uploadReferenceImageAsTempUrl(image: ReferenceImage) {
    const dataUrl = await imageToDataUrl(image);
    if (!dataUrl) return "";
    const file = dataUrlToFile({ ...image, dataUrl });
    const result = await uploadTempImage(file);
    return result.url;
}

async function compressImageDataUrl(dataUrl: string, storageKey?: string) {
    const sourceBlob = storageKey ? await getImageBlob(storageKey) : null;
    const sourceUrl = sourceBlob ? URL.createObjectURL(sourceBlob) : dataUrl;
    try {
        const image = await loadImageElement(sourceUrl);
        const candidates = [
            { maxSide: 1024, quality: 0.82 },
            { maxSide: 896, quality: 0.76 },
            { maxSide: 768, quality: 0.7 },
            { maxSide: 640, quality: 0.64 },
            { maxSide: 512, quality: 0.58 },
        ];
        let best = dataUrl;
        for (const candidate of candidates) {
            const next = await renderCompressedImage(image, candidate.maxSide, candidate.quality);
            if (getDataUrlByteSize(next) < getDataUrlByteSize(best)) best = next;
            if (getDataUrlByteSize(best) <= 500 * 1024) return best;
        }
        return best;
    } finally {
        if (sourceBlob) URL.revokeObjectURL(sourceUrl);
    }
}

function loadImageElement(url: string) {
    return new Promise<HTMLImageElement>((resolve, reject) => {
        const image = new Image();
        image.onload = () => resolve(image);
        image.onerror = () => reject(new Error("参考图读取失败，请换一张图片重试"));
        image.src = url;
    });
}

function renderCompressedImage(image: HTMLImageElement, maxSide: number, quality: number) {
    const scale = Math.min(1, maxSide / Math.max(image.naturalWidth || 1, image.naturalHeight || 1));
    const width = Math.max(1, Math.round((image.naturalWidth || 1) * scale));
    const height = Math.max(1, Math.round((image.naturalHeight || 1) * scale));
    const canvas = document.createElement("canvas");
    canvas.width = width;
    canvas.height = height;
    const context = canvas.getContext("2d");
    if (!context) throw new Error("参考图压缩失败");
    context.drawImage(image, 0, 0, width, height);
    return canvas.toDataURL("image/jpeg", quality);
}

function unwrapVideoResponse(payload: ApiVideoResponse) {
    return unwrapEnvelope(payload, "接口没有返回视频任务");
}

function unwrapSeedanceTask(payload: ApiEnvelope<SeedanceTask>) {
    return unwrapEnvelope(payload, "Seedance 接口没有返回任务");
}

function unwrapXAIVideoTask(payload: ApiEnvelope<XAIVideoTask>) {
    return unwrapEnvelope(payload, "视频接口没有返回任务");
}

function unwrapNewApiVideoTask(payload: ApiEnvelope<NewApiVideoTask>) {
    if (payload && typeof payload === "object" && "code" in payload && typeof payload.code === "string") {
        const code = payload.code.toLowerCase();
        if (code !== "success" && code !== "ok") throw new Error(readEnvelopeMessage(payload) || "视频接口没有返回任务");
        if (!payload.data) throw new Error("视频接口没有返回任务");
        return normalizeNewApiVideoTask(payload.data);
    }
    return normalizeNewApiVideoTask(unwrapEnvelope(payload, "视频接口没有返回任务"));
}

function readVideoTaskUrls(state: Partial<NewApiVideoTask>) {
    const urls = [
        state.result_url,
        state.url,
        state.video_url,
        state.metadata?.result_url,
        state.metadata?.url,
        state.metadata?.video_url,
        state.video?.url,
        ...(state.output || []),
        ...(state.metadata?.result_urls || []),
        state.original_watermarked_video_url,
        state.metadata?.original_watermarked_video_url,
    ].filter((item): item is string => typeof item === "string" && /^https?:\/\//i.test(item));
    return Array.from(new Set(urls));
}

function normalizeNewApiVideoTask(payload: NewApiVideoTask) {
    const nested = payload?.data ? normalizeNewApiVideoTask(payload.data) : null;
    const status = normalizeTaskStatus(payload.status || payload.state || payload.task_status, payload.success);
    const nestedStatus = normalizeTaskStatus(nested?.status || nested?.state || nested?.task_status, nested?.success);
    return {
        ...payload,
        ...nested,
        id: payload.id || payload.task_id || nested?.id || nested?.task_id,
        task_id: payload.task_id || nested?.task_id,
        status: nestedStatus || status || payload.status || nested?.status,
        url: payload.result_url || payload.url || payload.video_url || nested?.result_url || nested?.url || nested?.video_url,
        video_url: payload.video_url || nested?.video_url,
        result_url: payload.result_url || nested?.result_url,
        output: payload.output?.length ? payload.output : nested?.output,
        video: payload.video || nested?.video,
        error: payload.error || nested?.error,
        progress: typeof payload.progress === "number" ? payload.progress : nested?.progress,
        success: typeof payload.success === "boolean" ? payload.success : nested?.success,
    } satisfies NewApiVideoTask;
}

function normalizeTaskStatus(value?: string, success?: boolean) {
    const status = String(value || "").toLowerCase();
    if (!status && success) return "completed";
    if (status === "success") return "completed";
    return status;
}

function unwrapEnvelope<T>(payload: ApiEnvelope<T>, emptyMessage: string): T {
    if (!payload) throw new Error(emptyMessage);
    if (typeof payload === "object") {
        const envelope = payload as ApiEnvelope<T>;
        if ("ok" in envelope && envelope.ok === false) {
            throw new Error(readEnvelopeMessage(envelope) || emptyMessage);
        }
        if ("code" in envelope && typeof envelope.code === "number") {
            if (envelope.code !== 0) throw new Error(readEnvelopeMessage(envelope) || "请求失败");
            if (!envelope.data) throw new Error(emptyMessage);
            return envelope.data;
        }
    }
    return payload as T;
}

function readEnvelopeMessage(payload: unknown) {
    if (!payload || typeof payload !== "object") return "";
    const value = payload as { error_detail?: unknown; error?: unknown; msg?: unknown; message?: unknown };
    return videoTaskErrorMessage(value.error_detail) || videoTaskErrorMessage(value.error) || videoTaskErrorMessage(value.msg) || videoTaskErrorMessage(value.message);
}

function videoTaskErrorMessage(error: unknown, depth = 0): string {
    if (depth > 4 || error == null) return "";
    if (typeof error === "string") {
        const text = error.trim();
        if (!text) return "";
        if (text.startsWith("{") || text.startsWith("[")) {
            try {
                return videoTaskErrorMessage(JSON.parse(text), depth + 1) || text;
            } catch {
                return text;
            }
        }
        return text;
    }
    if (typeof error !== "object") return "";
    const value = error as Record<string, unknown>;
    for (const key of ["error_detail", "error", "message", "msg", "detail"]) {
        const message = videoTaskErrorMessage(value[key], depth + 1);
        if (message) return message;
    }
    return "";
}

function readAxiosError(error: unknown, fallback: string) {
    if (axios.isCancel(error)) return "请求已取消";
    if (axios.isAxiosError<ApiEnvelope<unknown>>(error)) {
        const responseData = error.response?.data;
        return readEnvelopeMessage(responseData) || statusMessage(error.response?.status, fallback);
    }
    if (error instanceof DOMException && error.name === "AbortError") return "请求已取消";
    return error instanceof Error ? error.message : fallback;
}

function errorMessage(error: unknown) {
    return error instanceof Error ? error.message : String(error || "未知错误");
}

function statusMessage(status: number | undefined, fallback: string) {
    if (status === 401 || status === 403) return "鉴权失败，请检查 API Key、套餐权限或模型权限";
    if (status === 429) return "请求被限流或额度不足，请稍后重试";
    return status ? `${fallback}（${status}）` : fallback;
}

function isXAIVideoModel(_model: string, baseUrl: string) {
    const normalizedBaseUrl = String(baseUrl || "").toLowerCase();
    return normalizedBaseUrl.includes("api.x.ai");
}

function isNewApiVideoGenerationModel(_model: string, baseUrl: string) {
    const normalizedBaseUrl = String(baseUrl || "").toLowerCase();
    return normalizedBaseUrl.includes("newapi");
}

function isWaninterVeoStyleModel(model: string) {
    const normalizedModel = modelOptionName(model).toLowerCase();
    return normalizedModel.includes("veo") || normalizedModel.includes("omni");
}

function ratioValue(value: string) {
    const [width, height] = value.split(":").map(Number);
    if (!width || !height) return 1;
    return width / height;
}

async function normalizeVideoBlob(blob: Blob) {
    const detected = await detectVideoMimeType(blob);
    if (detected) return blob.type === detected ? blob : blob.slice(0, blob.size, detected);
    const type = blob.type.toLowerCase().split(";")[0].trim();
    if (type.startsWith("video/")) return blob;
    await assertNotVideoErrorBlob(blob);
    if (!type || type === "application/octet-stream" || type === "binary/octet-stream") {
        throw new Error("视频下载结果不是可识别的视频格式，请检查上游返回的成片地址和 Content-Type");
    }
    throw new Error(`视频下载结果不是可播放的视频类型：${type}`);
}

async function detectVideoMimeType(blob: Blob) {
    const bytes = new Uint8Array(await blob.slice(0, Math.min(blob.size, 64)).arrayBuffer());
    if (bytes.length >= 12 && ascii(bytes, 4, 8) === "ftyp") return ascii(bytes, 8, 12) === "qt  " ? "video/quicktime" : "video/mp4";
    if (bytes.length >= 4 && bytes[0] === 0x1a && bytes[1] === 0x45 && bytes[2] === 0xdf && bytes[3] === 0xa3) return "video/webm";
    return "";
}

function ascii(bytes: Uint8Array, start: number, end: number) {
    return String.fromCharCode(...bytes.slice(start, end));
}

async function assertNotVideoErrorBlob(blob: Blob) {
    const type = blob.type.toLowerCase();
    const shouldParse = type.includes("json") || type.startsWith("text/") || (await blobLooksLikeJSON(blob));
    if (!shouldParse) return;
    let payload: { code?: number; msg?: string; message?: string; error?: string | { message?: string } };
    try {
        payload = JSON.parse(await blob.text()) as { code?: number; msg?: string; message?: string; error?: string | { message?: string } };
    } catch {
        return;
    }
    const message = payload.msg || payload.message || videoTaskErrorMessage(payload.error);
    if (typeof payload.code === "number" && payload.code !== 0) throw new ExplicitVideoDownloadError(message || "视频下载失败");
    if (message) throw new ExplicitVideoDownloadError(message);
}

async function blobLooksLikeJSON(blob: Blob) {
    const preview = await blob.slice(0, Math.min(blob.size, 64)).text();
    const first = preview.trimStart()[0];
    return first === "{" || first === "[";
}

function isPublicMediaUrl(value: string) {
    try {
        const url = new URL(value);
        const host = url.hostname.toLowerCase();
        if (url.protocol !== "http:" && url.protocol !== "https:") return false;
        if (host === "localhost" || host === "::1" || host.endsWith(".local") || /^127\./.test(host) || /^10\./.test(host) || /^192\.168\./.test(host) || /^172\.(1[6-9]|2\d|3[01])\./.test(host)) return false;
        return true;
    } catch {
        return false;
    }
}

function delay(ms: number, signal?: AbortSignal) {
    return new Promise<void>((resolve, reject) => {
        if (signal?.aborted) {
            reject(new DOMException("Aborted", "AbortError"));
            return;
        }
        const timer = setTimeout(resolve, ms);
        signal?.addEventListener(
            "abort",
            () => {
                clearTimeout(timer);
                reject(new DOMException("Aborted", "AbortError"));
            },
            { once: true },
        );
    });
}

function blobToDataUrl(blob: Blob) {
    return new Promise<string>((resolve, reject) => {
        const reader = new FileReader();
        reader.onload = () => resolve(String(reader.result || ""));
        reader.onerror = () => reject(new Error("读取本地素材失败"));
        reader.readAsDataURL(blob);
    });
}
