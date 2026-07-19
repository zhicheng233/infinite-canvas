import apiClient from "./client";

/**
 * Metrics configuration for new-api perf-metrics adapter.
 * SuperAdmin only; base URL is never persisted client-side.
 */
export type MetricsConfig = {
    metrics_base_url: string;
};

/**
 * SuperAdmin: Get the current metrics configuration.
 * Base URL is visible for debugging; never send to unauthorized clients.
 */
export async function getMetricsConfig(): Promise<MetricsConfig> {
    const res = await apiClient.get("/admin/metrics-config");
    return res.data.data;
}

/**
 * SuperAdmin: Update the metrics base URL.
 * Base URL must be valid and not expose SSRF vectors.
 */
export async function updateMetricsConfig(input: { metrics_base_url?: string }): Promise<MetricsConfig> {
    const res = await apiClient.post("/admin/metrics-config", input);
    return res.data.data;
}
