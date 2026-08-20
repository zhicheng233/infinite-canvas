export const customVideoFeatureNames = ["seconds", "dimensions", "images", "input_reference", "style_references", "element_references", "reference_images", "reference_mode", "input_video", "audio", "n"] as const;
export const customVideoMediaFeatureNames = ["images", "input_reference", "style_references", "element_references", "reference_images", "input_video"] as const;
export const customVideoReferenceModes = ["frame", "style", "element"] as const;
export const customVideoDefaultKeys = {
    seconds: "seconds",
    dimensions: "size",
    images: "images",
    input_reference: "input_reference",
    style_references: "style_references",
    element_references: "element_references",
    reference_images: "reference_images",
    reference_mode: "reference_mode",
    input_video: "input_video",
    audio: "audio",
    n: "n",
} as const;
export const customVideoDimensionDefaultKeys = { size: "size", aspect_ratio: "aspect_ratio" } as const;

export type CustomVideoFeature = (typeof customVideoFeatureNames)[number];
export type CustomVideoMediaFeature = (typeof customVideoMediaFeatureNames)[number];
export type CustomVideoReferenceMode = (typeof customVideoReferenceModes)[number];
export type CustomVideoSecondsConfig =
    | { readonly enabled: boolean; readonly key: string; readonly mode: "range"; readonly min: number; readonly max: number; readonly step: number; readonly default: number }
    | { readonly enabled: boolean; readonly key: string; readonly mode: "options"; readonly options: readonly number[]; readonly default: number };
export type CustomVideoDimensionsConfig = { readonly enabled: boolean; readonly mode: "size" | "aspect_ratio"; readonly key: string; readonly options: readonly string[]; readonly default: string };
export type CustomVideoMediaConfig = { readonly enabled: boolean; readonly required: boolean; readonly key: string; readonly max_count: number };
export type CustomVideoReferenceModeConfig = { readonly enabled: boolean; readonly key: string; readonly options: readonly CustomVideoReferenceMode[]; readonly default: CustomVideoReferenceMode | "" };
export type CustomVideoAudioConfig = { readonly enabled: boolean; readonly key: string; readonly mode: "fixed" | "user"; readonly value: boolean };
export type CustomVideoNConfig = { readonly enabled: boolean; readonly key: string; readonly value: number };
export type CustomVideoConfig = {
    readonly seconds: CustomVideoSecondsConfig;
    readonly dimensions: CustomVideoDimensionsConfig;
    readonly images: CustomVideoMediaConfig;
    readonly input_reference: CustomVideoMediaConfig;
    readonly style_references: CustomVideoMediaConfig;
    readonly element_references: CustomVideoMediaConfig;
    readonly reference_images: CustomVideoMediaConfig;
    readonly reference_mode: CustomVideoReferenceModeConfig;
    readonly input_video: CustomVideoMediaConfig;
    readonly audio: CustomVideoAudioConfig;
    readonly n: CustomVideoNConfig;
};
export type CustomVideoRuntimeValues = { readonly seconds?: number; readonly dimension?: string; readonly reference_mode?: CustomVideoReferenceMode; readonly audio?: boolean };
export type CustomVideoConfigSummary = {
    readonly enabled: readonly CustomVideoFeature[];
    readonly aliases: Readonly<Partial<Record<CustomVideoFeature, string>>>;
    readonly seconds: CustomVideoSecondsConfig | null;
    readonly dimensions: CustomVideoDimensionsConfig | null;
    readonly media_limits: Readonly<Partial<Record<CustomVideoMediaFeature, number>>>;
    readonly media_required: Readonly<Partial<Record<CustomVideoMediaFeature, boolean>>>;
    readonly reference_mode: CustomVideoReferenceModeConfig | null;
    readonly audio: CustomVideoAudioConfig | null;
    readonly n: number | null;
};
export type CustomVideoConfigResult = { readonly ok: true; readonly config: CustomVideoConfig } | { readonly ok: false; readonly errors: readonly string[] };

export function normalizeAndValidateCustomVideoConfig(value: unknown): CustomVideoConfigResult {
    if (!isRecord(value)) return { ok: false, errors: ["video_custom_config 格式错误"] };
    const errors: string[] = [];
    const secondsValue = recordValue(value.seconds, "seconds", errors);
    const dimensionsValue = recordValue(value.dimensions, "dimensions", errors);
    const secondsEnabled = booleanValue(secondsValue.enabled, "seconds.enabled", errors);
    const dimensionsEnabled = booleanValue(dimensionsValue.enabled, "dimensions.enabled", errors);
    const secondsModeValue = textValue(secondsValue.mode, "seconds.mode", errors);
    const dimensionsModeValue = textValue(dimensionsValue.mode, "dimensions.mode", errors);
    const secondsMode = secondsModeValue === "options" ? "options" : "range";
    const dimensionsMode = dimensionsModeValue === "aspect_ratio" ? "aspect_ratio" : "size";
    if (secondsEnabled && secondsModeValue !== "range" && secondsModeValue !== "options") errors.push("seconds.mode 必须是 range 或 options");
    if (dimensionsEnabled && dimensionsModeValue !== "size" && dimensionsModeValue !== "aspect_ratio") errors.push("dimensions.mode 必须是 size 或 aspect_ratio");
    const secondsDefault = integerValue(secondsValue.default, "seconds.default", errors);
    const seconds: CustomVideoSecondsConfig =
        secondsMode === "options"
            ? { enabled: secondsEnabled, key: textValue(secondsValue.key, "seconds.key", errors).trim(), mode: "options", options: uniqueSortedIntegers(secondsValue.options, "seconds.options", errors), default: secondsDefault }
            : {
                  enabled: secondsEnabled,
                  key: textValue(secondsValue.key, "seconds.key", errors).trim(),
                  mode: "range",
                  min: optionalIntegerValue(secondsValue.min, "seconds.min", errors),
                  max: optionalIntegerValue(secondsValue.max, "seconds.max", errors),
                  step: optionalIntegerValue(secondsValue.step, "seconds.step", errors),
                  default: secondsDefault,
              };
    const dimensions: CustomVideoDimensionsConfig = {
        enabled: dimensionsEnabled,
        mode: dimensionsMode,
        key: textValue(dimensionsValue.key, "dimensions.key", errors).trim(),
        options: uniqueSortedStrings(dimensionsValue.options, "dimensions.options", errors),
        default: textValue(dimensionsValue.default, "dimensions.default", errors).trim(),
    };
    const media = {
        images: normalizeMediaConfig(value.images, "images", errors),
        input_reference: normalizeMediaConfig(value.input_reference, "input_reference", errors),
        style_references: normalizeMediaConfig(value.style_references, "style_references", errors),
        element_references: normalizeMediaConfig(value.element_references, "element_references", errors),
        reference_images: normalizeMediaConfig(value.reference_images, "reference_images", errors),
        input_video: normalizeMediaConfig(value.input_video, "input_video", errors),
    };
    const referenceModeValue = recordValue(value.reference_mode, "reference_mode", errors);
    const referenceModeOptions = uniqueReferenceModes(referenceModeValue.options, errors);
    const referenceModeDefaultValue = textValue(referenceModeValue.default, "reference_mode.default", errors).trim();
    const reference_mode: CustomVideoReferenceModeConfig = {
        enabled: booleanValue(referenceModeValue.enabled, "reference_mode.enabled", errors),
        key: textValue(referenceModeValue.key, "reference_mode.key", errors).trim(),
        options: referenceModeOptions,
        default: isCustomVideoReferenceMode(referenceModeDefaultValue) ? referenceModeDefaultValue : "",
    };
    const audioValue = recordValue(value.audio, "audio", errors);
    const audioEnabled = booleanValue(audioValue.enabled, "audio.enabled", errors);
    const audioModeValue = textValue(audioValue.mode, "audio.mode", errors);
    if (audioEnabled && audioModeValue !== "fixed" && audioModeValue !== "user") errors.push("audio.mode 必须是 fixed 或 user");
    const audio: CustomVideoAudioConfig = { enabled: audioEnabled, key: textValue(audioValue.key, "audio.key", errors).trim(), mode: audioModeValue === "user" ? "user" : "fixed", value: booleanValue(audioValue.value, "audio.value", errors) };
    const nValue = recordValue(value.n, "n", errors);
    const n: CustomVideoNConfig = { enabled: booleanValue(nValue.enabled, "n.enabled", errors), key: textValue(nValue.key, "n.key", errors).trim(), value: integerValue(nValue.value, "n.value", errors) };
    const config: CustomVideoConfig = { seconds, dimensions, ...media, reference_mode, audio, n };
    validateSemantics(config, errors);
    return errors.length ? { ok: false, errors } : { ok: true, config };
}

function validateSemantics(config: CustomVideoConfig, errors: string[]) {
    if (config.seconds.enabled) {
        if (config.seconds.mode === "range") {
            if (config.seconds.min < 1 || config.seconds.min > config.seconds.default || config.seconds.default > config.seconds.max || config.seconds.max > 3600) errors.push("seconds range 必须满足 1 <= min <= default <= max <= 3600");
            if (config.seconds.step < 1) errors.push("seconds.step 必须是正整数");
            else if ((config.seconds.default - config.seconds.min) % config.seconds.step !== 0) errors.push("seconds.default 必须在步长网格上");
        } else {
            if (config.seconds.options.length < 1 || config.seconds.options.length > 100) errors.push("seconds.options 必须包含 1-100 项");
            if (config.seconds.options.some((item) => item < 1 || item > 3600)) errors.push("seconds.options 每项必须在 1-3600 之间");
            if (!config.seconds.options.includes(config.seconds.default)) errors.push("seconds.default 必须在 options 中");
        }
    }
    if (config.dimensions.enabled) {
        if (config.dimensions.options.length < 1 || config.dimensions.options.length > 50) errors.push("dimensions.options 必须包含 1-50 项");
        if (!config.dimensions.options.includes(config.dimensions.default)) errors.push("dimensions.default 必须在 options 中");
    }
    for (const name of customVideoMediaFeatureNames) if (config[name].enabled && (!Number.isSafeInteger(config[name].max_count) || config[name].max_count < 1)) errors.push(`${name}.max_count 必须是正安全整数`);
    if (config.reference_mode.enabled) {
        if (!config.reference_images.enabled) errors.push("reference_mode 仅可在 reference_images 启用时启用");
        if (!config.reference_mode.options.length) errors.push("reference_mode.options 必须是 frame/style/element 的非空子集");
        if (!config.reference_mode.default || !config.reference_mode.options.includes(config.reference_mode.default)) errors.push("reference_mode.default 必须在 options 中");
    }
    if (config.n.enabled && (config.n.value < 1 || config.n.value > 16)) errors.push("n.value 必须在 1-16 之间");
    const seen = new Map<string, CustomVideoFeature>();
    for (const name of customVideoFeatureNames.filter((item) => config[item].enabled)) {
        const key = config[name].key;
        if (!key) errors.push(`${name}.key 不能为空`);
        else if (key === "model" || key === "prompt") errors.push(`${name}.key 不能是 ${key}`);
        else if (seen.has(key)) errors.push(`${name}.key 与 ${seen.get(key)}.key 重复`);
        else seen.set(key, name);
    }
}

function normalizeMediaConfig(value: unknown, field: CustomVideoMediaFeature, errors: string[]): CustomVideoMediaConfig {
    const item = recordValue(value, field, errors);
    const enabled = booleanValue(item.enabled, `${field}.enabled`, errors);
    const required = booleanValue(item.required, `${field}.required`, errors);
    return { enabled, required: enabled && required, key: textValue(item.key, `${field}.key`, errors).trim(), max_count: integerValue(item.max_count, `${field}.max_count`, errors) };
}

function recordValue(value: unknown, field: string, errors: string[]): Record<string, unknown> {
    if (value === undefined || value === null) return {};
    if (isRecord(value)) return value;
    errors.push(`${field} 格式错误`);
    return {};
}

function textValue(value: unknown, field: string, errors: string[]) {
    if (value === undefined || value === null) return "";
    if (typeof value === "string") return value;
    errors.push(`${field} 格式错误`);
    return "";
}

function booleanValue(value: unknown, field: string, errors: string[]) {
    if (value === undefined || value === null) return false;
    if (typeof value === "boolean") return value;
    errors.push(`${field} 格式错误`);
    return false;
}

function integerValue(value: unknown, field: string, errors: string[]) {
    if (value === undefined || value === null) return 0;
    if (typeof value === "number" && Number.isInteger(value)) return value;
    errors.push(`${field} 必须是整数`);
    return 0;
}

function optionalIntegerValue(value: unknown, field: string, errors: string[]) {
    return value === undefined || value === null ? 0 : integerValue(value, field, errors);
}

function uniqueSortedIntegers(value: unknown, field: string, errors: string[]) {
    if (value === undefined || value === null) return [];
    if (!Array.isArray(value) || value.some((item) => typeof item !== "number" || !Number.isInteger(item))) {
        errors.push(`${field} 必须是整数数组`);
        return [];
    }
    return Array.from(new Set(value)).sort((left, right) => left - right);
}

function uniqueSortedStrings(value: unknown, field: string, errors: string[]) {
    if (value === undefined || value === null) return [];
    if (!Array.isArray(value) || value.some((item) => typeof item !== "string" || !item.trim())) {
        errors.push(`${field} 不能包含空字符串`);
        return [];
    }
    return Array.from(new Set(value.map((item) => item.trim()))).sort();
}

function uniqueReferenceModes(value: unknown, errors: string[]): CustomVideoReferenceMode[] {
    const values = uniqueSortedStrings(value, "reference_mode.options", errors);
    if (values.some((item) => !isCustomVideoReferenceMode(item))) errors.push("reference_mode.options 必须是 frame/style/element 的非空子集");
    return customVideoReferenceModes.filter((item) => values.includes(item));
}

function isRecord(value: unknown): value is Record<string, unknown> {
    return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

function isCustomVideoReferenceMode(value: string): value is CustomVideoReferenceMode {
    return customVideoReferenceModes.some((item) => item === value);
}
