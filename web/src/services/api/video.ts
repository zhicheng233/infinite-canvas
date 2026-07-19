import axios from "axios";
import { isLoggedIn, proxyAiGet } from "./ai-proxy";
import { API_BASE } from "./client";

import { notifyCreditBalanceChanged } from "@/constant/credits";
import { dataUrlToFile, getDataUrlByteSize } from "@/lib/image-utils";
import { getMediaBlob, uploadMediaFile, type UploadedFile } from "@/services/file-storage";
import { getImageBlob, imageToDataUrl } from "@/services/image-storage";
import { uploadTempImage } from "@/services/api/temp-media";
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
    error?: { code?: string | number; message?: string } | null;
    data?: NewApiVideoTask | null;
};
type SeedanceTask = {
    id: string;
    status?: "queued" | "running" | "succeeded" | "failed" | "cancelled" | "expired";
    error?: { code?: string; message?: string } | null;
    content?: { video_url?: string; last_frame_url?: string } | null;
};
type ApiEnvelope<T> = T | { code?: number; data?: T | null; msg?: string };
type RequestOptions = { signal?: AbortSignal };

export type VideoGenerationResult = { blob?: Blob; url?: string; mimeType?: string };
export type VideoGenerationTask = { id: string; provider: "openai" | "seedance" | "xai" | "newapi" | "yijia"; model: string; channelId?: number; channelModelId?: number };
export type VideoGenerationTaskState = { status: "pending" } | { status: "completed"; result: VideoGenerationResult } | { status: "failed"; error: string };

function aiApiUrl(config: AiConfig, path: string) {
    if (isLoggedIn()) {
        return buildProxyApiUrl(API_BASE, config, config.model || config.videoModel, path);
    }
    return buildApiUrl(readLocalAiCredentials().baseUrl, path);
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
        if (configuredRoute !== "seedance" && (videoReferences.length || audioReferences.length)) {
            throw new Error("当前视频模型暂不支持参考视频或参考音频，请仅保留参考图片");
        }
        if (configuredRoute === "openai") return createOpenAIVideoTask(requestConfig, selectedModel, prompt, references, options);
        if (configuredRoute === "veo_json") return createVeoJsonVideoTask(requestConfig, selectedModel, prompt, references, options);
        if (configuredRoute === "waninter") return createWaninterVideoTask(requestConfig, selectedModel, prompt, references, options);
        if (configuredRoute === "yijia") return createYijiaVideoTask(requestConfig, selectedModel, prompt, references, options);
        if (configuredRoute === "xai") return createXAIVideoTask(requestConfig, selectedModel, prompt, references, options);
        if (configuredRoute === "newapi") return createNewApiVideoTask(requestConfig, selectedModel, prompt, references, options);
        if (configuredRoute === "seedance") return createSeedanceTask(requestConfig, selectedModel, prompt, references, videoReferences, audioReferences, options);
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
    if (task.provider === "newapi") return pollNewApiVideoTask(requestConfig, task, options);
    if (task.provider === "yijia") return pollYijiaVideoTask(requestConfig, task, options);
    return pollOpenAIVideoTask(requestConfig, task, options);
}

export async function storeGeneratedVideo(result: VideoGenerationResult): Promise<UploadedFile> {
    if (result.blob) return uploadMediaFile(result.blob, "video");
    if (result.url) return { url: result.url, storageKey: "", bytes: 0, mimeType: result.mimeType || "video/mp4" };
    throw new Error("视频接口没有返回可播放的视频");
}

async function createOpenAIVideoTask(config: AiConfig, model: string, prompt: string, references: ReferenceImage[], options?: RequestOptions): Promise<VideoGenerationTask> {
    const body = new FormData();
    body.append("model", modelOptionName(model));
    body.append("prompt", prompt);
    body.append("seconds", normalizeVideoSecondsForModel(config, model, config.videoSeconds));
    if (normalizeVideoSize(config.size)) body.append("size", normalizeVideoSize(config.size)!);
    body.append("resolution_name", normalizeVideoResolution(config.vquality));
    body.append("preset", "normal");
    const files = await Promise.all(references.slice(0, 7).map(async (image) => dataUrlToFile({ ...image, dataUrl: await imageToDataUrl(image) })));
    files.forEach((file) => body.append("input_reference[]", file));
    try {
        const created = unwrapVideoResponse((await axios.post<ApiVideoResponse>(aiApiUrl(config, "/videos"), body, { headers: aiHeaders(config), signal: options?.signal })).data);
        const taskId = created.id || created.task_id;
        if (!taskId) throw new Error("视频接口没有返回任务 ID");
        notifyCreditBalanceChanged();
        return { id: taskId, provider: "openai", model };
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
        const ratio = normalizeXAIVideoAspectRatio(config.size);
        if (ratio) payload.aspect_ratio = ratio;
        const resolution = normalizeVideoResolution(config.vquality);
        if (resolution) payload.resolution = resolution;
    }
    try {
        const created = unwrapXAIVideoTask((await axios.post<ApiEnvelope<XAIVideoTask>>(aiApiUrl(config, "/videos/generations"), payload, { headers: aiHeaders(config, "application/json"), signal: options?.signal })).data);
        const requestId = created.request_id || created.id;
        if (!requestId) throw new Error("视频接口没有返回任务 ID");
        notifyCreditBalanceChanged();
        return { id: requestId, provider: "xai", model };
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
        const created = unwrapVideoResponse((await axios.post<ApiVideoResponse>(aiApiUrl(config, "/videos"), payload, { headers: aiHeaders(config, "application/json"), signal: options?.signal })).data);
        const taskId = created.id || created.task_id;
        if (!taskId) throw new Error("视频接口没有返回任务 ID");
        notifyCreditBalanceChanged();
        return { id: taskId, provider: "openai", model };
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
        const created = unwrapVideoResponse((await axios.post<ApiVideoResponse>(aiApiUrl(config, "/videos"), payload, { headers: aiHeaders(config, "application/json"), signal: options?.signal })).data);
        const taskId = created.id || created.task_id;
        if (!taskId) throw new Error("瑙嗛鎺ュ彛娌℃湁杩斿洖浠诲姟 ID");
        notifyCreditBalanceChanged();
        return { id: taskId, provider: "openai", model };
    } catch (error) {
        throw new Error(readAxiosError(error, "瑙嗛浠诲姟鍒涘缓澶辫触"));
    }
}

async function createYijiaVideoTask(config: AiConfig, model: string, prompt: string, references: ReferenceImage[], options?: RequestOptions): Promise<VideoGenerationTask> {
    const payload: Record<string, unknown> = {
        model: modelOptionName(model),
        prompt,
        size: normalizeVideoSize(config.size) || "1280x720",
        seconds: normalizeVideoSecondsForModel(config, model, config.videoSeconds),
        n: 1,
        watermark: boolConfig(config.videoWatermark, false),
        private: false,
        storyboard: false,
    };
    if (references.length > 1) throw new Error("当前视频模型只支持单张参考图");
    if (references[0]) payload.input_reference = await resolveYijiaInputReference(references[0]);
    try {
        const created = unwrapVideoResponse((await axios.post<ApiVideoResponse>(aiApiUrl(config, "/videos"), payload, { headers: aiHeaders(config, "application/json"), signal: options?.signal })).data);
        const taskId = created.id || created.task_id;
        if (!taskId) throw new Error("视频接口没有返回任务 ID");
        notifyCreditBalanceChanged();
        return { id: taskId, provider: "yijia", model };
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
        const created = unwrapNewApiVideoTask((await axios.post<ApiEnvelope<NewApiVideoTask>>(aiApiUrl(config, "/video/generations"), payload, { headers: aiHeaders(config, "application/json"), signal: options?.signal })).data);
        const taskId = created.task_id || created.id;
        if (!taskId) throw new Error("视频接口没有返回任务 ID");
        notifyCreditBalanceChanged();
        return { id: taskId, provider: "newapi", model };
    } catch (error) {
        throw new Error(readAxiosError(error, "视频任务创建失败"));
    }
}

async function pollYijiaVideoTask(config: AiConfig, task: VideoGenerationTask, options?: RequestOptions): Promise<VideoGenerationTaskState> {
    return pollOpenAIVideoTask(config, task, options);
}

async function pollOpenAIVideoTask(config: AiConfig, task: VideoGenerationTask, options?: RequestOptions): Promise<VideoGenerationTaskState> {
    try {
        const video = unwrapVideoResponse((await axios.get<ApiVideoResponse>(aiApiUrl(config, `/videos/${task.id}`), { headers: aiHeaders(config), signal: options?.signal })).data);
        const status = String(video.status || "").toLowerCase();
        if (status === "completed" || status === "succeeded" || status === "done") {
            const result = await resolveVideoTaskResult(config, video as NewApiVideoTask, options);
            if (result) return { status: "completed", result };
            const content = await axios.get<Blob>(aiApiUrl(config, `/videos/${task.id}/content`), { headers: aiHeaders(config), responseType: "blob", signal: options?.signal });
            await assertVideoBlob(content.data);
            return { status: "completed", result: { blob: content.data } };
        }
        if (status === "failed" || status === "cancelled" || status === "error") return { status: "failed", error: video.error?.message || "视频生成失败" };
        return { status: "pending" };
    } catch (error) {
        throw new Error(readAxiosError(error, "视频任务查询失败"));
    }
}

async function pollXAIVideoTask(config: AiConfig, task: VideoGenerationTask, options?: RequestOptions): Promise<VideoGenerationTaskState> {
    try {
        const state = unwrapXAIVideoTask((await axios.get<ApiEnvelope<XAIVideoTask>>(aiApiUrl(config, `/videos/${task.id}`), { headers: aiHeaders(config), signal: options?.signal })).data);
        const status = String(state.status || "").toLowerCase();
        if (status === "done" || status === "completed" || status === "succeeded") {
            const url = state.video?.url;
            if (!url) return { status: "failed", error: "视频生成成功但没有返回视频地址" };
            return { status: "completed", result: await videoResultFromUrl(config, url, options) };
        }
        if (status === "failed" || status === "cancelled" || status === "error") {
            return { status: "failed", error: state.error?.message || "视频生成失败" };
        }
        return { status: "pending" };
    } catch (error) {
        throw new Error(readAxiosError(error, "视频任务查询失败"));
    }
}

async function pollNewApiVideoTask(config: AiConfig, task: VideoGenerationTask, options?: RequestOptions): Promise<VideoGenerationTaskState> {
    try {
        const state = unwrapNewApiVideoTask((await axios.get<ApiEnvelope<NewApiVideoTask>>(aiApiUrl(config, `/video/generations/${task.id}`), { headers: aiHeaders(config), signal: options?.signal })).data);
        const status = String(state.status || "").toLowerCase();
        if (status === "completed" || status === "succeeded" || status === "done") {
            const result = await resolveVideoTaskResult(config, state, options);
            if (result) return { status: "completed", result };
            return { status: "failed", error: "视频生成成功但没有返回可播放的视频地址" };
        }
        if (status === "failed" || status === "cancelled" || status === "error") {
            return { status: "failed", error: state.error?.message || "视频生成失败" };
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
        const created = unwrapSeedanceTask((await axios.post<ApiEnvelope<SeedanceTask>>(seedanceApiUrl(config), payload, { headers: aiHeaders(config, "application/json"), signal: options?.signal })).data);
        if (!created.id) throw new Error("Seedance 接口没有返回任务 ID");
        notifyCreditBalanceChanged();
        return { id: created.id, provider: "seedance", model };
    } catch (error) {
        throw new Error(readAxiosError(error, "Seedance 任务创建失败"));
    }
}

async function pollSeedanceTask(config: AiConfig, task: VideoGenerationTask, options?: RequestOptions): Promise<VideoGenerationTaskState> {
    try {
        const state = unwrapSeedanceTask((await axios.get<ApiEnvelope<SeedanceTask>>(seedanceApiUrl(config, task.id), { headers: aiHeaders(config), signal: options?.signal })).data);
        if (state.status === "succeeded") {
            const url = state.content?.video_url;
            if (!url) return { status: "failed", error: "Seedance 任务成功但没有返回视频 URL" };
            return { status: "completed", result: await videoResultFromUrl(config, url, options) };
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

function seedanceApiUrl(config: AiConfig, taskId?: string) {
    return aiApiUrl(config, `/contents/generations/tasks${taskId ? `/${encodeURIComponent(taskId)}` : ""}`);
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

async function videoResultFromUrl(config: AiConfig, url: string, options?: RequestOptions): Promise<VideoGenerationResult> {
    try {
        const response = await fetchVideoBlob(config, url, options);
        await assertVideoBlob(response.data);
        return { blob: response.data };
    } catch (error) {
        if (axios.isCancel(error) || options?.signal?.aborted) throw error;
        if (requiresAuthenticatedVideoContent(url)) throw error;
        return { url, mimeType: "video/mp4" };
    }
}

async function resolveVideoTaskResult(config: AiConfig, state: Partial<NewApiVideoTask>, options?: RequestOptions) {
    for (const url of readVideoTaskUrls(state)) {
        try {
            return await videoResultFromUrl(config, url, options);
        } catch {
            continue;
        }
    }
    return null;
}

async function fetchVideoBlob(config: AiConfig, url: string, options?: RequestOptions) {
    if (isLoggedIn()) {
        const proxyPath = toProxyableVideoPath(url);
        if (proxyPath) {
            return proxyAiGet(proxyPath, { signal: options?.signal, responseType: "blob" as string });
        }
    }
    return axios.get<Blob>(url, { responseType: "blob", signal: options?.signal });
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

function requiresAuthenticatedVideoContent(url: string) {
    try {
        const target = new URL(url);
        return /\/v1\/videos\/[^/]+\/content$/i.test(target.pathname);
    } catch {
        return false;
    }
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
        state.url,
        state.video_url,
        state.result_url,
        state.video?.url,
        state.metadata?.url,
        state.metadata?.video_url,
        state.metadata?.result_url,
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
        url: payload.url || payload.video_url || payload.result_url || nested?.url || nested?.video_url || nested?.result_url,
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
    if (typeof payload === "object" && "code" in payload && typeof payload.code === "number") {
        if (payload.code !== 0) throw new Error(payload.msg || "请求失败");
        if (!payload.data) throw new Error(emptyMessage);
        return payload.data;
    }
    return payload as T;
}

function readEnvelopeMessage(payload: unknown) {
    if (!payload || typeof payload !== "object") return "";
    const value = payload as { msg?: string; message?: string; error?: { message?: string } };
    return value.msg || value.message || value.error?.message || "";
}

function readAxiosError(error: unknown, fallback: string) {
    if (axios.isCancel(error)) return "请求已取消";
    if (axios.isAxiosError<{ error?: { message?: string }; msg?: string; code?: number }>(error)) {
        const responseData = error.response?.data;
        return responseData?.msg || responseData?.error?.message || statusMessage(error.response?.status, fallback);
    }
    if (error instanceof DOMException && error.name === "AbortError") return "请求已取消";
    return error instanceof Error ? error.message : fallback;
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

async function assertVideoBlob(blob: Blob) {
    if (!blob.type.includes("json")) return;
    let payload: { code?: number; msg?: string; error?: { message?: string } };
    try {
        payload = JSON.parse(await blob.text()) as { code?: number; msg?: string; error?: { message?: string } };
    } catch {
        return;
    }
    if (typeof payload.code === "number" && payload.code !== 0) throw new Error(payload.msg || "视频下载失败");
    if (payload.error?.message) throw new Error(payload.error.message);
}

function isPublicMediaUrl(value: string) {
    return /^https?:\/\//i.test(value || "");
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
