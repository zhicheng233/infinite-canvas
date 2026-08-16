import { describe, expect, test } from "bun:test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";

import { CustomVideoReferenceInputs, appendCustomVideoMedia, customVideoReferenceInputRoles, removeCustomVideoMedia } from "./custom-video-reference-inputs";
import type { CustomVideoConfig } from "@/lib/custom-video-config";
import { createEmptyCustomVideoMediaState } from "@/lib/custom-video-runtime";

const config: CustomVideoConfig = {
    seconds: { enabled: false, key: "seconds", mode: "range", min: 1, max: 1, step: 1, default: 1 },
    dimensions: { enabled: false, mode: "size", key: "size", options: [], default: "" },
    images: { enabled: true, required: false, key: "images", max_count: 1 },
    input_reference: { enabled: false, required: false, key: "input_reference", max_count: 1 },
    style_references: { enabled: true, required: true, key: "style_references", max_count: 4 },
    element_references: { enabled: true, required: false, key: "element_references", max_count: 3 },
    reference_images: { enabled: true, required: false, key: "reference_images", max_count: 2 },
    reference_mode: { enabled: true, key: "reference_mode", options: ["frame", "style"], default: "style" },
    input_video: { enabled: true, required: false, key: "input_video", max_count: 1 },
    audio: { enabled: false, key: "audio", mode: "fixed", value: false },
    n: { enabled: false, key: "n", value: 1 },
};

describe("custom video reference inputs", () => {
    test("keeps role media independent while applying each role limit", () => {
        const initial = createEmptyCustomVideoMediaState();
        const images = appendCustomVideoMedia(initial, "images", ["https://example.com/first.png", "https://example.com/ignored.png"], 1);
        const styled = appendCustomVideoMedia(
            images,
            "style_references",
            ["https://example.com/style-1.png", "https://example.com/style-2.png", "https://example.com/style-3.png", "https://example.com/style-4.png", "https://example.com/ignored-style.png"],
            4,
        );
        const afterDelete = removeCustomVideoMedia(styled, "style_references", 1);

        expect(afterDelete.images).toEqual(["https://example.com/first.png"]);
        expect(afterDelete.style_references).toEqual(["https://example.com/style-1.png", "https://example.com/style-3.png", "https://example.com/style-4.png"]);
        expect(afterDelete.element_references).toEqual([]);
    });

    test("renders only enabled roles and shows reference mode only with both dependencies", () => {
        expect(customVideoReferenceInputRoles(config)).toEqual(["images", "style_references", "element_references", "reference_images", "input_video"]);

        const markup = renderToStaticMarkup(createElement(CustomVideoReferenceInputs, { config, media: createEmptyCustomVideoMediaState(), referenceMode: "style", onChange: () => undefined, onUpload: async () => [] }));
        expect(markup).toContain("普通参考图");
        expect(markup).toContain("风格参考图");
        expect(markup).toContain("元素参考图");
        expect(markup).toContain("兼容参考图");
        expect(markup).toContain("源视频");
        expect(markup).toContain("风格参考图（必填）");
        expect(markup).toContain("普通参考图（可选）");
        expect(markup).not.toContain("首帧参考图");
        expect(markup).toContain("参考图模式");

        const withoutMode = renderToStaticMarkup(
            createElement(CustomVideoReferenceInputs, {
                config: { ...config, reference_mode: { enabled: false, key: "reference_mode", options: [], default: "" } },
                media: createEmptyCustomVideoMediaState(),
                onChange: () => undefined,
                onUpload: async () => [],
            }),
        );
        expect(withoutMode).not.toContain("参考图模式");
    });
});
