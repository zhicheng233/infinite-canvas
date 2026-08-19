import type { CustomVideoConfig } from "@/lib/custom-video-config";
import { customVideoRequiredMediaErrors, normalizeCustomVideoRuntimeState, normalizeCustomVideoRuntimeStateForModelSwitch, type CustomVideoRuntimeSnapshot } from "@/lib/custom-video-runtime";

export type VideoCustomVideoGenerationState = {
    readonly runtime?: CustomVideoRuntimeSnapshot;
    readonly error?: string;
};

export function videoCustomVideoRuntimeForModelSwitch(config: CustomVideoConfig, runtime: CustomVideoRuntimeSnapshot | undefined): CustomVideoRuntimeSnapshot {
    return normalizeCustomVideoRuntimeStateForModelSwitch(config, runtime?.values, runtime?.media);
}

export function videoCustomVideoGenerationState(config: CustomVideoConfig | null, runtime: CustomVideoRuntimeSnapshot | undefined): VideoCustomVideoGenerationState {
    if (!config) return { error: "该模型的自定义视频配置无效，请联系管理员" };
    const normalized = normalizeCustomVideoRuntimeState(config, runtime?.values, runtime?.media);
    return { runtime: normalized, error: customVideoRequiredMediaErrors(config, normalized.media)[0]?.message };
}
