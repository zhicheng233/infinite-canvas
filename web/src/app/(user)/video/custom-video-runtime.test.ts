import { describe, expect, test } from "bun:test";

import type { CustomVideoConfig } from "@/lib/custom-video-config";
import { videoCustomVideoGenerationState, videoCustomVideoRuntimeForModelSwitch } from "./custom-video-runtime";

const config: CustomVideoConfig = {
    seconds: { enabled: true, key: "seconds", mode: "options", options: [5, 8], default: 8 },
    dimensions: { enabled: true, mode: "size", key: "size", options: ["1280x720"], default: "1280x720" },
    images: { enabled: true, required: false, key: "images", max_count: 1 },
    input_reference: { enabled: false, required: false, key: "input_reference", max_count: 1 },
    style_references: { enabled: true, required: false, key: "style_references", max_count: 4 },
    element_references: { enabled: false, required: false, key: "element_references", max_count: 3 },
    reference_images: { enabled: false, required: false, key: "reference_images", max_count: 4 },
    reference_mode: { enabled: false, key: "reference_mode", options: [], default: "" },
    input_video: { enabled: false, required: false, key: "input_video", max_count: 1 },
    audio: { enabled: false, key: "audio", mode: "fixed", value: false },
    n: { enabled: true, key: "n", value: 1 },
};

describe("standalone custom video runtime", () => {
    test("allows prompt-only generation with optional media", () => {
        expect(videoCustomVideoGenerationState(config, undefined)).toEqual({
            runtime: {
                values: { seconds: 8, dimension: "1280x720" },
                media: { images: [], input_reference: [], style_references: [], element_references: [], reference_images: [], input_video: [] },
            },
            error: undefined,
        });
    });

    test("blocks the missing required role without accepting another role", () => {
        const requiredConfig = { ...config, style_references: { ...config.style_references, required: true } } satisfies CustomVideoConfig;

        expect(
            videoCustomVideoGenerationState(requiredConfig, { values: {}, media: { images: ["https://example.com/reference.png"], input_reference: [], style_references: [], element_references: [], reference_images: [], input_video: [] } }),
        ).toMatchObject({
            error: "缺少必填素材：风格参考图",
        });
    });

    test("resets invalid destination scalars and preserves only valid same-role media on model switch", () => {
        const targetConfig = {
            ...config,
            style_references: { ...config.style_references, max_count: 1 },
        } satisfies CustomVideoConfig;

        expect(
            videoCustomVideoRuntimeForModelSwitch(targetConfig, {
                values: { seconds: 6, dimension: "720x1280" },
                media: {
                    images: ["https://example.com/image.png"],
                    input_reference: ["https://example.com/disabled.png"],
                    style_references: ["not-a-url", "https://example.com/style.png"],
                    element_references: [],
                    reference_images: [],
                    input_video: [],
                },
            }),
        ).toEqual({
            values: { seconds: 8, dimension: "1280x720" },
            media: {
                images: ["https://example.com/image.png"],
                input_reference: [],
                style_references: ["https://example.com/style.png"],
                element_references: [],
                reference_images: [],
                input_video: [],
            },
        });
    });
});
