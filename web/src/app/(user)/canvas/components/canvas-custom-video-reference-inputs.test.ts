import { describe, expect, test } from "bun:test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";

import { CanvasCustomVideoReferenceInputs, appendCanvasCustomVideoMedia, canvasCustomVideoReferenceRoles, removeCanvasCustomVideoMedia } from "./canvas-custom-video-reference-inputs";
import { canvasThemes } from "@/lib/canvas-theme";
import type { CustomVideoConfig } from "@/lib/custom-video-config";
import { createEmptyCustomVideoMediaState, normalizeCustomVideoRuntimeState } from "@/lib/custom-video-runtime";

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
    return Array.from({ length: count }, (_, index) => `https://media.example.com/${name}-${index + 1}`);
}

describe("canvas custom video reference inputs", () => {
    test("keeps roles independent while ignoring only sources beyond configured limits", () => {
        const initial = normalizeCustomVideoRuntimeState(config, {}, createEmptyCustomVideoMediaState());
        const withImages = appendCanvasCustomVideoMedia(initial, "images", [...mediaSources("image", 2), "https://media.example.com/ignored-image"], config);
        const withStyles = appendCanvasCustomVideoMedia(withImages, "style_references", [...mediaSources("style", 5), "https://media.example.com/ignored-style"], config);
        const withElements = appendCanvasCustomVideoMedia(withStyles, "element_references", [...mediaSources("element", 6), "https://media.example.com/ignored-element"], config);
        const withReferences = appendCanvasCustomVideoMedia(withElements, "reference_images", [...mediaSources("reference", 5), "https://media.example.com/ignored-reference"], config);
        const next = removeCanvasCustomVideoMedia(withReferences, "style_references", 0, config);

        expect(next.media.images).toEqual(mediaSources("image", 2));
        expect(next.media.style_references).toEqual(mediaSources("style", 5).slice(1));
        expect(next.media.element_references).toEqual(mediaSources("element", 6));
        expect(next.media.reference_images).toEqual(mediaSources("reference", 5));
        expect(next.media.input_video).toEqual([]);
    });

    test("uses a compact collapsed summary until the role inputs are explicitly opened", () => {
        expect(canvasCustomVideoReferenceRoles(config)).toEqual(["images", "style_references", "element_references", "reference_images", "input_video"]);

        const markup = renderToStaticMarkup(
            createElement(CanvasCustomVideoReferenceInputs, {
                config,
                runtime: normalizeCustomVideoRuntimeState(config, {}, { style_references: ["https://media.example.com/style.png"] }),
                theme: canvasThemes.light,
                onChange: () => undefined,
            }),
        );

        expect(markup).toContain("分角色素材");
        expect(markup).toContain("已选 1 项");
        expect(markup).toContain("必填：风格参考图");
        expect(markup).not.toContain("普通参考图");
    });
});
