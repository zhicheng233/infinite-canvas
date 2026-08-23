import { describe, expect, test } from "bun:test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";

import { CustomVideoSettingsPanel, resolveCustomVideoSettingsState } from "./custom-video-settings-panel";
import { VideoSettingsPanel } from "./video-settings-panel";
import { canvasThemes } from "@/lib/canvas-theme";
import type { CustomVideoConfig } from "@/lib/custom-video-config";
import { createEmptyCustomVideoMediaState } from "@/lib/custom-video-runtime";
import { useConfigStore } from "@/stores/use-config-store";

const rangeConfig: CustomVideoConfig = {
    seconds: { enabled: true, key: "seconds", mode: "range", min: 3, max: 10, step: 1, default: 6 },
    dimensions: { enabled: true, mode: "size", key: "size", options: ["1280x720", "720x1280"], default: "1280x720" },
    images: { enabled: false, required: false, key: "images", max_count: 1 },
    input_reference: { enabled: false, required: false, key: "input_reference", max_count: 1 },
    style_references: { enabled: false, required: false, key: "style_references", max_count: 4 },
    element_references: { enabled: false, required: false, key: "element_references", max_count: 3 },
    reference_images: { enabled: false, required: false, key: "reference_images", max_count: 4 },
    reference_mode: { enabled: false, key: "reference_mode", options: [], default: "" },
    input_video: { enabled: false, required: false, key: "input_video", max_count: 1 },
    audio: { enabled: true, key: "audio", mode: "user", value: true },
    n: { enabled: true, key: "n", value: 1 },
};

describe("custom video settings panel state", () => {
    test("blocks the panel when the selected custom model lacks a valid config", () => {
        expect(resolveCustomVideoSettingsState(null)).toEqual({ kind: "invalid" });
    });

    test("keeps valid range, size, and user audio values", () => {
        const state = resolveCustomVideoSettingsState(rangeConfig, {
            values: { seconds: 8, dimension: "720x1280", audio: false },
            media: createEmptyCustomVideoMediaState(),
        });

        expect(state).toEqual({
            kind: "ready",
            config: rangeConfig,
            runtime: { values: { seconds: 8, dimension: "720x1280", audio: false }, media: createEmptyCustomVideoMediaState() },
        });
    });

    test("preserves invalid runtime values for serializer rejection", () => {
        const config: CustomVideoConfig = {
            ...rangeConfig,
            seconds: { enabled: true, key: "seconds", mode: "options", options: [5, 8], default: 5 },
            dimensions: { enabled: true, mode: "aspect_ratio", key: "aspect_ratio", options: ["16:9", "9:16"], default: "16:9" },
        };

        const state = resolveCustomVideoSettingsState(config, {
            values: { seconds: 6, dimension: "1:1" },
            media: createEmptyCustomVideoMediaState(),
        });

        expect(state).toEqual({
            kind: "ready",
            config,
            runtime: { values: { seconds: 6, dimension: "1:1", audio: true }, media: createEmptyCustomVideoMediaState() },
        });
    });

    test("renders a Slider for ranges and fixed size choices without an audio switch", () => {
        const markup = renderToStaticMarkup(createElement(CustomVideoSettingsPanel, { config: { ...rangeConfig, audio: { enabled: true, key: "audio", mode: "fixed", value: true } }, theme: canvasThemes.light, showTitle: false }));

        expect(markup).toContain("3s - 10s");
        expect(markup).toContain("1280x720");
        expect(markup).toContain("720x1280");
        expect(markup).not.toContain("宽高比");
        expect(markup).not.toContain("生成声音");
        expect(markup).not.toContain("生成张数");
    });

    test("renders compact seconds choices, aspect ratios, and the user audio switch", () => {
        const config: CustomVideoConfig = {
            ...rangeConfig,
            seconds: { enabled: true, key: "seconds", mode: "options", options: [5, 8], default: 5 },
            dimensions: { enabled: true, mode: "aspect_ratio", key: "aspect_ratio", options: ["16:9", "9:16"], default: "16:9" },
            reference_images: { enabled: true, required: false, key: "reference_images", max_count: 1 },
            reference_mode: { enabled: true, key: "reference_mode", options: ["frame", "style"], default: "style" },
        };
        const markup = renderToStaticMarkup(createElement(CustomVideoSettingsPanel, { config, theme: canvasThemes.dark, showTitle: false }));

        expect(markup).toContain("5s");
        expect(markup).toContain("8s");
        expect(markup).toContain("宽高比");
        expect(markup).toContain("16:9");
        expect(markup).toContain("9:16");
        expect(markup).toContain("参考图模式");
        expect(markup).toContain("首帧");
        expect(markup).toContain("风格");
        expect(markup).toContain("生成声音");
    });

    test("includes connected role media in the top material summary", () => {
        const config: CustomVideoConfig = { ...rangeConfig, input_reference: { enabled: true, required: false, key: "input_reference", max_count: 2 } };
        const markup = renderToStaticMarkup(
            createElement(CustomVideoSettingsPanel, {
                config,
                theme: canvasThemes.light,
                showTitle: false,
                connectedMedia: { input_reference: [{ nodeId: "image-1", title: "首帧.png", source: "https://example.com/first.png" }] },
            }),
        );

        expect(markup).toContain("已选 1 / 2");
    });

    test("renders a blocking message instead of generic controls for a missing custom config", () => {
        const markup = renderToStaticMarkup(createElement(CustomVideoSettingsPanel, { config: null, theme: canvasThemes.light, showTitle: false }));

        expect(markup).toContain("该模型的自定义视频配置无效，请联系管理员");
        expect(markup).toContain("生成已禁用");
        expect(markup).toContain('role="alert"');
        expect(markup).toContain('data-generation-blocked="true"');
        expect(markup).not.toContain("清晰度");
    });

    test("routes a custom model without catalog config to the blocking panel", () => {
        const config = {
            ...useConfigStore.getState().config,
            model: "custom-video",
            videoModel: "custom-video",
            modelRoutes: { "video:custom-video": "custom" },
            modelCustomVideoConfigs: {},
        };
        const markup = renderToStaticMarkup(createElement(VideoSettingsPanel, { config, model: "custom-video", onConfigChange: () => undefined, theme: canvasThemes.light, showTitle: false }));

        expect(markup).toContain("该模型的自定义视频配置无效，请联系管理员");
        expect(markup).not.toContain("720p");
    });

    test("routes a custom model with catalog config to the dedicated controls", () => {
        const config = {
            ...useConfigStore.getState().config,
            model: "custom-video",
            videoModel: "custom-video",
            modelRoutes: { "video:custom-video": "custom" },
            modelCustomVideoConfigs: { "custom-video": { ...rangeConfig, seconds: { enabled: true, key: "seconds", mode: "options", options: [5, 8], default: 5 } } },
        };
        const markup = renderToStaticMarkup(createElement(VideoSettingsPanel, { config, model: "custom-video", onConfigChange: () => undefined, theme: canvasThemes.light, showTitle: false }));

        expect(markup).toContain("5s");
        expect(markup).toContain("8s");
        expect(markup).not.toContain("清晰度");
    });
});
