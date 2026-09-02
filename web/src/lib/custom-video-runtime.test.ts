import { describe, expect, test } from "bun:test";

import type { CanvasNodeMetadata } from "@/app/(user)/canvas/types";
import { customVideoMediaFeatureNames, type CustomVideoConfig } from "./custom-video-config";
import { createEmptyCustomVideoMediaState, normalizeCustomVideoRuntimeContainer, normalizeCustomVideoRuntimeState, normalizeCustomVideoRuntimeStateForModelSwitch, type CustomVideoRuntimeContainer, type CustomVideoRuntimeSnapshot } from "./custom-video-runtime";

const sourceConfig: CustomVideoConfig = {
    seconds: { enabled: true, key: "seconds", mode: "range", min: 3, max: 10, step: 1, default: 6 },
    dimensions: { enabled: true, mode: "size", key: "size", options: ["1280x720", "720x1280"], default: "1280x720" },
    images: { enabled: true, required: false, key: "images", max_count: 1 },
    input_reference: { enabled: true, required: false, key: "input_reference", max_count: 1 },
    style_references: { enabled: true, required: false, key: "style_references", max_count: 4 },
    element_references: { enabled: true, required: false, key: "element_references", max_count: 3 },
    reference_images: { enabled: true, required: false, key: "reference_images", max_count: 4 },
    reference_mode: { enabled: true, key: "reference_mode", options: ["frame", "style", "element"], default: "element" },
    input_video: { enabled: true, required: false, key: "input_video", max_count: 1 },
    audio: { enabled: true, key: "audio", mode: "user", value: true },
    n: { enabled: true, key: "n", value: 1 },
};

const configuredLimitConfig: CustomVideoConfig = {
    ...sourceConfig,
    images: { enabled: true, required: false, key: "images", max_count: 2 },
    input_reference: { enabled: false, required: false, key: "input_reference", max_count: 2 },
    style_references: { enabled: true, required: false, key: "style_references", max_count: 5 },
    element_references: { enabled: true, required: false, key: "element_references", max_count: 6 },
    reference_images: { enabled: true, required: false, key: "reference_images", max_count: 5 },
    input_video: { enabled: true, required: false, key: "input_video", max_count: 2 },
};

function mediaSources(name: string, count: number) {
    return Array.from({ length: count }, (_, index) => `https://example.com/${name}-${index + 1}`);
}

describe("custom video runtime state", () => {
    test("keeps same-role media through model switches up to each configured limit", () => {
        const targetConfig: CustomVideoConfig = {
            ...configuredLimitConfig,
            seconds: { enabled: true, key: "seconds", mode: "options", options: [5, 8], default: 8 },
            dimensions: { enabled: true, mode: "aspect_ratio", key: "ratio", options: ["16:9", "9:16"], default: "9:16" },
            reference_mode: { enabled: true, key: "reference_mode", options: ["frame", "style"], default: "style" },
        };

        const normalized = normalizeCustomVideoRuntimeStateForModelSwitch(
            targetConfig,
            { seconds: 7, dimension: "1280x720", reference_mode: "element", audio: false },
            {
                images: mediaSources("image", 3),
                input_reference: mediaSources("stale-input", 2),
                style_references: mediaSources("style", 6),
                element_references: mediaSources("element", 7),
                reference_images: mediaSources("reference", 6),
                input_video: mediaSources("video", 3),
                unknown_role: ["https://example.com/unknown.png"],
            },
        );

        expect(normalized.values).toEqual({ seconds: 8, dimension: "9:16", reference_mode: "style", audio: false });
        expect(normalized.media).toEqual({
            images: mediaSources("image", 2),
            input_reference: [],
            style_references: mediaSources("style", 5),
            element_references: mediaSources("element", 6),
            reference_images: mediaSources("reference", 5),
            input_video: mediaSources("video", 2),
        });
        expect(Object.keys(normalized.media)).toEqual([...customVideoMediaFeatureNames]);
    });

    test("preserves scalar values that remain valid after switching models", () => {
        const targetConfig: CustomVideoConfig = {
            ...sourceConfig,
            seconds: { enabled: true, key: "seconds", mode: "options", options: [6, 8], default: 6 },
            dimensions: { enabled: true, mode: "size", key: "size", options: ["720x1280", "1920x1080"], default: "1920x1080" },
            reference_mode: { enabled: true, key: "reference_mode", options: ["style", "element"], default: "element" },
        };

        const normalized = normalizeCustomVideoRuntimeStateForModelSwitch(targetConfig, { seconds: 8, dimension: "720x1280", reference_mode: "style", audio: false });

        expect(normalized.values).toEqual({ seconds: 8, dimension: "720x1280", reference_mode: "style", audio: false });
    });

    test("clears disabled roles and clips single-value roles to one URL", () => {
        const targetConfig: CustomVideoConfig = {
            ...sourceConfig,
            images: { enabled: false, required: false, key: "images", max_count: 1 },
            audio: { enabled: true, key: "audio", mode: "fixed", value: true },
        };

        const normalized = normalizeCustomVideoRuntimeState(
            targetConfig,
            {},
            {
                images: ["https://example.com/disabled.png"],
                input_reference: ["https://example.com/input-1.png", "https://example.com/input-2.png"],
                style_references: [],
                element_references: [],
                reference_images: [],
                input_video: ["data:video/mp4;base64,AAAA", "https://example.com/input-2.mp4"],
            },
        );

        expect(normalized.media.images).toEqual([]);
        expect(normalized.media.input_reference).toEqual(["https://example.com/input-1.png"]);
        expect(normalized.media.input_video).toEqual(["data:video/mp4;base64,AAAA"]);
        expect(normalized.values.audio).toBeUndefined();
    });

    test("clears reference_mode unless both dependencies are enabled", () => {
        const withoutReferenceImages: CustomVideoConfig = {
            ...sourceConfig,
            reference_images: { enabled: false, required: false, key: "reference_images", max_count: 4 },
        };
        const withoutReferenceMode: CustomVideoConfig = {
            ...sourceConfig,
            reference_mode: { enabled: false, key: "reference_mode", options: [], default: "" },
        };

        expect(normalizeCustomVideoRuntimeState(withoutReferenceImages, { reference_mode: "frame" }, {}).values.reference_mode).toBeUndefined();
        expect(normalizeCustomVideoRuntimeState(withoutReferenceMode, { reference_mode: "frame" }, {}).values.reference_mode).toBeUndefined();
        expect(normalizeCustomVideoRuntimeState(sourceConfig, {}, {}).values.reference_mode).toBe("element");
    });

    test("filters non-string media objects from persisted snapshots", () => {
        const normalized = normalizeCustomVideoRuntimeState(
            sourceConfig,
            {},
            {
                style_references: ["https://example.com/reference.png", "/backend-api/files/reference.png", "blob:https://example.com/transient", { name: "reference.png" }, new Blob(["image"])],
            },
        );

        expect(normalized.media.style_references).toEqual(["https://example.com/reference.png", "/backend-api/files/reference.png", "blob:https://example.com/transient"]);
    });

    test("reopens persisted runtime containers with configured media counts intact", () => {
        const persistedRuntime: CustomVideoRuntimeSnapshot = {
            values: { seconds: 8, dimension: "720x1280", reference_mode: "style", audio: false },
            media: {
                images: mediaSources("persisted-image", 3),
                input_reference: mediaSources("persisted-stale-input", 2),
                style_references: mediaSources("persisted-style", 6),
                element_references: mediaSources("persisted-element", 7),
                reference_images: mediaSources("persisted-reference", 6),
                input_video: mediaSources("persisted-video", 3),
            },
        };

        const canvasMetadata = { model: "custom-video", customVideoRuntime: persistedRuntime } satisfies CanvasNodeMetadata;
        const generationLog = { customVideoRuntime: persistedRuntime } satisfies CustomVideoRuntimeContainer;
        const reopenedCanvasMetadata: CanvasNodeMetadata = JSON.parse(JSON.stringify(canvasMetadata));
        const reopenedGenerationLog: CustomVideoRuntimeContainer = JSON.parse(JSON.stringify(generationLog));
        const expectedRuntime: CustomVideoRuntimeSnapshot = {
            values: persistedRuntime.values,
            media: {
                images: mediaSources("persisted-image", 2),
                input_reference: [],
                style_references: mediaSources("persisted-style", 5),
                element_references: mediaSources("persisted-element", 6),
                reference_images: mediaSources("persisted-reference", 5),
                input_video: mediaSources("persisted-video", 2),
            },
        };

        expect(normalizeCustomVideoRuntimeContainer(configuredLimitConfig, reopenedCanvasMetadata)).toEqual({ model: "custom-video", customVideoRuntime: expectedRuntime });
        expect(normalizeCustomVideoRuntimeContainer(configuredLimitConfig, reopenedGenerationLog)).toEqual({ customVideoRuntime: expectedRuntime });
    });

    test("leaves legacy non-custom metadata unchanged", () => {
        const legacyMetadata: CanvasNodeMetadata = {
            model: "legacy-video",
            seconds: "6",
            references: ["https://example.com/legacy.png"],
            videoReferenceMode: "reference",
        };

        const normalized = normalizeCustomVideoRuntimeContainer(null, legacyMetadata);

        expect(normalized).toBe(legacyMetadata);
        expect(normalized).toEqual(legacyMetadata);
    });

    test("normalizes a configured snapshot through the shared container boundary", () => {
        const metadata: CanvasNodeMetadata = {
            model: "custom-video",
            customVideoRuntime: {
                values: { seconds: 99, dimension: "invalid", reference_mode: "frame" },
                media: {
                    ...createEmptyCustomVideoMediaState(),
                    style_references: ["https://example.com/style-1.png", "https://example.com/style-2.png", "https://example.com/style-3.png", "https://example.com/style-4.png"],
                },
            },
        };
        const targetConfig: CustomVideoConfig = {
            ...sourceConfig,
            style_references: { enabled: true, required: false, key: "style_references", max_count: 2 },
        };

        const normalized = normalizeCustomVideoRuntimeContainer(targetConfig, metadata);

        expect(normalized).toEqual({
            model: "custom-video",
            customVideoRuntime: {
                values: { seconds: 6, dimension: "1280x720", reference_mode: "frame", audio: true },
                media: {
                    ...createEmptyCustomVideoMediaState(),
                    style_references: ["https://example.com/style-1.png", "https://example.com/style-2.png"],
                },
            },
        });
        expect(normalized).not.toBe(metadata);
    });
});
