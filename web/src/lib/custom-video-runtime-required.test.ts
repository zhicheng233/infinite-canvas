import { describe, expect, test } from "bun:test";

import { customVideoMediaFeatureNames, type CustomVideoConfig, type CustomVideoMediaFeature } from "./custom-video-config";
import { createEmptyCustomVideoMediaState, customVideoRequiredMediaErrors, normalizeCustomVideoRuntimeState, normalizeCustomVideoRuntimeStateForModelSwitch } from "./custom-video-runtime";

const optionalConfig: CustomVideoConfig = {
    seconds: { enabled: true, key: "seconds", mode: "options", options: [5, 8], default: 8 },
    dimensions: { enabled: true, mode: "size", key: "size", options: ["1280x720"], default: "1280x720" },
    images: { enabled: true, required: false, key: "images", max_count: 1 },
    input_reference: { enabled: true, required: false, key: "input_reference", max_count: 1 },
    style_references: { enabled: true, required: false, key: "style_references", max_count: 4 },
    element_references: { enabled: true, required: false, key: "element_references", max_count: 3 },
    reference_images: { enabled: true, required: false, key: "reference_images", max_count: 4 },
    reference_mode: { enabled: true, key: "reference_mode", options: ["frame", "style"], default: "style" },
    input_video: { enabled: true, required: false, key: "input_video", max_count: 1 },
    audio: { enabled: true, key: "audio", mode: "user", value: true },
    n: { enabled: true, key: "n", value: 1 },
};

const acceptedSources = ["https://example.com/reference.png", "data:image/png;base64,AAAA", "blob:https://example.com/reference", "/backend-api/files/reference.png"] as const;
const requiredErrors = {
    images: "缺少必填素材：普通参考图",
    input_reference: "缺少必填素材：首帧参考图",
    style_references: "缺少必填素材：风格参考图",
    element_references: "缺少必填素材：元素参考图",
    reference_images: "缺少必填素材：兼容参考图",
    input_video: "缺少必填素材：源视频",
} as const satisfies Readonly<Record<CustomVideoMediaFeature, string>>;

describe("custom video prompt-only runtime", () => {
    test("uses scalar defaults and empty role media when the snapshot is absent", () => {
        const normalized = normalizeCustomVideoRuntimeState(optionalConfig, undefined, undefined);

        expect(normalized).toEqual({
            values: { seconds: 8, dimension: "1280x720", reference_mode: "style", audio: true },
            media: createEmptyCustomVideoMediaState(),
        });
        expect(customVideoRequiredMediaErrors(optionalConfig, normalized.media)).toEqual([]);
    });

    test("defaults absent fields in a partial snapshot without replacing explicit invalid scalars", () => {
        const partial = normalizeCustomVideoRuntimeState(optionalConfig, { seconds: 5 }, undefined);
        const invalid = normalizeCustomVideoRuntimeState(optionalConfig, { seconds: 7, dimension: "invalid", reference_mode: "invalid", audio: "invalid" }, undefined);

        expect(partial.values).toEqual({ seconds: 5, dimension: "1280x720", reference_mode: "style", audio: true });
        expect(invalid.values as unknown).toEqual({ seconds: 7, dimension: "invalid", reference_mode: "invalid", audio: "invalid" });
    });
});

describe("custom video required media roles", () => {
    for (const role of customVideoMediaFeatureNames) {
        test(`${role} requires normalized valid media`, () => {
            const config = configWithRequiredRole(role);

            expect(customVideoRequiredMediaErrors(config, { [role]: ["", "   ", "not-a-url", "ftp://example.com/reference"] })).toEqual([{ role, message: requiredErrors[role] }]);
            for (const source of acceptedSources) expect(customVideoRequiredMediaErrors(config, { [role]: [source] })).toEqual([]);
        });
    }

    test("does not use media from a disabled role to satisfy another required role", () => {
        const config: CustomVideoConfig = {
            ...optionalConfig,
            images: { ...optionalConfig.images, enabled: false, required: true },
            style_references: { ...optionalConfig.style_references, required: true },
        };

        expect(customVideoRequiredMediaErrors(config, { images: ["https://example.com/reference.png"] })).toEqual([{ role: "style_references", message: "缺少必填素材：风格参考图" }]);
    });

    test("filters invalid sources before applying the role cap", () => {
        const config = configWithRequiredRole("input_reference");

        expect(customVideoRequiredMediaErrors(config, { input_reference: ["not-a-url", "https://example.com/reference.png"] })).toEqual([]);
    });

    test("reports a destination role gap without moving media between roles on a model switch", () => {
        const config = configWithRequiredRole("input_reference");

        const normalized = normalizeCustomVideoRuntimeStateForModelSwitch(config, undefined, { images: ["https://example.com/reference.png"] });

        expect(normalized.media.images).toEqual(["https://example.com/reference.png"]);
        expect(normalized.media.input_reference).toEqual([]);
        expect(customVideoRequiredMediaErrors(config, normalized.media).map((error) => error.role)).toEqual(["input_reference"]);
    });
});

function configWithRequiredRole(role: CustomVideoMediaFeature): CustomVideoConfig {
    return { ...optionalConfig, [role]: { ...optionalConfig[role], required: true } };
}
