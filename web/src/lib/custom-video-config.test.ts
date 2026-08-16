import { describe, expect, test } from "bun:test";

import type { ChannelModelInfo } from "@/services/api/channel";
import { normalizeChannelModelUpdateInput } from "@/services/api/channel-models-admin";
import type { VideoConfigPreset } from "@/services/api/video-config-presets";
import { createDefaultCustomVideoConfig, customVideoMediaFeatureNames, normalizeAndValidateCustomVideoConfig, normalizeCustomVideoConfig, summarizeCustomVideoConfig, type CustomVideoConfig } from "./custom-video-config";

const mediaSemantics = [
    { name: "disabled", enabled: false, required: true, expected: false },
    { name: "optional", enabled: true, required: false, expected: false },
    { name: "required", enabled: true, required: true, expected: true },
] as const;

describe("custom video media required contract", () => {
    for (const role of customVideoMediaFeatureNames) {
        for (const semantics of mediaSemantics) {
            test(`${role} preserves ${semantics.name} semantics when normalized`, () => {
                const defaults = createDefaultCustomVideoConfig();
                const input = {
                    ...defaults,
                    [role]: { ...defaults[role], enabled: semantics.enabled, required: semantics.required },
                };

                const result = normalizeAndValidateCustomVideoConfig(input);

                expect(result.ok).toBe(true);
                if (!result.ok) return;
                expect(result.config[role].required).toBe(semantics.expected);
            });
        }
    }

    test("defaults expose required only on the six media roles", () => {
        const config = createDefaultCustomVideoConfig();

        const mediaRequired = customVideoMediaFeatureNames.map((role) => config[role].required);
        const scalarFields = [config.seconds, config.dimensions, config.reference_mode, config.audio, config.n];

        expect(mediaRequired).toEqual([false, false, false, false, false, false]);
        expect(scalarFields.every((field) => !("required" in field))).toBe(true);
    });

    test("summary retains enabled optional and required media semantics", () => {
        const config = requiredSnapshotConfig();

        const summary = summarizeCustomVideoConfig(config);

        expect(summary.media_required).toEqual({ images: false, input_reference: true, style_references: false, element_references: true, reference_images: false, input_video: true });
    });
});

test("preset and channel-model JSON snapshots retain required flags", () => {
    const config = requiredSnapshotConfig();
    const preset: VideoConfigPreset = { id: 7, name: "required snapshot", config, created_at: "", updated_at: "" };
    const update = normalizeChannelModelUpdateInput({ video_route: "custom", video_custom_config: config });
    const channelModel: ChannelModelInfo = {
        id: 11,
        channel_id: 3,
        model_name: "custom-video",
        capabilities: ["video"],
        enabled: true,
        image_generate_route: "auto",
        image_edit_route: "auto",
        video_route: "custom",
        video_durations: [],
        video_customizable: false,
        video_custom_config: update.video_custom_config,
        sort_order: 0,
    };

    const presetSnapshot: unknown = JSON.parse(JSON.stringify(preset));
    const channelModelSnapshot: unknown = JSON.parse(JSON.stringify(channelModel));
    const presetConfig = normalizeCustomVideoConfig(isRecord(presetSnapshot) ? presetSnapshot.config : undefined);
    const channelModelConfig = normalizeCustomVideoConfig(isRecord(channelModelSnapshot) ? channelModelSnapshot.video_custom_config : undefined);

    expect(requiredFlags(presetConfig)).toEqual([false, true, false, true, false, true]);
    expect(requiredFlags(channelModelConfig)).toEqual([false, true, false, true, false, true]);
});

function requiredSnapshotConfig(): CustomVideoConfig {
    const defaults = createDefaultCustomVideoConfig();
    const result = normalizeAndValidateCustomVideoConfig({
        ...defaults,
        images: { ...defaults.images, enabled: true, required: false },
        input_reference: { ...defaults.input_reference, enabled: true, required: true },
        style_references: { ...defaults.style_references, enabled: true, required: false },
        element_references: { ...defaults.element_references, enabled: true, required: true },
        reference_images: { ...defaults.reference_images, enabled: true, required: false },
        input_video: { ...defaults.input_video, enabled: true, required: true },
    });
    if (!result.ok) throw new Error(result.errors.join("; "));
    return result.config;
}

function requiredFlags(config: CustomVideoConfig | null): readonly boolean[] {
    return config ? customVideoMediaFeatureNames.map((role) => config[role].required) : [];
}

function isRecord(value: unknown): value is Record<string, unknown> {
    return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}
