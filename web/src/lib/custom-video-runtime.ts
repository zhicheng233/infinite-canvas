import { customVideoMediaFeatureNames, customVideoReferenceModes } from "./custom-video-config";
import type { CustomVideoConfig, CustomVideoMediaFeature, CustomVideoReferenceMode, CustomVideoRuntimeValues } from "./custom-video-config";

export type CustomVideoMediaState = Readonly<Record<CustomVideoMediaFeature, readonly string[]>>;

export type CustomVideoRuntimeSnapshot = {
    readonly values: CustomVideoRuntimeValues;
    readonly media: CustomVideoMediaState;
};

export type CustomVideoRuntimeInput = {
    readonly values?: CustomVideoRuntimeValues;
    readonly media?: Readonly<Partial<Record<CustomVideoMediaFeature, unknown>>>;
};

export type CustomVideoRuntimeContainer = {
    readonly customVideoRuntime?: CustomVideoRuntimeSnapshot;
};

export type CustomVideoRequiredMediaError = {
    readonly role: CustomVideoMediaFeature;
    readonly message: string;
};

const customVideoRequiredMediaLabels = {
    images: "普通参考图",
    input_reference: "首帧参考图",
    style_references: "风格参考图",
    element_references: "元素参考图",
    reference_images: "兼容参考图",
    input_video: "源视频",
} as const satisfies Readonly<Record<CustomVideoMediaFeature, string>>;

export function createEmptyCustomVideoMediaState(): CustomVideoMediaState {
    return {
        images: [],
        input_reference: [],
        style_references: [],
        element_references: [],
        reference_images: [],
        input_video: [],
    };
}

export function normalizeCustomVideoRuntimeState(config: CustomVideoConfig, values: unknown = undefined, media: unknown = undefined): CustomVideoRuntimeSnapshot {
    return {
        values: normalizeRuntimeValues(config, isRecord(values) ? values : {}),
        media: normalizeMedia(config, media),
    };
}

export function normalizeCustomVideoRuntimeStateForModelSwitch(config: CustomVideoConfig, values: unknown = undefined, media: unknown = undefined): CustomVideoRuntimeSnapshot {
    return {
        values: normalizeRuntimeValuesForModelSwitch(config, isRecord(values) ? values : {}),
        media: normalizeMedia(config, media),
    };
}

export function customVideoRequiredMediaErrors(config: CustomVideoConfig, media: unknown = undefined): readonly CustomVideoRequiredMediaError[] {
    const normalized = normalizeMedia(config, media);
    return customVideoMediaFeatureNames.flatMap((role) => (config[role].enabled && config[role].required && normalized[role].length === 0 ? [{ role, message: `缺少必填素材：${customVideoRequiredMediaLabels[role]}` }] : []));
}

export function normalizeCustomVideoRuntimeContainer<T extends CustomVideoRuntimeContainer>(config: CustomVideoConfig | null | undefined, container: T): T {
    if (!config) return container;
    return {
        ...container,
        customVideoRuntime: normalizeCustomVideoRuntimeStateForModelSwitch(config, container.customVideoRuntime?.values, container.customVideoRuntime?.media),
    };
}

function normalizeRuntimeValues(config: CustomVideoConfig, values: Readonly<Record<string, unknown>>): CustomVideoRuntimeValues {
    const normalized: CustomVideoRuntimeValues = {};
    if (config.seconds.enabled) defineRuntimeValue(normalized, "seconds", values.seconds === undefined ? config.seconds.default : values.seconds);
    if (config.dimensions.enabled) defineRuntimeValue(normalized, "dimension", values.dimension === undefined ? config.dimensions.default : values.dimension);
    if (config.reference_images.enabled && config.reference_mode.enabled) defineRuntimeValue(normalized, "reference_mode", values.reference_mode === undefined ? config.reference_mode.default || undefined : values.reference_mode);
    if (config.audio.enabled && config.audio.mode === "user") defineRuntimeValue(normalized, "audio", values.audio === undefined ? config.audio.value : values.audio);
    return normalized;
}

function normalizeRuntimeValuesForModelSwitch(config: CustomVideoConfig, values: Readonly<Record<string, unknown>>): CustomVideoRuntimeValues {
    const seconds = normalizeSeconds(config, values.seconds);
    const dimension = normalizeDimension(config, values.dimension);
    const referenceMode = normalizeReferenceMode(config, values.reference_mode);
    const audio = normalizeAudio(config, values.audio);
    return {
        ...(seconds === undefined ? {} : { seconds }),
        ...(dimension === undefined ? {} : { dimension }),
        ...(referenceMode === undefined ? {} : { reference_mode: referenceMode }),
        ...(audio === undefined ? {} : { audio }),
    };
}

function defineRuntimeValue(values: CustomVideoRuntimeValues, key: keyof CustomVideoRuntimeValues, value: unknown) {
    if (value !== undefined) Object.defineProperty(values, key, { value, enumerable: true });
}

function normalizeSeconds(config: CustomVideoConfig, value: unknown) {
    if (!config.seconds.enabled) return undefined;
    if (typeof value !== "number" || !Number.isInteger(value)) return config.seconds.default;
    if (config.seconds.mode === "options") return config.seconds.options.includes(value) ? value : config.seconds.default;
    return value >= config.seconds.min && value <= config.seconds.max && (value - config.seconds.min) % config.seconds.step === 0 ? value : config.seconds.default;
}

function normalizeDimension(config: CustomVideoConfig, value: unknown) {
    if (!config.dimensions.enabled) return undefined;
    return typeof value === "string" && config.dimensions.options.includes(value) ? value : config.dimensions.default;
}

function normalizeReferenceMode(config: CustomVideoConfig, value: unknown): CustomVideoReferenceMode | undefined {
    if (!config.reference_images.enabled || !config.reference_mode.enabled) return undefined;
    if (isCustomVideoReferenceMode(value) && config.reference_mode.options.includes(value)) return value;
    return isCustomVideoReferenceMode(config.reference_mode.default) ? config.reference_mode.default : undefined;
}

function normalizeAudio(config: CustomVideoConfig, value: unknown) {
    if (!config.audio.enabled || config.audio.mode !== "user") return undefined;
    return typeof value === "boolean" ? value : config.audio.value;
}

function normalizeMedia(config: CustomVideoConfig, value: unknown): CustomVideoMediaState {
    const record = isRecord(value) ? value : {};
    return {
        images: normalizeMediaRole(config, "images", record.images),
        input_reference: normalizeMediaRole(config, "input_reference", record.input_reference),
        style_references: normalizeMediaRole(config, "style_references", record.style_references),
        element_references: normalizeMediaRole(config, "element_references", record.element_references),
        reference_images: normalizeMediaRole(config, "reference_images", record.reference_images),
        input_video: normalizeMediaRole(config, "input_video", record.input_video),
    };
}

function normalizeMediaRole(config: CustomVideoConfig, role: CustomVideoMediaFeature, value: unknown) {
    if (!config[role].enabled || !Array.isArray(value)) return [];
    return value
        .filter(isMediaSource)
        .map((item) => item.trim())
        .slice(0, config[role].max_count);
}

function isMediaSource(value: unknown): value is string {
    if (typeof value !== "string") return false;
    const source = value.trim();
    if (!source) return false;
    if (source.startsWith("data:") || source.startsWith("blob:") || source.startsWith("/")) return true;
    try {
        const protocol = new URL(source).protocol;
        return protocol === "http:" || protocol === "https:";
    } catch {
        return false;
    }
}

function isCustomVideoReferenceMode(value: unknown): value is CustomVideoReferenceMode {
    return typeof value === "string" && customVideoReferenceModes.some((mode) => mode === value);
}

function isRecord(value: unknown): value is Readonly<Record<string, unknown>> {
    return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}
