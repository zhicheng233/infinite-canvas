import apiClient from "./client";
import type { ApiModelTestResult } from "./api-config";

export type ModelCapability = "image" | "video" | "text" | "audio";
export type ModelStatus = "draft" | "active" | "disabled";
export type ProtocolMode = "inherit" | "override";

export type ProtocolDefault = {
    capability: ModelCapability;
    operation: string;
    adapter: string;
    config: Record<string, unknown>;
};

export type ModelServiceChannel = {
    id: number;
    name: string;
    base_url: string;
    has_key: boolean;
    enabled: boolean;
    archived: boolean;
    video_api_standard: "default" | "binghuo";
    new_api_channel_id?: number | null;
    metrics_base_url?: string | null;
    remark?: string;
    sync_status: string;
    sync_error?: string;
    synced_at?: string | null;
    config_revision: number;
    protocol_defaults: ProtocolDefault[];
    model_count: number;
    ready_model_count: number;
};

export type SaveModelServiceChannelInput = {
    expected_revision?: number;
    name: string;
    base_url: string;
    api_key?: string;
    enabled: boolean;
    video_api_standard: "default" | "binghuo";
    new_api_channel_id?: number | null;
    metrics_base_url?: string | null;
    remark?: string;
};

export type EffectiveProtocol = {
    source: "channel" | "model" | "";
    adapter: string;
    config: Record<string, unknown>;
    config_version: number;
    contract_key: string;
};

export type ModelOperation = {
    capability: ModelCapability;
    operation: string;
    enabled: boolean;
    mode: ProtocolMode;
    adapter: string;
    config: Record<string, unknown>;
    effective: EffectiveProtocol;
};

export type ModelPricingRule = {
    id: number;
    capability: ModelCapability;
    scope: "default" | "implementation";
    scope_id: number;
    credits_per_unit: number;
    unit_type: "per_image" | "per_video" | "per_video_second" | "per_token";
    pricing_mode: "per_unit" | "video_dynamic";
    pricing_rule: string;
    config_revision: number;
    effective_source?: "default" | "implementation";
};

export type ModelReadinessIssue = {
    code: string;
    capability?: ModelCapability;
    operation?: string;
    message: string;
};

export type ModelConfig = {
    id: number;
    channel_id: number;
    channel_name: string;
    channel_remark?: string;
    catalog_model_id: number;
    public_key: string;
    display_name: string;
    upstream_model_id: string;
    status: ModelStatus;
    discovery_status: "present" | "missing";
    last_discovered_at?: string;
    config_revision: number;
    legacy_unreviewed: boolean;
    archived: boolean;
    sort_order: number;
    operations: ModelOperation[];
    pricing: ModelPricingRule[];
    readiness_issues: ModelReadinessIssue[];
    ready: boolean;
};

export type SaveModelOperationInput = Omit<ModelOperation, "effective">;
export type SaveModelPricingInput = Pick<ModelPricingRule, "capability" | "credits_per_unit" | "unit_type" | "pricing_mode" | "pricing_rule">;

export type UpdateModelConfigInput = {
    expected_revision: number;
    public_key: string;
    display_name: string;
    upstream_model_id: string;
    status: ModelStatus;
    sort_order: number;
    operations: SaveModelOperationInput[];
    pricing_overrides: SaveModelPricingInput[];
};

export type ModelTestDraftInput = {
    capability: ModelCapability;
    operation: string;
    prompt?: string;
    size?: string;
    aspect_ratio?: string;
    seconds?: number;
    reference_count?: number;
    draft?: UpdateModelConfigInput;
};

export type ChannelSyncReport = {
    discovered: number;
    created: number;
    restored: number;
    missing: number;
    unchanged: number;
    model_ids: number[];
};

export async function listModelServiceChannels() {
    const response = await apiClient.get("/admin/model-service/channels");
    return response.data.data.channels as ModelServiceChannel[];
}

export async function createModelServiceChannel(input: SaveModelServiceChannelInput) {
    const response = await apiClient.post("/admin/model-service/channels", input);
    return response.data.data;
}

export async function updateModelServiceChannel(id: number, input: SaveModelServiceChannelInput) {
    const response = await apiClient.put(`/admin/model-service/channels/${id}`, input);
    return response.data.data;
}

export async function syncModelServiceChannel(id: number) {
    const response = await apiClient.post(`/admin/model-service/channels/${id}/sync`);
    return response.data.data as ChannelSyncReport;
}

export async function setModelServiceChannelArchived(id: number, archived: boolean) {
    const response = await apiClient.post(`/admin/model-service/channels/${id}/${archived ? "archive" : "restore"}`);
    return response.data.data as { archived: boolean };
}

export async function previewChannelProtocolDefaults(id: number, expectedRevision: number, defaults: ProtocolDefault[]) {
    const response = await apiClient.put(`/admin/model-service/channels/${id}/protocol-defaults/preview`, { expected_revision: expectedRevision, defaults });
    return response.data.data as { affected_model_ids: number[]; issues: ModelReadinessIssue[] };
}

export async function saveChannelProtocolDefaults(id: number, expectedRevision: number, defaults: ProtocolDefault[]) {
    const response = await apiClient.put(`/admin/model-service/channels/${id}/protocol-defaults`, { expected_revision: expectedRevision, defaults });
    return response.data.data as { saved: boolean };
}

export async function listModelConfigs(params: { channelId?: number; capability?: string; status?: string; search?: string; includeArchived?: boolean } = {}) {
    const response = await apiClient.get("/admin/model-service/models", {
        params: {
            channel_id: params.channelId || undefined,
            capability: params.capability || undefined,
            status: params.status || undefined,
            search: params.search || undefined,
            include_archived: params.includeArchived || undefined,
        },
    });
    return response.data.data.models as ModelConfig[];
}

export async function getModelConfig(id: number) {
    const response = await apiClient.get(`/admin/model-service/models/${id}`);
    return response.data.data as ModelConfig;
}

export async function updateModelConfig(id: number, input: UpdateModelConfigInput) {
    const response = await apiClient.put(`/admin/model-service/models/${id}`, input);
    return response.data.data as ModelConfig;
}

export async function testModelConfig(id: number, input: ModelTestDraftInput) {
    const response = await apiClient.post(`/admin/model-service/models/${id}/test`, input);
    return response.data.data as ApiModelTestResult;
}

export async function setModelConfigArchived(id: number, archived: boolean) {
    const response = await apiClient.post(`/admin/model-service/models/${id}/${archived ? "archive" : "restore"}`);
    return response.data.data as { archived: boolean };
}

export async function saveDefaultModelPricing(catalogModelId: number, input: SaveModelPricingInput) {
    const response = await apiClient.put(`/admin/model-service/pricing/defaults/${catalogModelId}`, input);
    return response.data.data as { saved: boolean };
}
