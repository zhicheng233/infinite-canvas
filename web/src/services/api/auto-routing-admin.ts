import apiClient from "./client";

export type AutoRoutingMember = {
    id: number;
    channel_model_id: number;
    channel_id: number;
    channel_name: string;
    model_name: string;
    priority: number;
    enabled: boolean;
    contract_valid: boolean;
    unavailable_reason?: string;
    success_rate: number;
    sample_count: number;
    p95_latency_ms: number;
    circuit_status: "closed" | "open" | "half_open";
};

export type AutoRoutingPool = {
    id: number;
    model: string;
    capability: "image" | "video" | "text" | "audio";
    contract_key: string;
    enabled: boolean;
    max_attempts: number;
    members: AutoRoutingMember[];
};

export type AutoRoutingSuggestion = Pick<AutoRoutingPool, "model" | "capability" | "contract_key"> & { members: AutoRoutingMember[] };

export async function listAutoRoutingPools() {
    const res = await apiClient.get("/admin/model-service/routing/pools");
    return res.data.data.pools as AutoRoutingPool[];
}

export async function listAutoRoutingSuggestions() {
    const res = await apiClient.get("/admin/model-service/routing/suggestions");
    return res.data.data.suggestions as AutoRoutingSuggestion[];
}

export async function createAutoRoutingPool(input: { model: string; capability: string; contract_key: string; channel_model_ids: number[] }) {
    const res = await apiClient.post("/admin/model-service/routing/pools", input);
    return res.data.data as AutoRoutingPool;
}

export async function updateAutoRoutingPool(id: number, input: { enabled?: boolean; max_attempts?: number; contract_key?: string; channel_model_ids?: number[] }) {
    const res = await apiClient.put(`/admin/model-service/routing/pools/${id}`, input);
    return res.data.data as AutoRoutingPool;
}

export async function updateAutoRoutingMember(poolId: number, memberId: number, input: { enabled?: boolean; priority?: number }) {
    const res = await apiClient.put(`/admin/model-service/routing/pools/${poolId}/members/${memberId}`, input);
    return res.data.data as AutoRoutingPool;
}

export async function deleteAutoRoutingPool(id: number) {
    await apiClient.delete(`/admin/model-service/routing/pools/${id}`);
}
