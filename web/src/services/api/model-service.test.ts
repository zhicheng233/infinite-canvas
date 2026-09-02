import { afterEach, describe, expect, it, jest } from "bun:test";

import apiClient from "./client";
import { listModelConfigs, listModelServiceChannels, saveDefaultModelPricing, testModelConfig, updateModelConfig, type UpdateModelConfigInput } from "./model-service";

const draft: UpdateModelConfigInput = {
    expected_revision: 4,
    public_key: "public-image",
    display_name: "Public image",
    upstream_model_id: "upstream-image",
    status: "draft",
    sort_order: 2,
    operations: [{ capability: "image", operation: "edit", enabled: true, mode: "override", adapter: "generations", config: { image_field: "image" } }],
    pricing_overrides: [{ capability: "image", credits_per_unit: 3, unit_type: "per_image", pricing_mode: "per_unit", pricing_rule: "" }],
};

describe("model service API", () => {
    afterEach(() => jest.restoreAllMocks());

    it("uses the normalized model-service channel and model endpoints", async () => {
        const get = jest.spyOn(apiClient, "get").mockResolvedValue({ data: { data: { channels: [], models: [] } } });
        await listModelServiceChannels();
        await listModelConfigs({ channelId: 7, capability: "image", status: "draft", search: "public", includeArchived: true });

        expect(get).toHaveBeenNthCalledWith(1, "/admin/model-service/channels");
        expect(get).toHaveBeenNthCalledWith(2, "/admin/model-service/models", {
            params: { channel_id: 7, capability: "image", status: "draft", search: "public", include_archived: true },
        });
    });

    it("sends one atomic model draft to save and test endpoints", async () => {
        const put = jest.spyOn(apiClient, "put").mockResolvedValue({ data: { data: draft } });
        const post = jest.spyOn(apiClient, "post").mockResolvedValue({ data: { data: { success: true } } });

        await updateModelConfig(31, draft);
        await testModelConfig(31, { capability: "image", operation: "edit", prompt: "test", reference_count: 1, draft });

        expect(put).toHaveBeenCalledWith("/admin/model-service/models/31", draft);
        expect(post).toHaveBeenCalledWith("/admin/model-service/models/31/test", { capability: "image", operation: "edit", prompt: "test", reference_count: 1, draft });
    });

    it("saves default pricing by public catalog model", async () => {
        const put = jest.spyOn(apiClient, "put").mockResolvedValue({ data: { data: { saved: true } } });
        const input = { capability: "video" as const, credits_per_unit: 6, unit_type: "per_video" as const, pricing_mode: "per_unit" as const, pricing_rule: "" };

        await saveDefaultModelPricing(12, input);

        expect(put).toHaveBeenCalledWith("/admin/model-service/pricing/defaults/12", input);
    });
});
