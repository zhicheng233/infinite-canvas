import apiClient from "./client";

/**
 * Model metrics with success rate and counts.
 * success_rate is null when unavailable, numeric 0 for actual zero percent.
 * channel_model_id, channel_id, and model_name identify the model.
 * status values: ok, unavailable, unmapped, stale, error.
 */
export type ModelMetrics = {
    channel_model_id: number;
    channel_id: number;
    model_name: string;
    request_count: number;
    success_count: number;
    success_rate: number | null;
    status: string;
    avg_latency_ms?: number | null;
    avg_tps?: number | null;
    recent_success_rates?: number[];
};

/**
 * Channel metrics with nested model metrics.
 * success_rate is null when unavailable, numeric 0 for actual zero percent.
 * newapi_channel_id is the upstream new-api channel ID or null if unmapped.
 * status values: ok, unavailable, unmapped, stale, error.
 */
export type ChannelMetrics = {
    channel_id: number;
    channel_name: string;
    new_api_channel_id: number | null;
    request_count: number;
    success_count: number;
    success_rate: number | null;
    status: string;
    models: ModelMetrics[];
};

/**
 * Full metrics response including metadata.
 * status values: ok, unavailable, unmapped, stale, error.
 * success_rate is null when unavailable, numeric 0 for actual zero percent.
 */
export type MetricsResponse = {
    channels: ChannelMetrics[];
    hours: number;
    window: string;
    status: string;
    error?: string;
    updated_at: string;
};

/**
 * Get metrics for all channels with a specific hours window.
 * Returns the full response including channels, hours, window, status, and updated_at.
 * success_rate is null for unavailable metrics, numeric 0 for actual zero percent.
 * Hours are validated server-side; invalid values are rejected with a non-zero code.
 */
export async function getMetrics(hours: number = 24): Promise<MetricsResponse> {
    const res = await apiClient.get("/channels/metrics", {
        params: { hours },
    });
    return res.data.data;
}
