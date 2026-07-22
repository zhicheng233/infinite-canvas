import apiClient from "./client";

/**
 * Redacted channel representation for authenticated users.
 * Never includes raw API keys or sensitive credentials.
 */
export type ChannelInfo = {
    id: number;
    name: string;
    enabled: boolean;
    new_api_channel_id?: number | null;
    metrics_base_url?: string | null;
    sync_status: string;
    sync_error?: string;
    synced_at?: string | null;
};

/**
 * Channel model catalog entry with enable/disable state and route metadata.
 */
export type ChannelModelInfo = {
    id: number;
    channel_id: number;
    model_name: string;
    capabilities: string[];
    enabled: boolean;
    image_generate_route: string;
    image_edit_route: string;
    video_route: string;
    video_durations: number[];
    video_customizable: boolean;
    sort_order: number;
};

/**
 * Response wrapper for GET /channels
 */
type ChannelsResponse = {
    channels: ChannelInfo[];
};

/**
 * Response wrapper for GET /channels/:id/models
 */
type ChannelModelsResponse = {
    models: ChannelModelInfo[];
};

/**
 * Get all enabled global channels visible to authenticated users.
 * Returns redacted channel data without API keys.
 */
export async function getChannels(): Promise<ChannelInfo[]> {
    const res = await apiClient.get("/channels");
    return res.data.data.channels;
}

/**
 * Get models for a specific channel by channel ID.
 * Returns only enabled models with available pricing.
 */
export async function getChannelModels(channelId: number): Promise<ChannelModelInfo[]> {
    const res = await apiClient.get(`/channels/${channelId}/models`);
    return res.data.data.models;
}

export type AutoChannelModelRef = {
    channel_id: number;
    channel_model_id: number;
    channel_name: string;
    success_rate: number;
};

export type AutoChannelModelInfo = {
    model: string;
    channels: AutoChannelModelRef[];
};

export async function getAutoChannelModels(): Promise<AutoChannelModelInfo[]> {
    const res = await apiClient.get("/channels/auto/models");
    return res.data.data.models;
}
