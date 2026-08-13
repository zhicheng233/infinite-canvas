import { customVideoConfigForModel, videoRouteForModel, type AiConfig } from "@/stores/use-config-store";
import { normalizeCustomVideoRuntimeState, type CustomVideoRuntimeSnapshot } from "@/lib/custom-video-runtime";

export function canvasCustomVideoRuntimeForModel(config: AiConfig, model: string, runtime: CustomVideoRuntimeSnapshot | undefined) {
    const customConfig = customVideoConfigForModel(config, model);
    return customConfig ? normalizeCustomVideoRuntimeState(customConfig, runtime?.values, runtime?.media) : undefined;
}

export function canvasVideoGenerationOptions(config: AiConfig, model: string, runtime: CustomVideoRuntimeSnapshot | undefined, signal: AbortSignal) {
    if (videoRouteForModel(config, model) !== "custom") return { signal };
    return { signal, customVideoRuntime: canvasCustomVideoRuntimeForModel(config, model, runtime) };
}
