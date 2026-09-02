import { describe, expect, it, jest } from "bun:test";

import type { ModelConfig } from "@/services/api/model-service";
import { completeConfigImport } from "./api-config-transfer";
import { completeOperations, groupModelsByCatalog, groupModelsByChannel, pricingDrafts } from "./model-service-models";
import { buildPricingRows } from "./model-service-pricing";

const baseModel: ModelConfig = {
    id: 11,
    channel_id: 1,
    channel_name: "Channel A",
    catalog_model_id: 7,
    public_key: "public-image",
    display_name: "Public image",
    upstream_model_id: "upstream-a",
    status: "active",
    discovery_status: "present",
    config_revision: 3,
    legacy_unreviewed: false,
    archived: false,
    sort_order: 0,
    operations: [
        { capability: "image", operation: "generate", enabled: true, mode: "inherit", adapter: "", config: {}, effective: { source: "channel", adapter: "generations", config: {}, config_version: 1, contract_key: "contract" } },
    ],
    pricing: [{ id: 1, capability: "image", scope: "default", scope_id: 0, credits_per_unit: 2, unit_type: "per_image", pricing_mode: "per_unit", pricing_rule: "", config_revision: 1, effective_source: "default" }],
    readiness_issues: [],
    ready: true,
};

describe("model service admin domain helpers", () => {
    it("groups the same catalog by channel view and model view without losing implementations", () => {
        const second = { ...baseModel, id: 22, channel_id: 2, channel_name: "Channel B", upstream_model_id: "upstream-b" };
        expect(groupModelsByChannel([baseModel, second]).map((group) => [group.name, group.models.length])).toEqual([
            ["Channel A", 1],
            ["Channel B", 1],
        ]);
        expect(groupModelsByCatalog([baseModel, second])).toMatchObject([{ id: 7, publicKey: "public-image", models: [{ id: 11 }, { id: 22 }] }]);
    });

    it("keeps inherited pricing visible and marks only implementation prices as overrides", () => {
        const inherited = pricingDrafts(baseModel.pricing);
        expect(inherited.image).toMatchObject({ override: false, effective_source: "default", credits_per_unit: 2 });

        const overriddenModel = {
            ...baseModel,
            id: 12,
            pricing: [{ ...baseModel.pricing[0], id: 2, scope: "implementation" as const, scope_id: 12, credits_per_unit: 5, effective_source: "implementation" as const }],
        };
        const rows = buildPricingRows([baseModel, overriddenModel]);
        expect(rows).toHaveLength(1);
        expect(rows[0]).toMatchObject({ defaultPricing: { credits_per_unit: 2 }, overrideCount: 1, implementations: [{ id: 11 }, { id: 12 }] });
    });

    it("completes a sparse operation draft while preserving its effective editable values", () => {
        const operations = completeOperations(baseModel.operations);
        expect(operations).toHaveLength(5);
        expect(operations[0]).toMatchObject({ capability: "image", operation: "generate", enabled: true, mode: "inherit" });
        expect(operations.find((item) => item.capability === "video")).toMatchObject({ enabled: false, mode: "inherit", config: {} });
    });

    it("refreshes model-service data only after an import succeeds", async () => {
        const order: string[] = [];
        const importer = jest.fn(async () => {
            order.push("import");
            return { stats: { channels: emptyStats, models: emptyStats, pricing: emptyStats, merge_groups: emptyStats, video_config_presets: emptyStats, auto_routing_pools: emptyStats }, conflicts: [], applied: true };
        });
        const refresh = jest.fn(async () => {
            order.push("refresh");
        });

        await completeConfigImport(importer, refresh);

        expect(order).toEqual(["import", "refresh"]);
        expect(importer).toHaveBeenCalledTimes(1);
        expect(refresh).toHaveBeenCalledTimes(1);
    });
});

const emptyStats = { create: 0, update: 0, skip: 0 };
