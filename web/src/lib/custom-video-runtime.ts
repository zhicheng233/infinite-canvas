import { customVideoReferenceModes } from "./custom-video-config";
import type { CustomVideoConfig, CustomVideoMediaFeature, CustomVideoReferenceMode, CustomVideoRuntimeValues } from "./custom-video-config";

export type CustomVideoMediaState = Readonly<Record<CustomVideoMediaFeature, readonly string[]>>;

export type CustomVideoRuntimeSnapshot = {
    readonly values: CustomVideoRuntimeValues;
    readonly media: CustomVideoMediaState;
};

export type CustomVideoRuntimeContainer = {
    readonly customVideoRuntime?: CustomVideoRuntimeSnapshot;
};

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
    const valueRecord = isRecord(values) ? values : {};
    const normalizedSeconds = normalizeSeconds(config, valueRecord.seconds);
    const normalizedDimension = normalizeDimension(config, valueRecord.dimension);
    const normalizedReferenceMode = normalizeReferenceMode(config, valueRecord.reference_mode);
    const normalizedAudio = normalizeAudio(config, valueRecord.audio);

    return {
        values: {
            ...(normalizedSeconds === undefined ? {} : { seconds: normalizedSeconds }),
            ...(normalizedDimension === undefined ? {} : { dimension: normalizedDimension }),
            ...(normalizedReferenceMode === undefined ? {} : { reference_mode: normalizedReferenceMode }),
            ...(normalizedAudio === undefined ? {} : { audio: normalizedAudio }),
        },
        media: normalizeMedia(config, media),
    };
}

export function normalizeCustomVideoRuntimeContainer<T extends CustomVideoRuntimeContainer>(config: CustomVideoConfig | null | undefined, container: T): T {
    if (!config) return container;
    return {
        ...container,
        customVideoRuntime: normalizeCustomVideoRuntimeState(config, container.customVideoRuntime?.values, container.customVideoRuntime?.media),
    };
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
