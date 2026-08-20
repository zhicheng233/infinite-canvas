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

const formerMediaHardLimits = { images: 1, input_reference: 1, style_references: 4, element_references: 3, reference_images: 4, input_video: 1 } as const;
const aboveFormerCapMediaCounts = { images: 2, input_reference: 2, style_references: 5, element_references: 4, reference_images: 5, input_video: 2 } as const;

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

    test("defaults expose required and max_count only on the six media roles", () => {
        const config = createDefaultCustomVideoConfig();

        const mediaRequired = customVideoMediaFeatureNames.map((role) => config[role].required);
        const mediaMaxCounts = customVideoMediaFeatureNames.map((role) => config[role].max_count);
        const scalarFields = [config.seconds, config.dimensions, config.reference_mode, config.audio, config.n];

        expect(mediaRequired).toEqual([false, false, false, false, false, false]);
        expect(mediaMaxCounts).toEqual([1, 1, 1, 1, 1, 1]);
        expect(scalarFields.every((field) => !("required" in field))).toBe(true);
    });

    test("summary retains enabled optional and required media semantics", () => {
        const config = requiredSnapshotConfig();

        const summary = summarizeCustomVideoConfig(config);

        expect(summary.media_required).toEqual({ images: false, input_reference: true, style_references: false, element_references: true, reference_images: false, input_video: true });
    });
});

describe("custom video media max_count contract", () => {
    for (const role of customVideoMediaFeatureNames) {
        for (const maxCount of [formerMediaHardLimits[role] + 1, 12, Number.MAX_SAFE_INTEGER]) {
            test(`${role} accepts positive safe integer max_count ${maxCount}`, () => {
                const defaults = createDefaultCustomVideoConfig();
                const result = normalizeAndValidateCustomVideoConfig({
                    ...defaults,
                    [role]: { ...defaults[role], enabled: true, max_count: maxCount },
                });

                expect(result.ok).toBe(true);
                if (!result.ok) return;
                expect(result.config[role].max_count).toBe(maxCount);
                expect(summarizeCustomVideoConfig(result.config).media_limits[role]).toBe(maxCount);
            });
        }

        for (const maxCount of [0, -1, 1.5, Number.MAX_SAFE_INTEGER + 1]) {
            test(`${role} rejects enabled max_count ${maxCount}`, () => {
                const defaults = createDefaultCustomVideoConfig();
                const result = normalizeAndValidateCustomVideoConfig({
                    ...defaults,
                    [role]: { ...defaults[role], enabled: true, max_count: maxCount },
                });

                expect(result.ok).toBe(false);
                if (result.ok) return;
                expect(result.errors.some((error) => error.includes(`${role}.max_count`))).toBe(true);
            });
        }

        test(`${role} tolerates disabled max_count zero and clears required`, () => {
            const defaults = createDefaultCustomVideoConfig();
            const result = normalizeAndValidateCustomVideoConfig({
                ...defaults,
                [role]: { ...defaults[role], required: true, max_count: 0 },
            });

            expect(result.ok).toBe(true);
            if (!result.ok) return;
            expect(result.config[role]).toEqual({ ...defaults[role], required: false, max_count: 0 });
        });

        test(`${role} rejects zero when a disabled config is re-enabled`, () => {
            const defaults = createDefaultCustomVideoConfig();
            const disabledResult = normalizeAndValidateCustomVideoConfig({
                ...defaults,
                [role]: { ...defaults[role], required: true, max_count: 0 },
            });
            expect(disabledResult.ok).toBe(true);
            if (!disabledResult.ok) return;

            const enabledResult = normalizeAndValidateCustomVideoConfig({
                ...disabledResult.config,
                [role]: { ...disabledResult.config[role], enabled: true },
            });

            expect(enabledResult.ok).toBe(false);
            if (enabledResult.ok) return;
            expect(enabledResult.errors.some((error) => error.includes(`${role}.max_count`))).toBe(true);
        });
    }
});

test("duplicate enabled aliases remain invalid", () => {
    const defaults = createDefaultCustomVideoConfig();
    const result = normalizeAndValidateCustomVideoConfig({
        ...defaults,
        images: { ...defaults.images, enabled: true, key: "duplicate" },
        input_video: { ...defaults.input_video, enabled: true, key: "duplicate" },
    });

    expect(result.ok).toBe(false);
    if (result.ok) return;
    expect(result.errors.some((error) => error.includes("重复"))).toBe(true);
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

test("custom channel and preset snapshots retain distinct above-cap media counts", () => {
    const config = aboveFormerCapSnapshotConfig();
    const update = normalizeChannelModelUpdateInput({ video_route: "custom", video_custom_config: config });

    expect(update.video_route).toBe("custom");
    expect(Object.keys(update).toSorted()).toEqual(["video_custom_config", "video_route"]);
    expect("id" in update).toBe(false);
    expect("preset_id" in update).toBe(false);
    if (!update.video_custom_config) throw new Error("custom channel config was not normalized");
    expect(mediaCounts(update.video_custom_config)).toEqual(aboveFormerCapMediaCountValues());
    expect(mediaCounts(update.video_custom_config)).not.toEqual([1, 1, 1, 1, 1, 1]);

    const channelModelSnapshot: unknown = JSON.parse(JSON.stringify(update));
    const channelModelConfig = normalizeSnapshotConfig(channelModelSnapshot, "video_custom_config");
    expect(mediaCounts(channelModelConfig)).toEqual(aboveFormerCapMediaCountValues());
    expect(channelModelConfig && "id" in channelModelConfig).toBe(false);
    expect(isRecord(channelModelSnapshot) ? channelModelSnapshot.preset_id : undefined).toBeUndefined();

    const preset: VideoConfigPreset = { id: 91, name: "above former caps", config, created_at: "", updated_at: "" };
    const presetSnapshot: unknown = JSON.parse(JSON.stringify(preset));
    const presetConfig = normalizeSnapshotConfig(presetSnapshot, "config");
    expect(mediaCounts(presetConfig)).toEqual(aboveFormerCapMediaCountValues());
    expect(presetConfig && "id" in presetConfig).toBe(false);
});

test("non-custom channel routes clear custom configuration", () => {
    const update = normalizeChannelModelUpdateInput({ video_route: "provider", video_custom_config: aboveFormerCapSnapshotConfig() });

    expect(update.video_route).toBe("provider");
    expect(update.video_custom_config).toBeNull();
});

describe("custom channel payload validation boundary", () => {
    for (const role of customVideoMediaFeatureNames) {
        for (const maxCount of [0, Number.MAX_SAFE_INTEGER + 1]) {
            test(`${role} rejects enabled ${maxCount} before payload creation`, () => {
                const defaults = createDefaultCustomVideoConfig();
                const input = {
                    video_route: "custom",
                    video_custom_config: {
                        ...defaults,
                        [role]: { ...defaults[role], enabled: true, max_count: maxCount },
                    },
                };

                expect(() => normalizeChannelModelUpdateInput(input)).toThrow("video_custom_config 无效");
            });
        }
    }
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

function aboveFormerCapSnapshotConfig(): CustomVideoConfig {
    const defaults = createDefaultCustomVideoConfig();
    const result = normalizeAndValidateCustomVideoConfig({
        ...defaults,
        images: { ...defaults.images, enabled: true, max_count: aboveFormerCapMediaCounts.images },
        input_reference: { ...defaults.input_reference, enabled: true, max_count: aboveFormerCapMediaCounts.input_reference },
        style_references: { ...defaults.style_references, enabled: true, max_count: aboveFormerCapMediaCounts.style_references },
        element_references: { ...defaults.element_references, enabled: true, max_count: aboveFormerCapMediaCounts.element_references },
        reference_images: { ...defaults.reference_images, enabled: true, max_count: aboveFormerCapMediaCounts.reference_images },
        input_video: { ...defaults.input_video, enabled: true, max_count: aboveFormerCapMediaCounts.input_video },
    });
    if (!result.ok) throw new Error(result.errors.join("; "));
    return result.config;
}

function aboveFormerCapMediaCountValues(): readonly number[] {
    return customVideoMediaFeatureNames.map((role) => aboveFormerCapMediaCounts[role]);
}

function mediaCounts(config: CustomVideoConfig | null): readonly number[] {
    return config ? customVideoMediaFeatureNames.map((role) => config[role].max_count) : [];
}

function normalizeSnapshotConfig(snapshot: unknown, field: string): CustomVideoConfig | null {
    return isRecord(snapshot) ? normalizeCustomVideoConfig(snapshot[field]) : null;
}

function requiredFlags(config: CustomVideoConfig | null): readonly boolean[] {
    return config ? customVideoMediaFeatureNames.map((role) => config[role].required) : [];
}

function isRecord(value: unknown): value is Record<string, unknown> {
    return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}
