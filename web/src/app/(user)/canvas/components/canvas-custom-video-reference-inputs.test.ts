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
    images: { enabled: true, key: "images", max_count: 1 },
    input_reference: { enabled: false, key: "input_reference", max_count: 1 },
    style_references: { enabled: true, key: "style_references", max_count: 2 },
    element_references: { enabled: false, key: "element_references", max_count: 3 },
    reference_images: { enabled: true, key: "reference_images", max_count: 1 },
    reference_mode: { enabled: true, key: "reference_mode", options: ["frame", "style"], default: "style" },
    input_video: { enabled: true, key: "input_video", max_count: 1 },
    audio: { enabled: false, key: "audio", mode: "fixed", value: false },
    n: { enabled: false, key: "n", value: 1 },
};

describe("canvas custom video reference inputs", () => {
    test("keeps role media independent and applies the active model limit", () => {
        const initial = normalizeCustomVideoRuntimeState(config, {}, createEmptyCustomVideoMediaState());
        const withImage = appendCanvasCustomVideoMedia(initial, "images", ["https://media.example.com/first.png", "https://media.example.com/ignored.png"], config);
        const withStyles = appendCanvasCustomVideoMedia(withImage, "style_references", ["https://media.example.com/style-1.png", "https://media.example.com/style-2.png", "https://media.example.com/ignored-style.png"], config);
        const next = removeCanvasCustomVideoMedia(withStyles, "style_references", 0, config);

        expect(next.media.images).toEqual(["https://media.example.com/first.png"]);
        expect(next.media.style_references).toEqual(["https://media.example.com/style-2.png"]);
        expect(next.media.input_video).toEqual([]);
    });

    test("uses a compact collapsed summary until the role inputs are explicitly opened", () => {
        expect(canvasCustomVideoReferenceRoles(config)).toEqual(["images", "style_references", "reference_images", "input_video"]);

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
        expect(markup).not.toContain("普通参考图");
    });
});
