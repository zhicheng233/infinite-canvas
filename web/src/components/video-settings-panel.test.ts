import { describe, expect, test } from "bun:test";

import { createDefaultCustomVideoConfig, type CustomVideoConfig } from "@/lib/custom-video-config";
import { createEmptyCustomVideoMediaState } from "@/lib/custom-video-runtime";
import { defaultConfig, type AiConfig } from "@/stores/use-config-store";
import { videoSettingsOverview } from "./video-settings-panel";

const customModel = "custom-video";

function configWithCustomVideo(customVideoConfig: CustomVideoConfig): AiConfig {
    return {
        ...defaultConfig,
        modelRoutes: { [`video:${customModel}`]: "custom" },
        modelCustomVideoConfigs: { [customModel]: customVideoConfig },
    };
}

function customConfig(overrides: Partial<CustomVideoConfig> = {}): CustomVideoConfig {
    return {
        ...createDefaultCustomVideoConfig(),
        seconds: { enabled: true, key: "seconds", mode: "range", min: 3, max: 10, step: 1, default: 6 },
        dimensions: { enabled: true, mode: "size", key: "size", options: ["1280x720", "720x1280"], default: "1280x720" },
        ...overrides,
    };
}

describe("video settings overview", () => {
    test("uses the selected custom size and runtime duration", () => {
        const config = configWithCustomVideo(customConfig());

        expect(videoSettingsOverview(config, customModel, { values: { dimension: "720x1280", seconds: 10 }, media: createEmptyCustomVideoMediaState() })).toBe("720x1280 · 10s");
    });

    test("uses the selected custom aspect ratio", () => {
        const config = configWithCustomVideo(customConfig({ dimensions: { enabled: true, mode: "aspect_ratio", key: "ratio", options: ["16:9", "9:16"], default: "16:9" } }));

        expect(videoSettingsOverview(config, customModel, { values: { dimension: "9:16" }, media: createEmptyCustomVideoMediaState() })).toBe("9:16 · 6s");
    });

    test("switches between standard and custom model values without stale custom state", () => {
        const config = configWithCustomVideo(customConfig());
        const standardConfig = { ...config, vquality: "1080", size: "1024x1024", videoSeconds: "12" };

        expect(videoSettingsOverview(standardConfig, "standard-video", { values: { dimension: "720x1280", seconds: 10 }, media: createEmptyCustomVideoMediaState() })).toBe("1080p · 方形 · 12s");
        expect(videoSettingsOverview(standardConfig, customModel, { values: { dimension: "720x1280", seconds: 10 }, media: createEmptyCustomVideoMediaState() })).toBe("720x1280 · 10s");
    });

    test("normalizes stale runtime values against the current custom model", () => {
        const config = configWithCustomVideo(customConfig());

        expect(videoSettingsOverview(config, customModel, { values: { dimension: "9:16", seconds: 20 }, media: createEmptyCustomVideoMediaState() })).toBe("1280x720 · 6s");
    });

    test("does not add legacy defaults when custom dimensions and seconds are disabled", () => {
        const config = configWithCustomVideo(customConfig({
            seconds: { enabled: false, key: "seconds", mode: "range", min: 0, max: 0, step: 0, default: 0 },
            dimensions: { enabled: false, mode: "size", key: "size", options: [], default: "" },
        }));

        expect(videoSettingsOverview(config, customModel)).toBe("自定义视频");
    });
});
