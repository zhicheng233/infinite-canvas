import apiClient from "./client";

export type WebhookConfig = {
    id?: number;
    tenant_id?: number;
    platform: string;
    webhook_url: string;
    enabled: boolean;
    template_down?: string;
    template_up?: string;
    interval_seconds?: number;
    cooldown_minutes?: number;
};

export type WebhookLogItem = {
    id: number;
    tenant_id?: number;
    platform: string;
    model_name: string;
    status: string;
    message: string;
    success: boolean;
    response_body: string;
    cooldown_skipped: boolean;
    created_at: string;
};

export type PollerStatus = {
    running: boolean;
    interval_seconds: number;
};

export type TestSendInput = {
    platform: string;
    message: string;
};

export type TestSendResult = {
    success: boolean;
    error?: string;
};

export async function listWebhookConfigs() {
    const res = await apiClient.get("/admin/webhook/config");
    return res.data.data as WebhookConfig[];
}

export async function saveWebhookConfig(input: Partial<WebhookConfig>) {
    const res = await apiClient.put("/admin/webhook/config", input);
    return res.data.data as WebhookConfig;
}

export async function testWebhookSend(input: TestSendInput) {
    const res = await apiClient.post("/admin/webhook/test", input);
    return res.data.data as TestSendResult;
}

export async function listWebhookLogs(limit?: number) {
    const res = await apiClient.get("/admin/webhook/logs", { params: { limit } });
    return res.data.data as WebhookLogItem[];
}

export async function startPoller() {
    const res = await apiClient.post("/admin/webhook/poller/start");
    return res.data.data as { started: boolean };
}

export async function stopPoller() {
    const res = await apiClient.post("/admin/webhook/poller/stop");
    return res.data.data as { stopped: boolean };
}

export async function getPollerStatus() {
    const res = await apiClient.get("/admin/webhook/poller/status");
    return res.data.data as PollerStatus;
}
