import {
    customVideoDefaultKeys,
    customVideoFeatureNames,
    customVideoMediaFeatureNames,
    normalizeAndValidateCustomVideoConfig,
    type CustomVideoConfig,
    type CustomVideoConfigSummary,
    type CustomVideoFeature,
    type CustomVideoMediaFeature,
} from "./custom-video-config-normalizer";

export {
    customVideoDefaultKeys,
    customVideoDimensionDefaultKeys,
    customVideoFeatureNames,
    customVideoMediaFeatureNames,
    customVideoMediaHardLimits,
    customVideoReferenceModes,
    normalizeAndValidateCustomVideoConfig,
} from "./custom-video-config-normalizer";
export type {
    CustomVideoAudioConfig,
    CustomVideoConfig,
    CustomVideoConfigResult,
    CustomVideoConfigSummary,
    CustomVideoDimensionsConfig,
    CustomVideoFeature,
    CustomVideoMediaConfig,
    CustomVideoMediaFeature,
    CustomVideoNConfig,
    CustomVideoReferenceMode,
    CustomVideoReferenceModeConfig,
    CustomVideoRuntimeValues,
    CustomVideoSecondsConfig,
} from "./custom-video-config-normalizer";

export function createDefaultCustomVideoConfig(): CustomVideoConfig {
    return {
        seconds: { enabled: false, key: customVideoDefaultKeys.seconds, mode: "range", min: 1, max: 1, step: 1, default: 1 },
        dimensions: { enabled: false, mode: "size", key: customVideoDefaultKeys.dimensions, options: [], default: "" },
        images: { enabled: false, key: customVideoDefaultKeys.images, max_count: 1 },
        input_reference: { enabled: false, key: customVideoDefaultKeys.input_reference, max_count: 1 },
        style_references: { enabled: false, key: customVideoDefaultKeys.style_references, max_count: 1 },
        element_references: { enabled: false, key: customVideoDefaultKeys.element_references, max_count: 1 },
        reference_images: { enabled: false, key: customVideoDefaultKeys.reference_images, max_count: 1 },
        reference_mode: { enabled: false, key: customVideoDefaultKeys.reference_mode, options: [], default: "" },
        input_video: { enabled: false, key: customVideoDefaultKeys.input_video, max_count: 1 },
        audio: { enabled: false, key: customVideoDefaultKeys.audio, mode: "fixed", value: false },
        n: { enabled: false, key: customVideoDefaultKeys.n, value: 1 },
    };
}

export function normalizeCustomVideoConfig(value: unknown): CustomVideoConfig | null {
    const result = normalizeAndValidateCustomVideoConfig(value);
    return result.ok ? result.config : null;
}

export function validateCustomVideoConfig(value: unknown): readonly string[] {
    const result = normalizeAndValidateCustomVideoConfig(value);
    return result.ok ? [] : result.errors;
}

export function enabledCustomVideoFeatures(config: CustomVideoConfig | null | undefined): CustomVideoFeature[] {
    return config ? customVideoFeatureNames.filter((name) => config[name].enabled) : [];
}

export function enabledCustomVideoMediaFeatures(config: CustomVideoConfig | null | undefined): CustomVideoMediaFeature[] {
    return config ? customVideoMediaFeatureNames.filter((name) => config[name].enabled) : [];
}

export function summarizeCustomVideoConfig(config: CustomVideoConfig): CustomVideoConfigSummary {
    const enabled = enabledCustomVideoFeatures(config);
    return {
        enabled,
        aliases: Object.fromEntries(enabled.map((name) => [name, config[name].key])),
        seconds: config.seconds.enabled ? config.seconds : null,
        dimensions: config.dimensions.enabled ? config.dimensions : null,
        media_limits: Object.fromEntries(enabledCustomVideoMediaFeatures(config).map((name) => [name, config[name].max_count])),
        reference_mode: config.reference_mode.enabled ? config.reference_mode : null,
        audio: config.audio.enabled ? config.audio : null,
        n: config.n.enabled ? config.n.value : null,
    };
}
