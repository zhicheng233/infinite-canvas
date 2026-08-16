import { customVideoConfigForModel, videoRouteForModel, type AiConfig } from "@/stores/use-config-store";
import { customVideoRequiredMediaErrors, normalizeCustomVideoRuntimeStateForModelSwitch, type CustomVideoRuntimeSnapshot } from "@/lib/custom-video-runtime";

export type CanvasCustomVideoGenerationState = {
    readonly runtime?: CustomVideoRuntimeSnapshot;
    readonly error?: string;
};

export function canvasCustomVideoRuntimeForModel(config: AiConfig, model: string, runtime: CustomVideoRuntimeSnapshot | undefined) {
    const customConfig = customVideoConfigForModel(config, model);
    return customConfig ? normalizeCustomVideoRuntimeStateForModelSwitch(customConfig, runtime?.values, runtime?.media) : undefined;
}

export function canvasCustomVideoGenerationState(config: AiConfig, model: string, runtime: CustomVideoRuntimeSnapshot | undefined): CanvasCustomVideoGenerationState {
    if (videoRouteForModel(config, model) !== "custom") return {};
    const customConfig = customVideoConfigForModel(config, model);
    if (!customConfig) return { error: "该模型的自定义视频配置无效，请联系管理员" };
    const normalized = canvasCustomVideoRuntimeForModel(config, model, runtime);
    return { runtime: normalized, error: customVideoRequiredMediaErrors(customConfig, normalized?.media)[0]?.message };
}

export function canvasVideoGenerationOptions(config: AiConfig, model: string, runtime: CustomVideoRuntimeSnapshot | undefined, signal: AbortSignal) {
    if (videoRouteForModel(config, model) !== "custom") return { signal };
    return { signal, customVideoRuntime: canvasCustomVideoGenerationState(config, model, runtime).runtime };
}
