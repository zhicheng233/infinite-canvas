"use client";

import { useMemo } from "react";
import { create } from "zustand";
import { persist } from "zustand/middleware";
import type { AutoChannelModelInfo, ChannelInfo, ChannelModelInfo } from "@/services/api/channel";
import type { MergeGroup } from "@/services/api/merge-groups-admin";
import type { MetricsResponse, ModelMetrics } from "@/services/api/metrics";
import type { PricingItem } from "@/services/api/pricing";

export type ModelChannel = {
    id: number;
    name: string;
    enabled: boolean;
};

export type ServerChannelModel = ChannelModelInfo;

export type ChannelModelOption = {
    value: string;
    channelId: number;
    channelModelId: number;
    channelName: string;
    rawModel: string;
    capability: ModelCapability;
    price: PricingItem;
    successRate: number | null;
    metricsStatus: string;
    imageGenerateRoute: string;
    imageEditRoute: string;
    videoRoute: string;
    videoDurations: number[];
    videoCustomizable: boolean;
    sortOrder: number;
};

export type LocalAiCredentials = {
    baseUrl: string;
    apiKey: string;
};

export type AiConfig = {
    imageChannelId: number | null;
    videoChannelId: number | null;
    textChannelId: number | null;
    audioChannelId: number | null;
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
    channelModelId?: number | null;
};

export type WebdavSyncConfig = {
    proxyMode: "direct" | "nextjs";
    url: string;
    username: string;
    password: string;
    directory: string;
    lastSyncedAt: string;
};

export function persistedConfigState(state: { config: AiConfig; webdav: WebdavSyncConfig }) {
    const config = { ...state.config } as Record<string, unknown>;
    for (const key of ["models", "imageModels", "videoModels", "textModels", "audioModels", "modelRoutes", "modelVideoDurations", "modelVideoCustomizable", "channelModelId", "apiKey", "baseUrl", "channels", "channelMode"]) {
        delete config[key];
    }
    return { config, webdav: state.webdav };
}

export const CONFIG_STORE_KEY = "infinite-canvas:ai_config_store";
const LOCAL_AI_CREDENTIALS_KEY = "infinite-canvas:local_ai_credentials";
export type ModelCapability = "image" | "video" | "text" | "audio";
export type ModelRouteCapability = "image" | "image_generate" | "image_edit" | "video";
export type ImageRouteMode = "auto" | "generations" | "edits" | "chat" | "banana";
export type VideoRouteMode = "auto" | "openai" | "veo_json" | "waninter" | "yijia" | "xai" | "newapi" | "seedance";
const CHANNEL_MODEL_SEPARATOR = "::";
const IMAGE_ROUTE_VALUES = new Set<string>(["generations", "edits", "chat", "banana"]);
const VIDEO_ROUTE_VALUES = new Set<string>(["openai", "veo_json", "waninter", "yijia", "xai", "newapi", "seedance"]);

export const defaultConfig: AiConfig = {
    imageChannelId: null,
    videoChannelId: null,
    textChannelId: null,
    audioChannelId: null,
    model: "gpt-image-2",
    imageModel: "gpt-image-2",
    videoModel: "grok-imagine-video",
    textModel: "gpt-5.5",
    audioModel: "gpt-4o-mini-tts",
    audioVoice: "alloy",
    audioFormat: "mp3",
    audioSpeed: "1",
    audioInstructions: "",
    videoSeconds: "6",
    vquality: "720",
    videoGenerateAudio: "true",
    videoWatermark: "false",
    systemPrompt: "",
    models: ["gpt-image-2", "grok-imagine-video", "gpt-5.5", "gpt-4o-mini-tts"],
    imageModels: ["gpt-image-2"],
    videoModels: ["grok-imagine-video"],
    textModels: ["gpt-5.5"],
    audioModels: ["gpt-4o-mini-tts"],
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
    serverChannels: ModelChannel[];
    serverChannelModels: Record<number, ServerChannelModel[]>;
    serverPricing: PricingItem[];
    serverMetrics: MetricsResponse | null;
    serverCatalogLoading: boolean;
    serverCatalogError: string | null;
    autoChannelModels: AutoChannelModelInfo[];
    serverMergeGroups: Record<number, MergeGroup[]>;
    isConfigOpen: boolean;
    shouldPromptContinue: boolean;
    updateConfig: <K extends keyof AiConfig>(key: K, value: AiConfig[K]) => void;
    updateWebdavConfig: <K extends keyof WebdavSyncConfig>(key: K, value: WebdavSyncConfig[K]) => void;
    isAiConfigReady: (config: AiConfig, model: string) => boolean;
    setServerChannels: (channels: ModelChannel[]) => void;
    setServerCatalogLoading: (loading: boolean) => void;
    setServerCatalogError: (error: string | null) => void;
    applyServerOptionMetadata: (pricing: PricingItem[], metrics: MetricsResponse | null) => void;
    applyServerModelCatalog: (catalog: {
        models?: string[];
        imageModels?: string[];
        videoModels?: string[];
        textModels?: string[];
        audioModels?: string[];
        modelRoutes?: Record<string, string>;
        modelVideoDurations?: Record<string, number[]>;
        modelVideoCustomizable?: Record<string, boolean>;
        channels?: ModelChannel[];
    }) => void;
    applyServerChannelCatalog: (channels: ChannelInfo[], channelModels: Record<number, ChannelModelInfo[]>) => void;
    applyAutoChannelModels: (models: AutoChannelModelInfo[]) => void;
    applyServerMergeGroups: (channelId: number, groups: MergeGroup[]) => void;
    selectCapabilityChannel: (capability: ModelCapability, channelId: number | null) => void;
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
    return (
        !isVideoModelName(model) &&
        !isAudioModelName(model) &&
        (value.includes("seedream") ||
            value.includes("gpt-image") ||
            value.includes("image") ||
            value.includes("dall-e") ||
            value.includes("dalle") ||
            value.includes("imagen") ||
            value.includes("flux") ||
            value.includes("sdxl") ||
            value.includes("stable-diffusion") ||
            value.includes("midjourney"))
    );
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

export function selectedChannelId(config: AiConfig, capability: ModelCapability): number | null {
    if (capability === "image") return config.imageChannelId;
    if (capability === "video") return config.videoChannelId;
    if (capability === "audio") return config.audioChannelId;
    return config.textChannelId;
}

export function selectedChannelIdentityForModel(config: AiConfig, value: string): { channelId: number; channelModelId: number } | null {
    const mergeParsed = parseMergeModelValue(value);
    if (mergeParsed) return { channelId: mergeParsed.channelId, channelModelId: 0 };
    const decoded = decodeChannelModel(value);
    if (decoded && decoded.channelId === "0") return { channelId: 0, channelModelId: 0 };
    const selectedModel: ServerChannelModel | null = config.channelModelId ? findChannelModelById(config.channelModelId) : null;
    if (selectedModel && selectedModel.model_name === modelOptionName(value)) return { channelId: selectedModel.channel_id, channelModelId: selectedModel.id };
    const decodedChannelId = toNullableChannelId(decoded?.channelId);
    const channelModelId = selectedChannelModelId(config, value);
    if (decodedChannelId && channelModelId) return { channelId: decodedChannelId, channelModelId };
    const channelId = selectedChannelId(config, capabilityForModel(config, value));
    return channelId && channelModelId ? { channelId, channelModelId } : null;
}

export function buildProxyApiUrl(apiBase: string, config: AiConfig, value: string, path: string) {
    const mergeParsed = parseMergeModelValue(value);
    if (mergeParsed) {
        return `${apiBase}/proxy?${new URLSearchParams({ path, channel_id: String(mergeParsed.channelId), fuzzy_group_name: mergeParsed.groupName }).toString()}`;
    }
    const identity = selectedChannelIdentityForModel(config, value);
    if (!identity) throw new Error("所选模型已失效，请刷新后重新选择");
    const query = new URLSearchParams({ path, channel_id: String(identity.channelId), channel_model_id: String(identity.channelModelId) });
    return `${apiBase}/proxy?${query.toString()}`;
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

function channelIdKey(capability: ModelCapability) {
    return `${capability}ChannelId` as "imageChannelId" | "videoChannelId" | "textChannelId" | "audioChannelId";
}

function capabilityForModel(config: AiConfig, value: string): ModelCapability {
    for (const capability of ["image", "video", "text", "audio"] as ModelCapability[]) {
        if (selectableModelsByCapability(config, capability).includes(value)) return capability;
    }
    const model = modelOptionName(value || "");
    if (modelIncludedInCapabilityList(config, "image", model) || modelMatchesCapability(model, "image")) return "image";
    if (modelIncludedInCapabilityList(config, "video", model) || modelMatchesCapability(model, "video")) return "video";
    if (modelIncludedInCapabilityList(config, "audio", model) || modelMatchesCapability(model, "audio")) return "audio";
    return "text";
}

function isAiConfigReady(config: AiConfig, model: string): boolean {
    // Logged-in users go through server proxy (admin configures API)
    if (typeof window !== "undefined" && window.localStorage.getItem("infinite-canvas:auth_token")) {
        return Boolean(model.trim() && selectedChannelIdentityForModel(config, model));
    }
    const local = readLocalAiCredentials();
    return Boolean(model.trim() && local.baseUrl.trim() && local.apiKey.trim());
}

let latestServerChannels: ModelChannel[] = [];

export const useConfigStore = create<ConfigStore>()(
    persist(
        (set, get) => ({
            config: defaultConfig,
            webdav: defaultWebdavSyncConfig,
            serverChannels: [] as ModelChannel[],
            serverChannelModels: {},
            serverPricing: [] as PricingItem[],
            serverMetrics: null,
            serverCatalogLoading: false,
            serverCatalogError: null,
            autoChannelModels: [] as AutoChannelModelInfo[],
            serverMergeGroups: {} as Record<number, MergeGroup[]>,
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
            setServerChannels: (channels) =>
                set(() => {
                    const serverChannels = normalizeServerChannels(channels);
                    latestServerChannels = serverChannels;
                    return { serverChannels };
                }),
            setServerCatalogLoading: (serverCatalogLoading) => set({ serverCatalogLoading }),
            setServerCatalogError: (serverCatalogError) => set({ serverCatalogError }),
            applyServerOptionMetadata: (serverPricing, serverMetrics) =>
                set((state) => ({
                    serverPricing,
                    serverMetrics,
                    config: applyChannelScopedSelections(state.config, state.serverChannels, state.serverChannelModels, serverPricing, serverMetrics),
                })),
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
                    const serverChannels = catalog.channels ? normalizeServerChannels(catalog.channels) : state.serverChannels;
                    latestServerChannels = serverChannels;
                    return {
                        serverChannels,
                        serverCatalogError: null,
                        config: {
                            ...state.config,
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
                            imageChannelId: normalizeSelectedChannelId(state.config.imageChannelId, serverChannels),
                            videoChannelId: normalizeSelectedChannelId(state.config.videoChannelId, serverChannels),
                            textChannelId: normalizeSelectedChannelId(state.config.textChannelId, serverChannels),
                            audioChannelId: normalizeSelectedChannelId(state.config.audioChannelId, serverChannels),
                        },
                    };
                }),
            applyServerChannelCatalog: (channels, channelModels) =>
                set((state) => {
                    const serverChannels = normalizeServerChannels(channels);
                    const serverChannelModels = normalizeServerChannelModels(channelModels, serverChannels);
                    latestServerChannels = serverChannels;
                    return {
                        serverChannels,
                        serverChannelModels,
                        serverCatalogError: null,
                        config: applyChannelScopedSelections(state.config, serverChannels, serverChannelModels, state.serverPricing, state.serverMetrics),
                    };
                }),
            applyAutoChannelModels: (autoChannelModels) => set({ autoChannelModels }),
            applyServerMergeGroups: (channelId, groups) =>
                set((state) => ({
                    serverMergeGroups: { ...state.serverMergeGroups, [channelId]: groups },
                })),
            selectCapabilityChannel: (capability, channelId) =>
                set((state) => {
                    const next = { ...state.config, [channelIdKey(capability)]: channelId };
                    const selectedChannelId = normalizeSelectedChannelId(channelId, state.serverChannels);
                    if (channelId === 0) {
                        const options = buildChannelModelOptions(state.serverChannels, state.serverChannelModels, state.serverPricing, state.serverMetrics, capability, 0, state.autoChannelModels).map((o) => o.value);
                        next[modelListKey(capability)] = options;
                        next[`${capability}Model`] = options.includes(next[`${capability}Model`]) ? next[`${capability}Model`] : options[0] || "";
                    } else if (selectedChannelId && state.serverChannelModels[selectedChannelId]) {
                        const mergeGroups = state.serverMergeGroups[selectedChannelId];
                        const options = buildChannelModelOptions(state.serverChannels, state.serverChannelModels, state.serverPricing, state.serverMetrics, capability, selectedChannelId, state.autoChannelModels, mergeGroups).map((o) => o.value);
                        next[modelListKey(capability)] = options;
                        const currentModel = next[`${capability}Model`];
                        next[`${capability}Model`] = options.includes(currentModel) ? currentModel : options[0] || "";
                    } else {
                        next[modelListKey(capability)] = [];
                        next[`${capability}Model`] = "";
                    }
                    return { config: next };
                }),
            openConfigDialog: (shouldPromptContinue = false) => set({ isConfigOpen: true, shouldPromptContinue }),
            setConfigDialogOpen: (isConfigOpen) => set({ isConfigOpen }),
            clearPromptContinue: () => set({ shouldPromptContinue: false }),
        }),
        {
            name: CONFIG_STORE_KEY,
            partialize: persistedConfigState,
            merge: (persisted, current) => {
                const persistedState = (persisted || {}) as Partial<ConfigStore>;
                const persistedConfig = (persistedState.config || {}) as Partial<AiConfig> & Record<string, unknown>;
                const persistedWebdav = (persistedState.webdav || {}) as Partial<WebdavSyncConfig>;
                const { baseUrl: _baseUrl, apiKey: _apiKey, channels: _channels, channelMode: _channelMode, ...safePersistedConfig } = persistedConfig;
                if (_baseUrl !== undefined || _apiKey !== undefined || _channels !== undefined || _channelMode !== undefined) sanitizePersistedConfigStorage();
                const config = { ...defaultConfig, ...safePersistedConfig } as AiConfig;
                const models = Array.isArray(config.models) && config.models.length ? normalizeServerModelCatalog(config.models) : defaultConfig.models;
                return {
                    ...current,
                    webdav: { ...defaultWebdavSyncConfig, ...persistedWebdav },
                    config: {
                        ...config,
                        models,
                        imageChannelId: toNullableChannelId(config.imageChannelId),
                        videoChannelId: toNullableChannelId(config.videoChannelId),
                        textChannelId: toNullableChannelId(config.textChannelId),
                        audioChannelId: toNullableChannelId(config.audioChannelId),
                        model: normalizeModelOptionValue(config.model || defaultConfig.model),
                        imageModel: normalizeModelOptionValue(config.imageModel || config.model),
                        videoModel: normalizeModelOptionValue(config.videoModel || "grok-imagine-video"),
                        textModel: normalizeModelOptionValue(config.textModel || config.model),
                        audioModel: normalizeModelOptionValue(config.audioModel || defaultConfig.audioModel),
                        audioVoice: config.audioVoice || defaultConfig.audioVoice,
                        audioFormat: config.audioFormat || defaultConfig.audioFormat,
                        audioSpeed: config.audioSpeed || defaultConfig.audioSpeed,
                        audioInstructions: config.audioInstructions || "",
                        videoSeconds: config.videoSeconds || "6",
                        vquality: config.vquality || "720",
                        videoGenerateAudio: config.videoGenerateAudio || "true",
                        videoWatermark: config.videoWatermark || "false",
                        canvasImageCount: config.canvasImageCount || "1",
                        imageModels: Array.isArray(persistedConfig.imageModels) ? normalizeModelList(config.imageModels) : filterModelsByCapability(models, "image"),
                        videoModels: Array.isArray(persistedConfig.videoModels) ? normalizeModelList(config.videoModels) : filterModelsByCapability(models, "video"),
                        textModels: Array.isArray(persistedConfig.textModels) ? normalizeModelList(config.textModels) : filterModelsByCapability(models, "text"),
                        audioModels: Array.isArray(persistedConfig.audioModels) ? normalizeModelList(config.audioModels) : filterModelsByCapability(models, "audio"),
                        modelRoutes: normalizeModelRoutes(config.modelRoutes, models),
                        modelVideoDurations: normalizeModelVideoDurations(config.modelVideoDurations, models),
                        modelVideoCustomizable: normalizeModelVideoCustomizable(config.modelVideoCustomizable, models),
                    },
                };
            },
        },
    ),
);

function normalizeModelList(models: string[]) {
    return Array.from(new Set((models || []).map((model) => model.trim()).filter(Boolean)))
        .map((model) => normalizeModelOptionValue(model))
        .filter(Boolean);
}

export function useEffectiveConfig() {
    const config = useConfigStore((state) => state.config);
    return useMemo(() => config, [config]);
}

export function createModelChannel(channel?: Partial<ModelChannel>): ModelChannel {
    return {
        id: toNullableChannelId(channel?.id) || 0,
        name: channel?.name?.trim() || "新渠道",
        enabled: channel?.enabled !== false,
    };
}

export function encodeChannelModel(channelId: string | number, model: string) {
    return `${channelId}${CHANNEL_MODEL_SEPARATOR}${model.trim()}`;
}

export function encodeChannelModelIdentity(channelId: string | number, channelModelId: string | number, model: string) {
    return `${channelId}${CHANNEL_MODEL_SEPARATOR}${channelModelId}${CHANNEL_MODEL_SEPARATOR}${model.trim()}`;
}

export function isChannelModelValue(value: string) {
    return value.includes(CHANNEL_MODEL_SEPARATOR);
}

export function decodeChannelModel(value: string) {
    const index = value.indexOf(CHANNEL_MODEL_SEPARATOR);
    if (index < 0) return null;
    const rest = value.slice(index + CHANNEL_MODEL_SEPARATOR.length);
    const modelIndex = rest.indexOf(CHANNEL_MODEL_SEPARATOR);
    if (modelIndex < 0) return { channelId: value.slice(0, index), channelModelId: null, model: rest };
    return { channelId: value.slice(0, index), channelModelId: toNullableChannelId(rest.slice(0, modelIndex)), model: rest.slice(modelIndex + CHANNEL_MODEL_SEPARATOR.length) };
}

export function isMergeModelValue(value: string): boolean {
    return value.startsWith("merge://");
}

export function parseMergeModelValue(value: string): { channelId: number; groupName: string } | null {
    if (!value.startsWith("merge://")) return null;
    const parts = value.replace("merge://", "").split("::");
    if (parts.length !== 2) return null;
    const channelId = parseInt(parts[0], 10);
    if (!channelId || Number.isNaN(channelId)) return null;
    return { channelId, groupName: parts[1] };
}

export function modelOptionName(value: string) {
    if (isMergeModelValue(value)) return parseMergeModelValue(value)?.groupName || value;
    return decodeChannelModel(value)?.model || value;
}

export function fixedVideoDurationForModel(value: string) {
    return null;
}

export function videoDurationOptionsForModel(config: Pick<AiConfig, "modelVideoDurations">, value: string) {
    const model = value.trim();
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
    const model = value.trim();
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
    return `${capability}:${model.trim()}`;
}

function inferRouteCapability(route: string): ModelRouteCapability | "" {
    if (IMAGE_ROUTE_VALUES.has(route)) return "image";
    if (VIDEO_ROUTE_VALUES.has(route)) return "video";
    return "";
}

export function modelRouteForCapability(config: Pick<AiConfig, "modelRoutes">, capability: ModelRouteCapability, value: string) {
    const model = value.trim();
    if (!model) return "auto";
    return config.modelRoutes?.[modelRouteKey(capability, model)] || "auto";
}

export function imageRouteForModel(config: Pick<AiConfig, "modelRoutes">, value: string) {
    return modelRouteForCapability(config, "image", value) as ImageRouteMode;
}

export function imageGenerateRouteForModel(config: Pick<AiConfig, "modelRoutes">, value: string) {
    const model = value.trim();
    if (!model) return "auto" as ImageRouteMode;
    return (config.modelRoutes?.[modelRouteKey("image_generate", model)] || "auto") as ImageRouteMode;
}

export function imageEditRouteForModel(config: Pick<AiConfig, "modelRoutes">, value: string) {
    const model = value.trim();
    if (!model) return "auto" as ImageRouteMode;
    return (config.modelRoutes?.[modelRouteKey("image_edit", model)] || config.modelRoutes?.[modelRouteKey("image", model)] || "auto") as ImageRouteMode;
}

export function videoRouteForModel(config: Pick<AiConfig, "modelRoutes">, value: string) {
    return modelRouteForCapability(config, "video", value) as VideoRouteMode;
}

export function modelOptionLabel(config: AiConfig, value: string) {
    const mergeParsed = parseMergeModelValue(value);
    if (mergeParsed) {
        const channel = latestServerChannels.find((item) => item.id === mergeParsed.channelId);
        return channel ? `${mergeParsed.groupName}（${channel.name}）` : mergeParsed.groupName;
    }
    const decoded = decodeChannelModel(value);
    if (!decoded) return value;
    const channelId = toNullableChannelId(decoded.channelId);
    const channel = latestServerChannels.find((item) => item.id === channelId);
    return channel ? `${decoded.model}（${channel.name}）` : decoded.model;
}

export function channelModelOptionsByCapability(capability: ModelCapability, channelId?: number | null) {
    const state = useConfigStore.getState();
    const resolvedChannelId = channelId ?? selectedChannelId(state.config, capability);
    const mergeGroups = resolvedChannelId ? state.serverMergeGroups[resolvedChannelId] : undefined;
    return buildChannelModelOptions(state.serverChannels, state.serverChannelModels, state.serverPricing, state.serverMetrics, capability, resolvedChannelId, state.autoChannelModels, mergeGroups);
}

export function buildChannelModelOptions(
    channels: ModelChannel[],
    channelModels: Record<number, ServerChannelModel[]>,
    pricing: PricingItem[],
    metrics: MetricsResponse | null,
    capability: ModelCapability,
    channelId?: number | null,
    autoChannelModels?: AutoChannelModelInfo[],
    mergeGroups?: MergeGroup[],
): ChannelModelOption[] {
    if (channelId === 0 && autoChannelModels?.length) {
        const prices = new Map(pricing.map((item) => [item.model, item]));
        return autoChannelModels
            .filter((am) => modelMatchesCapability(am.model, capability))
            .map((am) => {
                const bestChannel = am.channels.reduce<AutoChannelModelRef | null>((best, curr) => (curr.success_rate > (best?.success_rate ?? -1) ? curr : best), null);
                return {
                    value: encodeChannelModelIdentity(0, 0, am.model),
                    channelId: 0,
                    channelModelId: 0,
                    channelName: "自动",
                    rawModel: am.model,
                    capability,
                    price: prices.get(am.model) || null,
                    successRate: bestChannel?.success_rate ?? null,
                    metricsStatus: bestChannel ? "ok" : "unavailable",
                    imageGenerateRoute: "auto",
                    imageEditRoute: "auto",
                    videoRoute: "auto",
                    videoDurations: [],
                    videoCustomizable: false,
                    sortOrder: 0,
                };
            })
            .filter((option) => option.price !== null)
            .sort((a, b) => {
                if (a.successRate === null && b.successRate !== null) return 1;
                if (a.successRate !== null && b.successRate === null) return -1;
                if (a.successRate !== null && b.successRate !== null && a.successRate !== b.successRate) return b.successRate - a.successRate;
                return a.rawModel.localeCompare(b.rawModel);
            });
    }
    const enabledChannels = new Map(channels.filter((channel) => channel.enabled).map((channel) => [channel.id, channel]));
    const pricesByKey = new Map<string, PricingItem>();
    for (const item of pricing) {
        const key = item.channel_id ? `${item.model}::${item.channel_id}` : item.model;
        pricesByKey.set(key, item);
    }
    const metricRows = new Map<number, ModelMetrics>();
    for (const channel of metrics?.channels || []) for (const model of channel.models || []) metricRows.set(model.channel_model_id, model);
    const options: ChannelModelOption[] = [];
    for (const model of Object.values(channelModels).flat()) {
        const channel = enabledChannels.get(model.channel_id);
        const channelKey = `${model.model_name}::${model.channel_id}`;
        const price = pricesByKey.get(channelKey) || pricesByKey.get(model.model_name) || null;
        if (!channel || (channelId && model.channel_id !== channelId) || !model.enabled || !price || !modelSupportsCapability(model, capability)) continue;
        const metric = metricRows.get(model.id);
        const metricsStatus = metric?.status || "unavailable";
        options.push({
            value: encodeChannelModelIdentity(model.channel_id, model.id, model.model_name),
            channelId: model.channel_id,
            channelModelId: model.id,
            channelName: channel.name,
            rawModel: model.model_name,
            capability,
            price,
            successRate: metricsStatus === "ok" && typeof metric?.success_rate === "number" ? metric.success_rate : null,
            metricsStatus,
            imageGenerateRoute: model.image_generate_route || "auto",
            imageEditRoute: model.image_edit_route || "auto",
            videoRoute: model.video_route || "auto",
            videoDurations: model.video_durations || [],
            videoCustomizable: model.video_customizable,
            sortOrder: model.sort_order,
        });
    }
    if (mergeGroups?.length && channelId && channelId > 0) {
        // Track which rawModel names are consumed by merge groups
        const consumedModelNames = new Set<string>();
        const mergedOptions: ChannelModelOption[] = [];

        for (const group of mergeGroups) {
            if (!group.enabled) continue;
            // Find models whose model_name starts with the group pattern
            const matchingModels = options.filter((opt) => opt.channelId === channelId && opt.rawModel.startsWith(group.pattern));
            if (!matchingModels.length) continue;

            for (const m of matchingModels) consumedModelNames.add(m.rawModel);

            const avgSuccessRate = matchingModels.some((m) => m.successRate !== null)
                ? Math.round(matchingModels.reduce((sum, m) => sum + (m.successRate ?? 0), 0) / matchingModels.length)
                : null;

            const bestMetricsStatus = matchingModels.some((m) => m.metricsStatus === "ok") ? "ok" : "unavailable";

            const channelName = channels.find((c) => c.id === channelId)?.name || "";
            mergedOptions.push({
                value: `merge://${channelId}::${group.group_name}`,
                channelId,
                channelModelId: 0,
                channelName,
                rawModel: group.group_name,
                capability,
                price: matchingModels[0]?.price || null,
                successRate: avgSuccessRate,
                metricsStatus: bestMetricsStatus,
                imageGenerateRoute: "auto",
                imageEditRoute: "auto",
                videoRoute: "auto",
                videoDurations: [],
                videoCustomizable: false,
                sortOrder: -1,
            });
        }

        if (mergedOptions.length) {
            // Filter out individual models consumed by merge groups
            const remaining = options.filter((opt) => !consumedModelNames.has(opt.rawModel));
            // Sort merged options first (sortOrder -1), then remaining by normal criteria
            return [...mergedOptions.sort((a, b) => a.rawModel.localeCompare(b.rawModel)), ...remaining.sort((left, right) => {
                if (left.successRate === null && right.successRate !== null) return 1;
                if (left.successRate !== null && right.successRate === null) return -1;
                if (left.successRate !== null && right.successRate !== null && left.successRate !== right.successRate) return right.successRate - left.successRate;
                return left.sortOrder - right.sortOrder || left.rawModel.localeCompare(right.rawModel) || left.channelModelId - right.channelModelId;
            })];
        }
    }
    return options.sort((left, right) => {
        if (left.successRate === null && right.successRate !== null) return 1;
        if (left.successRate !== null && right.successRate === null) return -1;
        if (left.successRate !== null && right.successRate !== null && left.successRate !== right.successRate) return right.successRate - left.successRate;
        return left.sortOrder - right.sortOrder || left.rawModel.localeCompare(right.rawModel) || left.channelModelId - right.channelModelId;
    });
}

export function modelOptionsFromChannels(_channels: ModelChannel[]) {
    return [];
}

export function normalizeModelOptionValue(value: string | undefined, _channels: ModelChannel[] = []) {
    const model = (value || "").trim();
    if (!model) return "";
    return model;
}

export function resolveModelChannel(config: AiConfig, value: string) {
    const decoded = decodeChannelModel(value);
    const decodedChannelId = toNullableChannelId(decoded?.channelId);
    const selectedId = decodedChannelId ?? selectedChannelId(config, capabilityForModel(config, value));
    return latestServerChannels.find((channel) => channel.id === selectedId) || latestServerChannels[0] || createModelChannel({ id: selectedId || 0, name: "默认渠道", enabled: true });
}

export function resolveModelRequestConfig(config: AiConfig, value: string): AiConfig {
    const selectedValue = value || config.model;
    const mergeParsed = parseMergeModelValue(selectedValue);
    if (mergeParsed) {
        const capability = capabilityForModel(config, mergeParsed.groupName);
        const next = { ...config, model: mergeParsed.groupName };
        next[channelIdKey(capability)] = mergeParsed.channelId;
        next.channelModelId = null;
        return next;
    }
    const decoded = decodeChannelModel(selectedValue);
    const rawModel = modelOptionName(selectedValue);
    const decodedChannelId = toNullableChannelId(decoded?.channelId);
    const capability = capabilityForModel(config, selectedValue);
    const channelId = decodedChannelId ?? selectedChannelId(config, capability);
    const channelModel = resolveChannelModel(channelId, rawModel, decoded?.channelModelId);
    const next = { ...config, model: isLoggedInInBrowser() && !channelModel ? "" : rawModel };
    if (channelId) next[channelIdKey(capability)] = channelId;
    if (channelModel) next.channelModelId = channelModel.id;
    return next;
}

export function selectedChannelModelId(config: AiConfig, value: string, capability?: ModelCapability) {
    const decoded = decodeChannelModel(value);
    const model = modelOptionName(value || "").trim();
    const selectedCapability = capability || capabilityForModel(config, model);
    const channelId = toNullableChannelId(decoded?.channelId) ?? selectedChannelId(config, selectedCapability);
    return resolveChannelModel(channelId, model, decoded?.channelModelId || config.channelModelId)?.id || null;
}

function uniqueRawModels(models: string[]) {
    return Array.from(new Set((models || []).map((model) => modelOptionName(model).trim()).filter(Boolean)));
}

function toNullableChannelId(value: unknown): number | null {
    if (value === null || value === undefined || value === "") return null;
    const id = Number(value);
    return Number.isInteger(id) && id > 0 ? id : null;
}

function normalizeServerChannels(channels?: ModelChannel[]) {
    return Array.from(
        new Map(
            (channels || [])
                .map((channel) => createModelChannel(channel))
                .filter((channel) => channel.id > 0 && channel.enabled)
                .map((channel) => [channel.id, channel]),
        ).values(),
    );
}

function normalizeServerChannelModels(items: Record<number, ChannelModelInfo[]>, channels: ModelChannel[]) {
    const enabledChannels = new Set(channels.map((channel) => channel.id));
    return Object.fromEntries(
        Object.entries(items || {}).map(([key, models]) => {
            const channelId = Number(key);
            const next = (models || []).filter((model) => enabledChannels.has(channelId) && model.enabled && model.channel_id === channelId && model.id > 0 && model.model_name.trim());
            return [channelId, next];
        }),
    );
}

function resolveChannelModel(channelId: number | null, modelName: string, modelId?: number | null) {
    if (!channelId) return null;
    const models = useConfigStore.getState().serverChannelModels[channelId] || [];
    return models.find((model) => (!modelId || model.id === modelId) && model.model_name === modelName) || null;
}

function findChannelModelById(modelId: number): ServerChannelModel | null {
    return (
        Object.values(useConfigStore.getState().serverChannelModels)
            .flat()
            .find((model) => model.id === modelId) || null
    );
}

function applyChannelScopedSelections(config: AiConfig, channels: ModelChannel[], models: Record<number, ServerChannelModel[]>, pricing: PricingItem[] = [], metrics: MetricsResponse | null = null) {
    const next = { ...config };
    for (const capability of ["image", "video", "text", "audio"] as ModelCapability[]) {
        const requestedChannelId = normalizeSelectedChannelId(config[channelIdKey(capability)], channels);
        const channelId = requestedChannelId && hasCapabilityModel(models[requestedChannelId], capability) ? requestedChannelId : channels.find((channel) => hasCapabilityModel(models[channel.id], capability))?.id || null;
        const options = channelId ? buildChannelModelOptions(channels, models, pricing, metrics, capability, channelId, []).map((option) => option.value) : [];
        next[channelIdKey(capability)] = channelId;
        next[modelListKey(capability)] = options;
        const current = next[`${capability}Model`];
        next[`${capability}Model`] = options.includes(current) ? current : options[0] || "";
    }
    next.models = Array.from(
        new Set(
            Object.values(models)
                .flat()
                .map((model) => model.model_name),
        ),
    ).sort();
    next.modelRoutes = {};
    next.modelVideoDurations = {};
    next.modelVideoCustomizable = {};
    for (const model of Object.values(models).flat()) {
        const option = encodeChannelModelIdentity(model.channel_id, model.id, model.model_name);
        if (model.image_generate_route && model.image_generate_route !== "auto") next.modelRoutes[modelRouteKey("image_generate", option)] = model.image_generate_route;
        if (model.image_edit_route && model.image_edit_route !== "auto") next.modelRoutes[modelRouteKey("image_edit", option)] = model.image_edit_route;
        if (model.video_route && model.video_route !== "auto") next.modelRoutes[modelRouteKey("video", option)] = model.video_route;
        if (model.video_durations.length) next.modelVideoDurations[option] = model.video_durations;
        if (model.video_customizable) next.modelVideoCustomizable[option] = true;
    }
    next.model = next.imageModel || next.videoModel || next.textModel || next.audioModel || "";
    return next;
}

function normalizeSelectedChannelId(value: unknown, channels: ModelChannel[]) {
    const id = toNullableChannelId(value);
    if (!id) return null;
    return channels.length && !channels.some((channel) => channel.id === id) ? null : id;
}

function hasCapabilityModel(models: ServerChannelModel[] | undefined, capability: ModelCapability) {
    return Boolean(models?.some((model) => model.enabled && modelSupportsCapability(model, capability)));
}

function modelSupportsCapability(model: ServerChannelModel, capability: ModelCapability) {
    return model.capabilities.includes(capability) || (!model.capabilities.length && modelMatchesCapability(model.model_name, capability));
}

function isLoggedInInBrowser() {
    return typeof window !== "undefined" && Boolean(window.localStorage.getItem("infinite-canvas:auth_token"));
}

export function readLocalAiCredentials(): LocalAiCredentials {
    if (typeof window === "undefined") return { baseUrl: "", apiKey: "" };
    try {
        const value = window.sessionStorage.getItem(LOCAL_AI_CREDENTIALS_KEY);
        if (!value) return { baseUrl: "", apiKey: "" };
        const parsed = JSON.parse(value) as Partial<LocalAiCredentials>;
        return { baseUrl: String(parsed.baseUrl || ""), apiKey: String(parsed.apiKey || "") };
    } catch {
        return { baseUrl: "", apiKey: "" };
    }
}

export function writeLocalAiCredentials(credentials: LocalAiCredentials) {
    if (typeof window === "undefined") return;
    const next = { baseUrl: credentials.baseUrl.trim(), apiKey: credentials.apiKey };
    if (!next.baseUrl && !next.apiKey) {
        window.sessionStorage.removeItem(LOCAL_AI_CREDENTIALS_KEY);
        return;
    }
    window.sessionStorage.setItem(LOCAL_AI_CREDENTIALS_KEY, JSON.stringify(next));
}

function sanitizePersistedConfigStorage() {
    if (typeof window === "undefined") return;
    queueMicrotask(() => {
        try {
            const raw = window.localStorage.getItem(CONFIG_STORE_KEY);
            if (!raw) return;
            const stored = JSON.parse(raw) as { state?: Partial<ConfigStore> & { config?: Record<string, unknown> }; version?: unknown };
            const config = stored.state?.config;
            if (!config) return;
            delete config.baseUrl;
            delete config.apiKey;
            delete config.channels;
            delete config.channelMode;
            window.localStorage.setItem(CONFIG_STORE_KEY, JSON.stringify(stored));
        } catch {}
    });
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
        const capability = prefix === "image" || prefix === "image_generate" || prefix === "image_edit" || prefix === "video" ? prefix : inferRouteCapability(routeName);
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
