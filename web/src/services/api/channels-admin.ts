import apiClient from "./client";
import type { ChannelInfo } from "./channel";

/**
 * SuperAdmin-only channel representation with sync metadata and base URL.
 * Never persists or logs the raw API key.
 */
export type ChannelAdminInfo = ChannelInfo & {
    base_url: string;
    has_key: boolean;
};

/**
 * Create channel request input.
 * API key is write-only and never returned in responses.
 */
export type SaveChannelInput = {
    name: string;
    base_url: string;
    api_key: string;
    enabled?: boolean;
    new_api_channel_id?: number | null;
    metrics_base_url?: string | null;
};

/**
 * Update channel request input.
 * Omit api_key to preserve the existing encrypted key.
 */
export type UpdateChannelInput = Partial<SaveChannelInput>;

/**
 * Response wrapper for GET /admin/channels
 */
type AdminChannelsResponse = {
    channels: ChannelAdminInfo[];
};

/**
 * SuperAdmin: Create a new global channel.
 * API key is sent only to this endpoint and never stored in client state.
 */
export async function createChannel(input: SaveChannelInput): Promise<ChannelAdminInfo> {
    const res = await apiClient.post("/admin/channels", input);
    return res.data.data;
}

/**
 * SuperAdmin: Get all channels including disabled ones.
 * Omits raw API keys; shows sync status and has_key flag.
 */
export async function listAllChannels(): Promise<ChannelAdminInfo[]> {
    const res = await apiClient.get("/admin/channels");
    return res.data.data.channels;
}

/**
 * SuperAdmin: Update a channel.
 * Omit api_key in the input to preserve the existing encrypted key.
 * Cannot be used to update models; use channel-models-admin endpoints instead.
 */
export async function updateChannel(channelId: number, input: UpdateChannelInput): Promise<ChannelAdminInfo> {
    const res = await apiClient.put(`/admin/channels/${channelId}`, input);
    return res.data.data;
}

/**
 * SuperAdmin: Disable a channel.
 * Disabled channels do not appear in authenticated reads but remain in logs and SuperAdmin views.
 */
export async function disableChannel(channelId: number): Promise<void> {
    await apiClient.post(`/admin/channels/${channelId}/disable`);
}
