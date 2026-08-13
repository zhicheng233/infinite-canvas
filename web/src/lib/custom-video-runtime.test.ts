import { describe, expect, test } from "bun:test";

import type { CanvasNodeMetadata } from "@/app/(user)/canvas/types";
import { customVideoMediaFeatureNames, type CustomVideoConfig } from "./custom-video-config";
import { createEmptyCustomVideoMediaState, normalizeCustomVideoRuntimeContainer, normalizeCustomVideoRuntimeState, type CustomVideoRuntimeContainer } from "./custom-video-runtime";

const sourceConfig: CustomVideoConfig = {
    seconds: { enabled: true, key: "seconds", mode: "range", min: 3, max: 10, step: 1, default: 6 },
    dimensions: { enabled: true, mode: "size", key: "size", options: ["1280x720", "720x1280"], default: "1280x720" },
    images: { enabled: true, key: "images", max_count: 1 },
    input_reference: { enabled: true, key: "input_reference", max_count: 1 },
    style_references: { enabled: true, key: "style_references", max_count: 4 },
    element_references: { enabled: true, key: "element_references", max_count: 3 },
    reference_images: { enabled: true, key: "reference_images", max_count: 4 },
    reference_mode: { enabled: true, key: "reference_mode", options: ["frame", "style", "element"], default: "element" },
    input_video: { enabled: true, key: "input_video", max_count: 1 },
    audio: { enabled: true, key: "audio", mode: "user", value: true },
    n: { enabled: true, key: "n", value: 1 },
};

describe("custom video runtime state", () => {
    test("falls back invalid scalar values and trims media when switching models", () => {
        const targetConfig: CustomVideoConfig = {
            ...sourceConfig,
            seconds: { enabled: true, key: "seconds", mode: "options", options: [5, 8], default: 8 },
            dimensions: { enabled: true, mode: "aspect_ratio", key: "ratio", options: ["16:9", "9:16"], default: "9:16" },
            style_references: { enabled: true, key: "style_references", max_count: 2 },
            element_references: { enabled: false, key: "element_references", max_count: 3 },
            reference_mode: { enabled: true, key: "reference_mode", options: ["frame", "style"], default: "style" },
        };

        const normalized = normalizeCustomVideoRuntimeState(
            targetConfig,
            { seconds: 7, dimension: "1280x720", reference_mode: "element", audio: false },
            {
                images: ["https://example.com/first.png"],
                input_reference: [],
                style_references: ["https://example.com/style-1.png", "https://example.com/style-2.png", "https://example.com/style-3.png"],
                element_references: ["https://example.com/element.png"],
                reference_images: [],
                input_video: [],
                unknown_role: ["https://example.com/unknown.png"],
            },
        );

        expect(normalized.values).toEqual({ seconds: 8, dimension: "9:16", reference_mode: "style", audio: false });
        expect(normalized.media.style_references).toEqual(["https://example.com/style-1.png", "https://example.com/style-2.png"]);
        expect(normalized.media.element_references).toEqual([]);
        expect(Object.keys(normalized.media)).toEqual(customVideoMediaFeatureNames);
    });

    test("preserves scalar values that remain valid after switching models", () => {
        const targetConfig: CustomVideoConfig = {
            ...sourceConfig,
            seconds: { enabled: true, key: "seconds", mode: "options", options: [6, 8], default: 6 },
            dimensions: { enabled: true, mode: "size", key: "size", options: ["720x1280", "1920x1080"], default: "1920x1080" },
            reference_mode: { enabled: true, key: "reference_mode", options: ["style", "element"], default: "element" },
        };

        const normalized = normalizeCustomVideoRuntimeState(targetConfig, { seconds: 8, dimension: "720x1280", reference_mode: "style", audio: false });

        expect(normalized.values).toEqual({ seconds: 8, dimension: "720x1280", reference_mode: "style", audio: false });
    });

    test("clears disabled roles and clips single-value roles to one URL", () => {
        const targetConfig: CustomVideoConfig = {
            ...sourceConfig,
            images: { enabled: false, key: "images", max_count: 1 },
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
            reference_images: { enabled: false, key: "reference_images", max_count: 4 },
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

    test("round-trips canonical scalar values and role media through snapshot JSON", () => {
        const snapshot = normalizeCustomVideoRuntimeState(
            sourceConfig,
            { seconds: 8, dimension: "720x1280", reference_mode: "style", audio: false },
            {
                reference_images: ["data:image/png;base64,AAAA"],
                input_video: ["blob:https://example.com/video"],
            },
        );

        const canvasMetadata = { customVideoRuntime: snapshot } satisfies CanvasNodeMetadata;
        const generationLog = { customVideoRuntime: snapshot } satisfies CustomVideoRuntimeContainer;

        expect(JSON.parse(JSON.stringify(canvasMetadata))).toEqual({ customVideoRuntime: snapshot });
        expect(JSON.parse(JSON.stringify(generationLog))).toEqual({ customVideoRuntime: snapshot });
        expect(snapshot.values.reference_mode).toBe("style");
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
            style_references: { enabled: true, key: "style_references", max_count: 2 },
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
