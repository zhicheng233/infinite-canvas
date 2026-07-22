import apiClient from "./client";

/**
 * A merge group that auto-categorizes incoming channels into named groups
 * based on a pattern against the upstream channel name.
 */
export type MergeGroup = {
    id: number;
    channel_id: number;
    group_name: string;
    pattern: string;
    enabled: boolean;
    created_at: string;
    updated_at: string;
};

/**
 * Input for creating a new merge group.
 */
export type CreateMergeGroupInput = {
    group_name: string;
    pattern: string;
};

type MergeGroupListResponse = {
    groups: MergeGroup[];
};

/**
 * SuperAdmin: List all merge groups for a channel.
 */
export async function listMergeGroups(channelId: number): Promise<MergeGroup[]> {
    const res = await apiClient.get(`/admin/channels/${channelId}/merge-groups`);
    return res.data.data.groups;
}

/**
 * SuperAdmin: Create a new merge group.
 */
export async function createMergeGroup(
    channelId: number,
    input: CreateMergeGroupInput,
): Promise<MergeGroup> {
    const res = await apiClient.post(`/admin/channels/${channelId}/merge-groups`, input);
    return res.data.data;
}

/**
 * SuperAdmin: Delete a merge group.
 */
export async function deleteMergeGroup(channelId: number, groupId: number): Promise<void> {
    await apiClient.delete(`/admin/channels/${channelId}/merge-groups/${groupId}`);
}

/**
 * SuperAdmin: Auto-create merge groups based on existing upstream channels.
 */
export async function autoCreateMergeGroups(channelId: number): Promise<MergeGroup[]> {
    const res = await apiClient.post(`/admin/channels/${channelId}/merge-groups/auto`);
    return res.data.data.groups;
}
