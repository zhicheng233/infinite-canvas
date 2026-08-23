import { customVideoConfigForModel, videoRouteForModel, type AiConfig } from "@/stores/use-config-store";
import { customVideoMediaFeatureNames, type CustomVideoConfig, type CustomVideoMediaFeature } from "@/lib/custom-video-config";
import {
    createEmptyCustomVideoMediaState,
    customVideoRequiredMediaErrors,
    normalizeCustomVideoRuntimeState,
    normalizeCustomVideoRuntimeStateForModelSwitch,
    normalizePersistedCustomVideoMedia,
    type CustomVideoMediaState,
    type CustomVideoRuntimeSnapshot,
} from "@/lib/custom-video-runtime";
import { resolveNodeGenerationImageSources, type NodeGenerationImageInput, type NodeGenerationImageSource } from "./canvas-node-generation";
import { canvasImageRoleLabel } from "../utils/canvas-connection-targets";

export type CanvasCustomVideoGenerationState = {
    readonly runtime?: CustomVideoRuntimeSnapshot;
    readonly error?: string;
};

export type CanvasCustomVideoGenerationInput = {
    readonly config: AiConfig;
    readonly model: string;
    readonly runtime?: CustomVideoRuntimeSnapshot;
    readonly graphImages: readonly NodeGenerationImageInput[];
};

export function canvasCustomVideoRuntimeForModel(config: AiConfig, model: string, runtime: CustomVideoRuntimeSnapshot | undefined) {
    const customConfig = customVideoConfigForModel(config, model);
    return customConfig ? normalizeCustomVideoRuntimeStateForModelSwitch(customConfig, runtime?.values, runtime?.media) : undefined;
}

export function canvasCustomVideoRuntimeForHydration(config: AiConfig, model: string, runtime: CustomVideoRuntimeSnapshot | undefined) {
    const customConfig = customVideoConfigForModel(config, model);
    if (!customConfig) return undefined;
    const normalized = normalizeCustomVideoRuntimeStateForModelSwitch(customConfig, runtime?.values, runtime?.media);
    return { values: normalized.values, media: normalizePersistedCustomVideoMedia(runtime?.media) } satisfies CustomVideoRuntimeSnapshot;
}

export function canvasCustomVideoGenerationState(config: AiConfig, model: string, runtime: CustomVideoRuntimeSnapshot | undefined): CanvasCustomVideoGenerationState {
    if (videoRouteForModel(config, model) !== "custom") return {};
    const customConfig = customVideoConfigForModel(config, model);
    if (!customConfig) return { error: "该模型的自定义视频配置无效，请联系管理员" };
    const normalized = canvasCustomVideoRuntimeForModel(config, model, runtime);
    return { runtime: normalized, error: customVideoRequiredMediaErrors(customConfig, normalized?.media)[0]?.message };
}

export async function resolveCanvasCustomVideoGenerationState(input: CanvasCustomVideoGenerationInput): Promise<CanvasCustomVideoGenerationState> {
    if (videoRouteForModel(input.config, input.model) !== "custom") return {};
    const customConfig = customVideoConfigForModel(input.config, input.model);
    if (!customConfig) return { error: "该模型的自定义视频配置无效，请联系管理员" };
    const media = mergeCanvasCustomVideoMedia(customConfig, input.runtime, await resolveNodeGenerationImageSources(input.graphImages));
    if ("error" in media) return media;
    const normalized = normalizeCustomVideoRuntimeState(customConfig, input.runtime?.values, media);
    return { runtime: normalized, error: customVideoRequiredMediaErrors(customConfig, normalized.media)[0]?.message };
}

export function canvasVideoGenerationOptions(config: AiConfig, model: string, runtime: CustomVideoRuntimeSnapshot | undefined, signal: AbortSignal) {
    if (videoRouteForModel(config, model) !== "custom") return { signal };
    return { signal, customVideoRuntime: canvasCustomVideoRuntimeForModel(config, model, runtime) };
}

function mergeCanvasCustomVideoMedia(config: CustomVideoConfig, runtime: CustomVideoRuntimeSnapshot | undefined, graphImages: readonly NodeGenerationImageSource[]): CustomVideoMediaState | { readonly error: string } {
    const empty = createEmptyCustomVideoMediaState();
    const media: Record<CustomVideoMediaFeature, string[]> = {
        images: [...empty.images],
        input_reference: [...empty.input_reference],
        style_references: [...empty.style_references],
        element_references: [...empty.element_references],
        reference_images: [...empty.reference_images],
        input_video: [...empty.input_video],
    };
    customVideoMediaFeatureNames.forEach((role) => media[role].push(...normalizedSourcesForRole(config, role, runtime?.media[role] || [])));

    for (const image of graphImages) {
        const role = image.targetImageRole || "images";
        if (!image.targetImageRole && !config.images.enabled) return { error: "旧图片连线无法用于当前模型，请重新连接到可用的图片角色入口" };
        if (!config[role].enabled) return { error: `图片角色“${canvasImageRoleLabel(role)}”当前不可用，请切换模型或重新连接` };
        const source = image.source ? normalizedSourcesForRole(config, role, [image.source])[0] : undefined;
        if (!source) return { error: `连接图片“${image.title}”没有可序列化来源，请重新上传或连接其他图片` };
        media[role].push(source);
    }

    for (const role of customVideoMediaFeatureNames) {
        media[role] = [...new Set(media[role])];
        if (media[role].length > config[role].max_count) return { error: `${customVideoMediaRoleLabel(role)}最多支持 ${config[role].max_count} 个素材，当前共 ${media[role].length} 个，请删除多余素材` };
    }
    return media;
}

function normalizedSourcesForRole(config: CustomVideoConfig, role: CustomVideoMediaFeature, sources: readonly string[]) {
    if (!config[role].enabled) return [];
    return sources.flatMap((source) => normalizeCustomVideoRuntimeState(config, undefined, { [role]: [source] }).media[role]);
}

function customVideoMediaRoleLabel(role: CustomVideoMediaFeature) {
    return role === "input_video" ? "源视频" : canvasImageRoleLabel(role);
}
