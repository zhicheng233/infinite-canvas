import { describe, expect, test } from "bun:test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";

import { CustomVideoReferenceInputs, appendCustomVideoMedia, customVideoReferenceInputRoles, removeCustomVideoMedia } from "./custom-video-reference-inputs";
import type { CustomVideoConfig } from "@/lib/custom-video-config";
import { createEmptyCustomVideoMediaState } from "@/lib/custom-video-runtime";

const config: CustomVideoConfig = {
    seconds: { enabled: false, key: "seconds", mode: "range", min: 1, max: 1, step: 1, default: 1 },
    dimensions: { enabled: false, mode: "size", key: "size", options: [], default: "" },
    images: { enabled: true, required: false, key: "images", max_count: 2 },
    input_reference: { enabled: false, required: false, key: "input_reference", max_count: 1 },
    style_references: { enabled: true, required: true, key: "style_references", max_count: 5 },
    element_references: { enabled: true, required: false, key: "element_references", max_count: 6 },
    reference_images: { enabled: true, required: false, key: "reference_images", max_count: 5 },
    reference_mode: { enabled: true, key: "reference_mode", options: ["frame", "style"], default: "style" },
    input_video: { enabled: true, required: false, key: "input_video", max_count: 2 },
    audio: { enabled: false, key: "audio", mode: "fixed", value: false },
    n: { enabled: false, key: "n", value: 1 },
};

function mediaSources(name: string, count: number) {
    return Array.from({ length: count }, (_, index) => `https://example.com/${name}-${index + 1}`);
}

describe("custom video reference inputs", () => {
    test("accepts configured counts above former caps and ignores only excess sources", () => {
        const initial = createEmptyCustomVideoMediaState();
        const images = appendCustomVideoMedia(initial, "images", [...mediaSources("image", 2), "https://example.com/ignored-image"], config.images.max_count);
        const styled = appendCustomVideoMedia(images, "style_references", [...mediaSources("style", 5), "https://example.com/ignored-style"], config.style_references.max_count);
        const elements = appendCustomVideoMedia(styled, "element_references", [...mediaSources("element", 6), "https://example.com/ignored-element"], config.element_references.max_count);
        const afterDelete = removeCustomVideoMedia(elements, "style_references", 1);

        expect(afterDelete.images).toEqual(mediaSources("image", 2));
        expect(afterDelete.style_references).toEqual([mediaSources("style", 5)[0], ...mediaSources("style", 5).slice(2)]);
        expect(afterDelete.element_references).toEqual(mediaSources("element", 6));
        expect(afterDelete.reference_images).toEqual([]);
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
