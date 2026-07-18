"use client";

import { useMemo } from "react";
import { create } from "zustand";
import { persist } from "zustand/middleware";
import { nanoid } from "nanoid";

export type ModelChannel = {
    id: string;
    name: string;
    baseUrl: string;
    apiKey: string;
    models: string[];
};

export type AiConfig = {
    channelMode: "remote" | "local";
    baseUrl: string;
    apiKey: string;
    channels: ModelChannel[];
    model: string;
    imageModel: string;
    videoModel: string;
    textModel: string;
    audioModel: string;
    audioVoice: string;
    audioFormat: string;
    audioSpeed: string;
    audioInstructions: string;
    videoSeconds: string;
    vquality: string;
    videoGenerateAudio: string;
    videoWatermark: string;
    systemPrompt: string;
    models: string[];
    imageModels: string[];
    videoModels: string[];
    textModels: string[];
    audioModels: string[];
    modelRoutes: Record<string, string>;
    modelVideoDurations: Record<string, number[]>;
    modelVideoCustomizable: Record<string, boolean>;
    quality: string;
    size: string;
    count: string;
    canvasImageCount: string;
};

export type WebdavSyncConfig = {
    proxyMode: "direct" | "nextjs";
    url: string;
    username: string;
    password: string;
    directory: string;
    lastSyncedAt: string;
};

export const CONFIG_STORE_KEY = "infinite-canvas:ai_config_store";
export type ModelCapability = "image" | "video" | "text" | "audio";
export type ModelRouteCapability = "image" | "image_generate" | "image_edit" | "video";
export type ImageRouteMode = "auto" | "generations" | "edits" | "chat" | "banana";
export type VideoRouteMode = "auto" | "openai" | "veo_json" | "waninter" | "yijia" | "xai" | "newapi" | "seedance";
const CHANNEL_MODEL_SEPARATOR = "::";
const IMAGE_ROUTE_VALUES = new Set<string>(["generations", "edits", "chat", "banana"]);
const VIDEO_ROUTE_VALUES = new Set<string>(["openai", "veo_json", "waninter", "yijia", "xai", "newapi", "seedance"]);

export const defaultConfig: AiConfig = {
    channelMode: "local",
    baseUrl: "",
    apiKey: "",
    channels: [
        {
            id: "default",
            name: "默认渠道",
            baseUrl: "",
            apiKey: "",
            models: ["gpt-image-2", "grok-imagine-video", "gpt-5.5", "gpt-4o-mini-tts"],
        },
    ],
    model: "default::gpt-image-2",
    imageModel: "default::gpt-image-2",
    videoModel: "default::grok-imagine-video",
    textModel: "default::gpt-5.5",
    audioModel: "default::gpt-4o-mini-tts",
    audioVoice: "alloy",
    audioFormat: "mp3",
    audioSpeed: "1",
    audioInstructions: "",
    videoSeconds: "6",
    vquality: "720",
    videoGenerateAudio: "true",
    videoWatermark: "false",
    systemPrompt: "",
    models: ["default::gpt-image-2", "default::grok-imagine-video", "default::gpt-5.5", "default::gpt-4o-mini-tts"],
    imageModels: ["default::gpt-image-2"],
    videoModels: ["default::grok-imagine-video"],
    textModels: ["default::gpt-5.5"],
    audioModels: ["default::gpt-4o-mini-tts"],
    modelRoutes: {},
    modelVideoDurations: {},
    modelVideoCustomizable: {},
    quality: "auto",
    size: "1:1",
    count: "1",
    canvasImageCount: "1",
};

export const defaultWebdavSyncConfig: WebdavSyncConfig = {
    proxyMode: "direct",
    url: "",
    username: "",
    password: "",
    directory: "infinite-canvas",
    lastSyncedAt: "",
};

type ConfigStore = {
    config: AiConfig;
    webdav: WebdavSyncConfig;
    isConfigOpen: boolean;
    shouldPromptContinue: boolean;
    updateConfig: <K extends keyof AiConfig>(key: K, value: AiConfig[K]) => void;
    updateWebdavConfig: <K extends keyof WebdavSyncConfig>(key: K, value: WebdavSyncConfig[K]) => void;
    isAiConfigReady: (config: AiConfig, model: string) => boolean;
    applyServerModelCatalog: (catalog: {
        models?: string[];
        imageModels?: string[];
        videoModels?: string[];
        textModels?: string[];
        audioModels?: string[];
        modelRoutes?: Record<string, string>;
        modelVideoDurations?: Record<string, number[]>;
        modelVideoCustomizable?: Record<string, boolean>;
    }) => void;
    openConfigDialog: (shouldPromptContinue?: boolean) => void;
    setConfigDialogOpen: (isOpen: boolean) => void;
    clearPromptContinue: () => void;
};

function isVideoModelName(model: string) {
    const value = modelOptionName(model).toLowerCase();
    return value.includes("seedance") || value.includes("video") || value.includes("sora") || value.includes("veo") || value.includes("kling") || value.includes("wan") || value.includes("hailuo");
}

function isImageModelName(model: string) {
    const value = modelOptionName(model).toLowerCase();
    return !isVideoModelName(model) && !isAudioModelName(model) && (value.includes("seedream") || value.includes("gpt-image") || value.includes("image") || value.includes("dall-e") || value.includes("dalle") || value.includes("imagen") || value.includes("flux") || value.includes("sdxl") || value.includes("stable-diffusion") || value.includes("midjourney"));
}

function isAudioModelName(model: string) {
    const value = modelOptionName(model).toLowerCase();
    return value.includes("audio") || value.includes("tts") || value.includes("speech") || value.includes("voice") || value.includes("music") || value.includes("sound");
}

function isTextModelName(model: string) {
    return !isImageModelName(model) && !isVideoModelName(model) && !isAudioModelName(model);
}

export function modelMatchesCapability(model: string, capability?: ModelCapability) {
    if (!capability) return true;
    if (capability === "image") return isImageModelName(model);
    if (capability === "video") return isVideoModelName(model);
    if (capability === "audio") return isAudioModelName(model);
    return isTextModelName(model);
}

export function filterModelsByCapability(models: string[], capability?: ModelCapability) {
    return capability ? models.filter((model) => modelMatchesCapability(model, capability)) : models;
}

export function selectableModelsByCapability(config: AiConfig, capability?: ModelCapability) {
    if (!capability) return config.models;
    return config[modelListKey(capability)];
}

function modelIncludedInCapabilityList(config: AiConfig, capability: ModelCapability, model: string) {
    const current = model.trim();
    if (!current) return false;
    const currentName = modelOptionName(current);
    return selectableModelsByCapability(config, capability).some((item) => item === current || modelOptionName(item) === currentName);
}

export function defaultModelForCapability(config: AiConfig, capability: ModelCapability) {
    if (capability === "image") return config.imageModel || config.model;
    if (capability === "video") return config.videoModel || config.model;
    if (capability === "audio") return config.audioModel || config.model;
    return config.textModel || config.model;
}

export function resolveCapabilityModel(config: AiConfig, capability: ModelCapability, currentValue?: string) {
    const current = (currentValue || "").trim();
    if (current && (modelIncludedInCapabilityList(config, capability, current) || modelMatchesCapability(current, capability))) return current;
    const defaultValue = defaultModelForCapability(config, capability);
    if (defaultValue && (modelIncludedInCapabilityList(config, capability, defaultValue) || modelMatchesCapability(defaultValue, capability))) return defaultValue;
    return selectableModelsByCapability(config, capability)[0] || defaultValue || current || "";
}

function modelListKey(capability: ModelCapability) {
    return `${capability}Models` as "imageModels" | "videoModels" | "textModels" | "audioModels";
}

function isAiConfigReady(config: AiConfig, model: string) {
    // Logged-in users go through server proxy (admin configures API)
    if (typeof window !== "undefined" && window.localStorage.getItem("infinite-canvas:auth_token")) return true;
    const channel = resolveModelChannel(config, model);
    return Boolean(model.trim() && channel.baseUrl.trim() && channel.apiKey.trim());
}

export const useConfigStore = create<ConfigStore>()(
    persist(
        (set, get) => ({
            config: defaultConfig,
            webdav: defaultWebdavSyncConfig,
            isConfigOpen: false,
            shouldPromptContinue: false,
            updateConfig: (key, value) =>
                set((state) => ({
                    config: {
                        ...state.config,
                        [key]: value,
                    },
                })),
            updateWebdavConfig: (key, value) =>
                set((state) => ({
                    webdav: {
                        ...state.webdav,
                        [key]: value,
                    },
                })),
            isAiConfigReady: (config, model) => isAiConfigReady(config, model),
            applyServerModelCatalog: (catalog) =>
                set((state) => {
                    const allModels = normalizeServerModelCatalog(catalog.models);
                    const derivedImageModels = catalog.imageModels?.length ? normalizeServerModelCatalog(catalog.imageModels) : filterModelsByCapability(allModels, "image");
                    const derivedVideoModels = catalog.videoModels?.length ? normalizeServerModelCatalog(catalog.videoModels) : filterModelsByCapability(allModels, "video");
                    const derivedTextModels = catalog.textModels?.length ? normalizeServerModelCatalog(catalog.textModels) : filterModelsByCapability(allModels, "text");
                    const derivedAudioModels = catalog.audioModels?.length ? normalizeServerModelCatalog(catalog.audioModels) : filterModelsByCapability(allModels, "audio");
                    const modelRoutes = normalizeModelRoutes(catalog.modelRoutes, allModels);
                    const modelVideoDurations = normalizeModelVideoDurations(catalog.modelVideoDurations, allModels);
                    const modelVideoCustomizable = normalizeModelVideoCustomizable(catalog.modelVideoCustomizable, allModels);
                    const nextChannels = state.config.channels.length
                        ? state.config.channels.map((channel, index) => (index === 0 ? { ...channel, models: allModels } : channel))
                        : [createModelChannel({ id: "default", name: "默认渠道", baseUrl: state.config.baseUrl, apiKey: state.config.apiKey, models: allModels })];
                    return {
                        config: {
                            ...state.config,
                            channels: nextChannels,
                            models: allModels,
                            imageModels: derivedImageModels,
                            videoModels: derivedVideoModels,
                            textModels: derivedTextModels,
                            audioModels: derivedAudioModels,
                            modelRoutes,
                            modelVideoDurations,
                            modelVideoCustomizable,
                            model: pickServerDefaultModel(state.config.model, allModels, derivedImageModels),
                            imageModel: pickServerDefaultModel(state.config.imageModel, derivedImageModels, allModels),
                            videoModel: pickServerDefaultModel(state.config.videoModel, derivedVideoModels, allModels),
                            textModel: pickServerDefaultModel(state.config.textModel, derivedTextModels, allModels),
                            audioModel: pickServerDefaultModel(state.config.audioModel, derivedAudioModels, allModels),
                        },
                    };
                }),
            openConfigDialog: (shouldPromptContinue = false) => set({ isConfigOpen: true, shouldPromptContinue }),
            setConfigDialogOpen: (isConfigOpen) => set({ isConfigOpen }),
            clearPromptContinue: () => set({ shouldPromptContinue: false }),
        }),
        {
            name: CONFIG_STORE_KEY,
            partialize: (state) => ({ config: state.config, webdav: state.webdav }),
            merge: (persisted, current) => {
                const persistedState = (persisted || {}) as Partial<ConfigStore>;
                const persistedConfig = (persistedState.config || {}) as Partial<AiConfig>;
                const persistedWebdav = (persistedState.webdav || {}) as Partial<WebdavSyncConfig>;
                const config = { ...defaultConfig, ...persistedConfig };
                if (!Array.isArray(persistedConfig.channels)) config.channels = [];
                const channels = normalizeChannels(config);
                const models = modelOptionsFromChannels(channels);
                return {
                    ...current,
                    webdav: { ...defaultWebdavSyncConfig, ...persistedWebdav },
                    config: {
                        ...config,
                        channelMode: "local",
                        channels,
                        models,
                        imageModel: normalizeModelOptionValue(config.imageModel || config.model, channels),
                        videoModel: normalizeModelOptionValue(config.videoModel || "grok-imagine-video", channels),
                        textModel: normalizeModelOptionValue(config.textModel || config.model, channels),
                        audioModel: normalizeModelOptionValue(config.audioModel || defaultConfig.audioModel, channels),
                        audioVoice: config.audioVoice || defaultConfig.audioVoice,
                        audioFormat: config.audioFormat || defaultConfig.audioFormat,
                        audioSpeed: config.audioSpeed || defaultConfig.audioSpeed,
                        audioInstructions: config.audioInstructions || "",
                        videoSeconds: config.videoSeconds || "6",
                        vquality: config.vquality || "720",
                        videoGenerateAudio: config.videoGenerateAudio || "true",
                        videoWatermark: config.videoWatermark || "false",
                        canvasImageCount: config.canvasImageCount || "1",
                        imageModels: Array.isArray(persistedConfig.imageModels) ? normalizeModelList(config.imageModels, channels) : filterModelsByCapability(models, "image"),
                        videoModels: Array.isArray(persistedConfig.videoModels) ? normalizeModelList(config.videoModels, channels) : filterModelsByCapability(models, "video"),
                        textModels: Array.isArray(persistedConfig.textModels) ? normalizeModelList(config.textModels, channels) : filterModelsByCapability(models, "text"),
                        audioModels: Array.isArray(persistedConfig.audioModels) ? normalizeModelList(config.audioModels, channels) : filterModelsByCapability(models, "audio"),
                        modelRoutes: normalizeModelRoutes(config.modelRoutes, models),
                        modelVideoDurations: normalizeModelVideoDurations(config.modelVideoDurations, models),
                        modelVideoCustomizable: normalizeModelVideoCustomizable(config.modelVideoCustomizable, models),
                    },
                };
            },
        },
    ),
);

function normalizeModelList(models: string[], channels: ModelChannel[]) {
    const allModelOptions = channels.flatMap((channel) => channel.models.map((model) => encodeChannelModel(channel.id, model)));
    return Array.from(new Set((models || []).map((model) => model.trim()).filter(Boolean)))
        .map((model) => normalizeModelOptionValue(model, channels))
        .filter((model) => !allModelOptions.length || allModelOptions.includes(model) || !isChannelModelValue(model));
}

export function useEffectiveConfig() {
    const config = useConfigStore((state) => state.config);
    return useMemo(() => ({ ...config, channelMode: "local" as const }), [config]);
}

export function createModelChannel(channel?: Partial<ModelChannel>): ModelChannel {
    return {
        id: channel?.id?.trim() || nanoid(),
        name: channel?.name?.trim() || "新渠道",
        baseUrl: channel?.baseUrl?.trim() || "https://api.openai.com",
        apiKey: channel?.apiKey || "",
        models: uniqueRawModels(channel?.models || []),
    };
}

export function encodeChannelModel(channelId: string, model: string) {
    return `${channelId}${CHANNEL_MODEL_SEPARATOR}${model.trim()}`;
}

export function isChannelModelValue(value: string) {
    return value.includes(CHANNEL_MODEL_SEPARATOR);
}

export function decodeChannelModel(value: string) {
    const index = value.indexOf(CHANNEL_MODEL_SEPARATOR);
    if (index < 0) return null;
    return { channelId: value.slice(0, index), model: value.slice(index + CHANNEL_MODEL_SEPARATOR.length) };
}

export function modelOptionName(value: string) {
    return decodeChannelModel(value)?.model || value;
}

export function fixedVideoDurationForModel(value: string) {
    return null;
}

export function videoDurationOptionsForModel(config: Pick<AiConfig, "modelVideoDurations">, value: string) {
    const model = modelOptionName(value).trim();
    const items = config.modelVideoDurations?.[model];
    if (!Array.isArray(items) || !items.length) return [];
    return Array.from(new Set(items.map((item) => Math.floor(Number(item) || 0)).filter((item) => item > 0))).sort((left, right) => left - right);
}

export function fixedConfiguredVideoDurationForModel(config: Pick<AiConfig, "modelVideoDurations">, value: string) {
    const items = videoDurationOptionsForModel(config, value);
    if (items.length === 1) return items[0];
    return fixedVideoDurationForModel(value);
}

export function isVideoDurationCustomizable(config: Pick<AiConfig, "modelVideoCustomizable">, value: string) {
    const model = modelOptionName(value).trim();
    return Boolean(config.modelVideoCustomizable?.[model]);
}

export function normalizeVideoDurationForModel(config: Pick<AiConfig, "modelVideoDurations" | "modelVideoCustomizable">, model: string, value: string) {
    const fixed = fixedConfiguredVideoDurationForModel(config, model);
    if (fixed) return String(fixed);
    const seconds = Math.max(1, Math.min(20, Math.floor(Number(value) || 6)));
    const options = videoDurationOptionsForModel(config, model);
    if (!options.length || isVideoDurationCustomizable(config, model)) return String(seconds);
    return String(options.includes(seconds) ? seconds : options[0]);
}

function modelRouteKey(capability: ModelRouteCapability, model: string) {
    return `${capability}:${modelOptionName(model).trim()}`;
}

function inferRouteCapability(route: string): ModelRouteCapability | "" {
    if (IMAGE_ROUTE_VALUES.has(route)) return "image";
    if (VIDEO_ROUTE_VALUES.has(route)) return "video";
    return "";
}

export function modelRouteForCapability(config: Pick<AiConfig, "modelRoutes">, capability: ModelRouteCapability, value: string) {
    const model = modelOptionName(value).trim();
    if (!model) return "auto";
    return config.modelRoutes?.[modelRouteKey(capability, model)] || "auto";
}

export function imageRouteForModel(config: Pick<AiConfig, "modelRoutes">, value: string) {
    return modelRouteForCapability(config, "image", value) as ImageRouteMode;
}

export function imageGenerateRouteForModel(config: Pick<AiConfig, "modelRoutes">, value: string) {
    const model = modelOptionName(value).trim();
    if (!model) return "auto" as ImageRouteMode;
    return (config.modelRoutes?.[modelRouteKey("image_generate", model)] || "auto") as ImageRouteMode;
}

export function imageEditRouteForModel(config: Pick<AiConfig, "modelRoutes">, value: string) {
    const model = modelOptionName(value).trim();
    if (!model) return "auto" as ImageRouteMode;
    return (config.modelRoutes?.[modelRouteKey("image_edit", model)] || config.modelRoutes?.[modelRouteKey("image", model)] || "auto") as ImageRouteMode;
}

export function videoRouteForModel(config: Pick<AiConfig, "modelRoutes">, value: string) {
    return modelRouteForCapability(config, "video", value) as VideoRouteMode;
}

export function modelOptionLabel(config: AiConfig, value: string) {
    const decoded = decodeChannelModel(value);
    if (!decoded) return value;
    const channel = config.channels.find((item) => item.id === decoded.channelId);
    return channel ? `${decoded.model}（${channel.name}）` : decoded.model;
}

export function modelOptionsFromChannels(channels: ModelChannel[]) {
    return uniqueModelOptions(channels.flatMap((channel) => channel.models.map((model) => encodeChannelModel(channel.id, model))));
}

export function normalizeModelOptionValue(value: string | undefined, channels: ModelChannel[]) {
    const model = (value || "").trim();
    if (!model) return "";
    const decoded = decodeChannelModel(model);
    if (decoded) {
        const channel = channels.find((item) => item.id === decoded.channelId);
        return channel && channel.models.includes(decoded.model) ? model : "";
    }
    const channel = channels.find((item) => item.models.includes(decoded?.model || model)) || channels[0];
    return channel && channel.models.includes(decoded?.model || model) ? encodeChannelModel(channel.id, decoded?.model || model) : model;
}

export function resolveModelChannel(config: AiConfig, value: string) {
    const decoded = decodeChannelModel(value);
    const model = decoded?.model || value;
    const matched = decoded ? config.channels.find((channel) => channel.id === decoded.channelId) : config.channels.find((channel) => channel.models.includes(model));
    return matched || config.channels[0] || createModelChannel({ id: "default", name: "默认渠道", baseUrl: config.baseUrl, apiKey: config.apiKey, models: config.models.map(modelOptionName) });
}

export function resolveModelRequestConfig(config: AiConfig, value: string) {
    const channel = resolveModelChannel(config, value);
    return {
        ...config,
        model: modelOptionName(value || config.model),
        baseUrl: channel.baseUrl,
        apiKey: channel.apiKey,
    };
}

function normalizeChannels(config: AiConfig) {
    const persistedChannels = Array.isArray(config.channels) ? config.channels : [];
    const channels = persistedChannels.map((channel, index) =>
        createModelChannel({
            ...channel,
            id: channel.id || (index === 0 ? "default" : `channel-${index + 1}`),
            name: channel.name || (index === 0 ? "默认渠道" : `渠道 ${index + 1}`),
            models: uniqueRawModels(channel.models || []),
        }),
    );
    if (!channels.length) {
        channels.push(
            createModelChannel({
                id: "default",
                name: "默认渠道",
                baseUrl: config.baseUrl || defaultConfig.baseUrl,
                apiKey: config.apiKey || "",
                models: uniqueRawModels([
                    ...(config.models || []),
                    config.model,
                    config.imageModel,
                    config.videoModel,
                    config.textModel,
                    config.audioModel,
                ]),
            }),
        );
    }
    return channels.map((channel) => ({ ...channel, models: uniqueRawModels(channel.models) }));
}

function uniqueRawModels(models: string[]) {
    return Array.from(new Set((models || []).map((model) => modelOptionName(model).trim()).filter(Boolean)));
}

function uniqueModelOptions(models: string[]) {
    return Array.from(new Set((models || []).map((model) => model.trim()).filter(Boolean)));
}

function normalizeServerModelCatalog(models?: string[]) {
    return uniqueModelOptions((models || []).map((model) => modelOptionName(model)));
}

function normalizeModelRoutes(routes: Record<string, string> | undefined, models: string[]) {
    const knownModels = new Set((models || []).map(modelOptionName));
    const normalized: Record<string, string> = {};
    for (const [key, route] of Object.entries(routes || {})) {
        const routeName = String(route || "").trim();
        if (!routeName || routeName === "auto") continue;
        const separatorIndex = key.indexOf(":");
        const prefix = separatorIndex >= 0 ? key.slice(0, separatorIndex) : "";
        const rawModel = separatorIndex >= 0 ? key.slice(separatorIndex + 1) : key;
        const capability = prefix === "image" || prefix === "image_generate" || prefix === "image_edit" || prefix === "video"
            ? prefix
            : inferRouteCapability(routeName);
        const modelName = modelOptionName(rawModel).trim();
        if (!modelName || !capability) continue;
        if (knownModels.size && !knownModels.has(modelName)) continue;
        normalized[modelRouteKey(capability, modelName)] = routeName;
    }
    return normalized;
}

function normalizeModelVideoDurations(items: Record<string, number[]> | undefined, models: string[]) {
    const knownModels = new Set((models || []).map(modelOptionName));
    const normalized: Record<string, number[]> = {};
    for (const [key, values] of Object.entries(items || {})) {
        const model = modelOptionName(key).trim();
        if (!model) continue;
        if (knownModels.size && !knownModels.has(model)) continue;
        const durations = Array.from(new Set((values || []).map((item) => Math.floor(Number(item) || 0)).filter((item) => item > 0))).sort((left, right) => left - right);
        if (!durations.length) continue;
        normalized[model] = durations;
    }
    return normalized;
}

function normalizeModelVideoCustomizable(items: Record<string, boolean> | undefined, models: string[]) {
    const knownModels = new Set((models || []).map(modelOptionName));
    const normalized: Record<string, boolean> = {};
    for (const [key, value] of Object.entries(items || {})) {
        const model = modelOptionName(key).trim();
        if (!model) continue;
        if (knownModels.size && !knownModels.has(model)) continue;
        if (!value) continue;
        normalized[model] = true;
    }
    return normalized;
}

function pickServerDefaultModel(currentValue: string, primaryOptions: string[], fallbackOptions: string[]) {
    const currentModelName = modelOptionName(currentValue || "");
    if (currentModelName && primaryOptions.includes(currentModelName)) return currentModelName;
    if (currentModelName && fallbackOptions.includes(currentModelName)) return currentModelName;
    return primaryOptions[0] || fallbackOptions[0] || currentModelName || "";
}

export function buildApiUrl(baseUrl: string, path: string) {
    let normalizedBaseUrl = baseUrl.trim().replace(/\/+$/, "");
    normalizedBaseUrl = normalizeArkPlanBaseUrl(normalizedBaseUrl);
    const lowerBaseUrl = normalizedBaseUrl.toLowerCase();
    const apiBaseUrl = lowerBaseUrl.endsWith("/v1") || lowerBaseUrl.endsWith("/api/v3") || lowerBaseUrl.endsWith("/api/plan/v3") ? normalizedBaseUrl : `${normalizedBaseUrl}/v1`;
    return `${apiBaseUrl}${path}`;
}

function normalizeArkPlanBaseUrl(baseUrl: string) {
    try {
        const url = new URL(baseUrl);
        const path = url.pathname.replace(/\/+$/, "");
        const lowerPath = path.toLowerCase();
        const arkPlanIndex = lowerPath.indexOf("/api/plan/v3");
        if (arkPlanIndex < 0) return baseUrl;
        const end = arkPlanIndex + "/api/plan/v3".length;
        if (lowerPath.length !== end && lowerPath[end] !== "/") return baseUrl;
        url.pathname = path.slice(0, end);
        url.search = "";
        url.hash = "";
        return url.toString().replace(/\/+$/, "");
    } catch {
        return baseUrl;
    }
}
