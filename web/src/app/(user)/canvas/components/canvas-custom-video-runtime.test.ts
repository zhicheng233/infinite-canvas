import { describe, expect, test } from "bun:test";

import { canvasCustomVideoGenerationState, canvasCustomVideoRuntimeForModel, canvasVideoGenerationOptions, resolveCanvasCustomVideoGenerationState } from "./canvas-custom-video-runtime";
import type { CanvasNodeMetadata } from "../types";
import { defaultConfig, type AiConfig } from "@/stores/use-config-store";
import type { CustomVideoConfig } from "@/lib/custom-video-config";
import { customVideoRequiredMediaErrors, type CustomVideoRuntimeSnapshot } from "@/lib/custom-video-runtime";
import { resolveNodeGenerationImageSources } from "./canvas-node-generation";

const model = "canvas-custom-video";
const customConfig = {
    seconds: { enabled: true, key: "seconds", mode: "options", options: [5, 8], default: 8 },
    dimensions: { enabled: true, mode: "size", key: "size", options: ["1280x720"], default: "1280x720" },
    images: { enabled: false, required: false, key: "images", max_count: 1 },
    input_reference: { enabled: false, required: false, key: "input_reference", max_count: 1 },
    style_references: { enabled: true, required: false, key: "style_references", max_count: 2 },
    element_references: { enabled: false, required: false, key: "element_references", max_count: 3 },
    reference_images: { enabled: false, required: false, key: "reference_images", max_count: 4 },
    reference_mode: { enabled: false, key: "reference_mode", options: [], default: "" },
    input_video: { enabled: true, required: false, key: "input_video", max_count: 1 },
    audio: { enabled: false, key: "audio", mode: "fixed", value: false },
    n: { enabled: true, key: "n", value: 1 },
} satisfies CustomVideoConfig;

const config = {
    ...defaultConfig,
    model,
    videoModel: model,
    modelRoutes: { [`video:${model}`]: "custom" },
    modelCustomVideoConfigs: { [model]: customConfig },
} satisfies AiConfig;

const runtime: CustomVideoRuntimeSnapshot = {
    values: { seconds: 7, dimension: "invalid" },
    media: {
        images: ["https://example.com/ignored.png"],
        input_reference: [],
        style_references: ["https://example.com/style-1.png", "https://example.com/style-2.png", "https://example.com/style-3.png"],
        element_references: [],
        reference_images: [],
        input_video: ["https://example.com/video.mp4", "https://example.com/ignored.mp4"],
    },
};

describe("canvas custom video runtime", () => {
    test("trims role media and resets invalid runtime values on a model switch", () => {
        const normalized = canvasCustomVideoRuntimeForModel(config, model, runtime);

        expect(normalized).toEqual({
            values: { seconds: 8, dimension: "1280x720" },
            media: {
                images: [],
                input_reference: [],
                style_references: ["https://example.com/style-1.png", "https://example.com/style-2.png"],
                element_references: [],
                reference_images: [],
                input_video: ["https://example.com/video.mp4"],
            },
        });
    });

    test("uses the same normalized snapshot for an initial request and retry", () => {
        const controller = new AbortController();
        const initial = canvasVideoGenerationOptions(config, model, runtime, controller.signal);
        const retry = canvasVideoGenerationOptions(config, model, "customVideoRuntime" in initial ? initial.customVideoRuntime : undefined, controller.signal);

        expect(initial).toHaveProperty("customVideoRuntime");
        expect(retry).toEqual(initial);
    });

    test("allows prompt-only generation when the active model has no required media", () => {
        expect(canvasCustomVideoGenerationState(config, model, undefined)).toEqual({
            runtime: {
                values: { seconds: 8, dimension: "1280x720" },
                media: { images: [], input_reference: [], style_references: [], element_references: [], reference_images: [], input_video: [] },
            },
            error: undefined,
        });
    });

    test("keeps the normalized snapshot when video node metadata is persisted", () => {
        const normalized = canvasCustomVideoRuntimeForModel(config, model, runtime);
        const metadata = { model, customVideoRuntime: normalized } satisfies CanvasNodeMetadata;

        expect(JSON.parse(JSON.stringify(metadata))).toEqual(metadata);
    });

    test("reports a newly required destination role after model-switch normalization", () => {
        const requiredConfig = { ...customConfig, style_references: { ...customConfig.style_references, required: true } } satisfies CustomVideoConfig;
        const requiredModelConfig = { ...config, modelCustomVideoConfigs: { [model]: requiredConfig } } satisfies AiConfig;

        const normalized = canvasCustomVideoRuntimeForModel(requiredModelConfig, model, { ...runtime, media: { ...runtime.media, style_references: ["invalid"] } });
        const initial = canvasCustomVideoGenerationState(requiredModelConfig, model, normalized);
        const retry = canvasCustomVideoGenerationState(requiredModelConfig, model, initial.runtime);

        expect(normalized?.media.style_references).toEqual([]);
        expect(customVideoRequiredMediaErrors(requiredConfig, normalized?.media)).toEqual([{ role: "style_references", message: "缺少必填素材：风格参考图" }]);
        expect(initial).toMatchObject({ runtime: normalized, error: "缺少必填素材：风格参考图" });
        expect(retry).toEqual(initial);
    });

    test("keeps a reachable canvas image source for custom video generation", async () => {
        const sources = await resolveNodeGenerationImageSources([
            {
                image: {
                    id: "image-node",
                    name: "角色.png",
                    type: "image/png",
                    dataUrl: "blob:http://127.0.0.1/local-image",
                    url: "https://cdn.example.com/role.png",
                },
                targetImageRole: "style_references",
            },
        ]);

        expect(sources[0]?.source).toBe("https://cdn.example.com/role.png");
    });

    test("routes a legacy image connection to normal reference images", async () => {
        const imagesConfig = { ...customConfig, images: { ...customConfig.images, enabled: true }, style_references: { ...customConfig.style_references, enabled: false } } satisfies CustomVideoConfig;
        const result = await resolveCanvasCustomVideoGenerationState({ config: configWith(imagesConfig), model, graphImages: [legacyGraphImage()] });
        const explicitAlias = await resolveCanvasCustomVideoGenerationState({ config: configWith(imagesConfig), model, graphImages: [{ ...legacyGraphImage(), targetImageRole: "images" }] });

        expect(result.error).toBeUndefined();
        expect(result.runtime?.media.images).toEqual(["https://cdn.example.com/legacy.png"]);
        expect(explicitAlias.runtime?.media.images).toEqual(["https://cdn.example.com/legacy.png"]);
    });

    test("routes a legacy image connection to the sole enabled first-frame role", async () => {
        const firstFrameConfig = { ...customConfig, input_reference: { ...customConfig.input_reference, enabled: true }, style_references: { ...customConfig.style_references, enabled: false } } satisfies CustomVideoConfig;
        const result = await resolveCanvasCustomVideoGenerationState({ config: configWith(firstFrameConfig), model, graphImages: [legacyGraphImage()] });

        expect(result.error).toBeUndefined();
        expect(result.runtime?.media.input_reference).toEqual(["https://cdn.example.com/legacy.png"]);
    });

    test("rejects an ambiguous legacy image connection instead of guessing its role", async () => {
        const ambiguousConfig = { ...customConfig, input_reference: { ...customConfig.input_reference, enabled: true } } satisfies CustomVideoConfig;
        const result = await resolveCanvasCustomVideoGenerationState({ config: configWith(ambiguousConfig), model, graphImages: [legacyGraphImage()] });

        expect(result).toEqual({ error: "旧图片连线无法确定素材角色，请重新连接到可用的图片角色入口" });
    });
});

function configWith(videoConfig: CustomVideoConfig): AiConfig {
    return { ...config, modelCustomVideoConfigs: { [model]: videoConfig } };
}

function legacyGraphImage() {
    return {
        image: {
            id: "legacy-image",
            name: "旧版图片参考.png",
            type: "image/png",
            dataUrl: "https://cdn.example.com/legacy.png",
            url: "https://cdn.example.com/legacy.png",
        },
    };
}
