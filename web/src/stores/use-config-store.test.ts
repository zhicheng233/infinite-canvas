import { describe, expect, test } from "bun:test";
import {
    buildChannelModelOptions,
    buildProxyApiUrl,
    decodeChannelModel,
    defaultConfig,
    defaultModelForCapability,
    encodeChannelModelIdentity,
    modelOptionName,
    persistedConfigState,
    resolveModelRequestConfig,
    selectedChannelId,
    selectedChannelIdentityForModel,
    useConfigStore,
} from "./use-config-store";

const channels = [
    { id: 1, name: "A", enabled: true, sync_status: "success" },
    { id: 2, name: "B", enabled: true, sync_status: "success" },
];
const models = {
    1: [
        { id: 11, channel_id: 1, model_name: "same-model", capabilities: ["image"], enabled: true, image_generate_route: "auto", image_edit_route: "auto", video_route: "auto", video_durations: [], video_customizable: false, sort_order: 2 },
        { id: 12, channel_id: 1, model_name: "zero-model", capabilities: ["image"], enabled: true, image_generate_route: "auto", image_edit_route: "auto", video_route: "auto", video_durations: [], video_customizable: false, sort_order: 1 },
    ],
    2: [
        { id: 22, channel_id: 2, model_name: "same-model", capabilities: ["image"], enabled: true, image_generate_route: "auto", image_edit_route: "auto", video_route: "auto", video_durations: [], video_customizable: false, sort_order: 0 },
        { id: 23, channel_id: 2, model_name: "stale-model", capabilities: ["image"], enabled: true, image_generate_route: "auto", image_edit_route: "auto", video_route: "auto", video_durations: [], video_customizable: false, sort_order: 3 },
    ],
};
const pricing = [
    { model: "same-model", credits_per_unit: 1, unit_type: "per_image" },
    { model: "zero-model", credits_per_unit: 1, unit_type: "per_image" },
    { model: "stale-model", credits_per_unit: 1, unit_type: "per_image" },
];

test("canonical identity keeps same raw model names distinct", () => {
    const value = encodeChannelModelIdentity(2, 22, " same-model ");
    expect(value).toBe("2::22::same-model");
    expect(decodeChannelModel(value)).toEqual({ channelId: "2", channelModelId: 22, model: "same-model" });
    expect(modelOptionName(value)).toBe("same-model");
});

test("four capability selections remain independent", () => {
    const config = { ...defaultConfig, imageChannelId: 1, videoChannelId: 2, textChannelId: 1, audioChannelId: 2, imageModel: "image", videoModel: "video", textModel: "text", audioModel: "audio" };
    expect(["image", "video", "text", "audio"].map((capability) => selectedChannelId(config, capability as never))).toEqual([1, 2, 1, 2]);
    expect(["image", "video", "text", "audio"].map((capability) => defaultModelForCapability(config, capability as never))).toEqual(["image", "video", "text", "audio"]);
});

test("rates sort descending with numeric zero before unavailable metrics", () => {
    const options = buildChannelModelOptions(
        channels,
        models,
        pricing,
        {
            channels: [
                { channel_id: 1, success_rate: 0, status: "ok", models: [{ channel_model_id: 12, channel_id: 1, model_name: "zero-model", request_count: 1, success_count: 0, success_rate: 0, status: "ok" }] },
                {
                    channel_id: 2,
                    success_rate: 90,
                    status: "ok",
                    models: [
                        { channel_model_id: 22, channel_id: 2, model_name: "same-model", request_count: 1, success_count: 1, success_rate: 90, status: "ok" },
                        { channel_model_id: 23, channel_id: 2, model_name: "stale-model", request_count: 1, success_count: 1, success_rate: 99, status: "stale" },
                    ],
                },
            ],
        },
        "image",
    );
    expect(options.map((option) => option.channelModelId)).toEqual([22, 12, 11, 23]);
    expect(options.find((option) => option.channelModelId === 12)?.successRate).toBe(0);
    expect(options.find((option) => option.channelModelId === 23)?.successRate).toBeNull();
});

test("persisted state excludes catalogs, identity, and API keys", () => {
    const state = persistedConfigState({ config: { ...defaultConfig, apiKey: "secret", baseUrl: "https://secret", channelModelId: 22 } as typeof defaultConfig & Record<string, unknown>, webdav: {} as never });
    expect(state.config).not.toHaveProperty("apiKey");
    expect(state.config).not.toHaveProperty("baseUrl");
    expect(state.config).not.toHaveProperty("channelModelId");
    expect(state.config).not.toHaveProperty("models");
});

test("authenticated stale identity fails closed and valid request includes both IDs", () => {
    useConfigStore.getState().applyServerChannelCatalog(channels, models);
    const config = { ...useConfigStore.getState().config, imageChannelId: 2, imageModel: "2::22::same-model" };
    const valid = buildProxyApiUrl("https://app.test/backend-api", config, config.imageModel, "/images/generations");
    const query = new URL(valid).searchParams;
    expect(query.get("channel_id")).toBe("2");
    expect(query.get("channel_model_id")).toBe("22");
    expect(selectedChannelIdentityForModel(config, "2::999::same-model")).toBeNull();
    expect(() => buildProxyApiUrl("https://app.test/backend-api", config, "2::999::same-model", "/images/generations")).toThrow("所选模型已失效");
    expect(resolveModelRequestConfig(config, "2::999::same-model").model).toBe("same-model");
});

describe("buildChannelModelOptions pricing-gated filter", () => {
    const pricedModel = { id: 1, channel_id: 1, model_name: "gpt-4", enabled: true, capabilities: ["text"], image_generate_route: "auto", image_edit_route: "auto", video_route: "auto", video_durations: [], video_customizable: false, sort_order: 1 };
    const unpricedModel = { id: 2, channel_id: 1, model_name: "gemini-2", enabled: true, capabilities: ["text"], image_generate_route: "auto", image_edit_route: "auto", video_route: "auto", video_durations: [], video_customizable: false, sort_order: 2 };
    const testChannels = [{ id: 1, name: "Test", enabled: true }];
    const testModels = { 1: [pricedModel, unpricedModel] };

    test("excludes models without pricing records", () => {
        const pricingOnly = [{ model: "gpt-4", credits_per_unit: 1, unit_type: "per_token" }];
        const options = buildChannelModelOptions(testChannels, testModels, pricingOnly, null, "text");
        expect(options).toHaveLength(1);
        expect(options[0].rawModel).toBe("gpt-4");
    });

    test("includes model after pricing record is added", () => {
        const initialPricing = [{ model: "gpt-4", credits_per_unit: 1, unit_type: "per_token" }];
        let options = buildChannelModelOptions(testChannels, testModels, initialPricing, null, "text");
        expect(options).toHaveLength(1);

        const updatedPricing = [
            ...initialPricing,
            { model: "gemini-2", credits_per_unit: 2, unit_type: "per_token" },
        ];
        options = buildChannelModelOptions(testChannels, testModels, updatedPricing, null, "text");
        expect(options).toHaveLength(2);
        expect(options.map((o) => o.rawModel)).toContain("gpt-4");
        expect(options.map((o) => o.rawModel)).toContain("gemini-2");
    });

    test("excludes model after pricing record is removed", () => {
        const initialPricing = [
            { model: "gpt-4", credits_per_unit: 1, unit_type: "per_token" },
            { model: "gemini-2", credits_per_unit: 2, unit_type: "per_token" },
        ];
        let options = buildChannelModelOptions(testChannels, testModels, initialPricing, null, "text");
        expect(options).toHaveLength(2);

        const updatedPricing = [initialPricing[0]];
        options = buildChannelModelOptions(testChannels, testModels, updatedPricing, null, "text");
        expect(options).toHaveLength(1);
        expect(options[0].rawModel).toBe("gpt-4");
    });
});
