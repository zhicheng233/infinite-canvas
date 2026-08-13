import apiClient from "./client";
import { normalizeCustomVideoConfig, type CustomVideoConfig } from "@/lib/custom-video-config";
import type { ChannelModelInfo } from "./channel";

/**
 * Sync result response wrapper.
 */
export type ChannelModelSyncResult = {
    synced: boolean;
};

/**
 * Update channel model enabled state and metadata.
 * Matches backend UpdateChannelModelInput: includes capabilities and sort_order.
 */
export type UpdateChannelModelInput = {
    enabled?: boolean;
    image_generate_route?: string;
    image_edit_route?: string;
    video_route?: string;
    video_durations?: number[];
    video_customizable?: boolean;
    video_custom_config?: CustomVideoConfig | null;
    sort_order?: number;
    capabilities?: string[];
};

export function normalizeChannelModelUpdateInput(input: UpdateChannelModelInput): UpdateChannelModelInput {
    const videoRoute = input.video_route?.trim();
    const { video_custom_config: _customConfig, ...metadata } = input;
    if (videoRoute === undefined) return metadata;
    if (videoRoute !== "custom") return { ...metadata, video_route: videoRoute, video_custom_config: null };
    const videoCustomConfig = normalizeCustomVideoConfig(input.video_custom_config);
    if (!videoCustomConfig) throw new Error("video_custom_config 无效");
    return { ...metadata, video_route: videoRoute, video_custom_config: videoCustomConfig };
}

/**
 * SuperAdmin: Synchronize channel models from upstream /models endpoint.
 * Preserves existing enabled flags and metadata where possible.
 * Failed sync returns an error but does not clear the existing catalog.
 */
export async function syncChannelModels(channelId: number): Promise<ChannelModelSyncResult> {
    const res = await apiClient.post(`/admin/channels/${channelId}/models/sync`, {});
    return res.data.data as ChannelModelSyncResult;
}

/**
 * SuperAdmin: Get all models for a channel (including disabled).
 */
export async function listChannelModelsAdmin(channelId: number): Promise<ChannelModelInfo[]> {
    const res = await apiClient.get(`/admin/channels/${channelId}/models`);
    return res.data.data.models;
}

/**
 * SuperAdmin: Update a single channel model's metadata and enabled state.
 */
export async function updateChannelModel(channelId: number, modelId: number, input: UpdateChannelModelInput): Promise<ChannelModelInfo> {
    const res = await apiClient.put(`/admin/channels/${channelId}/models/${modelId}`, normalizeChannelModelUpdateInput(input));
    return res.data.data;
}

/**
 * SuperAdmin: Disable a specific model in a channel.
 * Shorthand for updateChannelModel with enabled: false.
 */
export async function disableChannelModel(channelId: number, modelId: number): Promise<ChannelModelInfo> {
    return updateChannelModel(channelId, modelId, { enabled: false });
}

/**
 * SuperAdmin: Enable a specific model in a channel.
 * Shorthand for updateChannelModel with enabled: true.
 */
export async function enableChannelModel(channelId: number, modelId: number): Promise<ChannelModelInfo> {
    return updateChannelModel(channelId, modelId, { enabled: true });
}
