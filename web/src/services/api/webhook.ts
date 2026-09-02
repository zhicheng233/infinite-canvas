import apiClient from "./client";

export type WebhookConfig = {
    id?: number;
    tenant_id?: number;
    platform: string;
    webhook_url: string;
    enabled: boolean;
    cooldown_minutes?: number;
};

export type WebhookLogItem = {
    id: number;
    tenant_id?: number;
    platform: string;
    channel_id?: number;
    channel_name?: string;
    model_name: string;
    status: string;
    message: string;
    success: boolean;
    response_body: string;
    cooldown_skipped: boolean;
    created_at: string;
};

export type TestSendInput = {
    platform: string;
    message: string;
};

export type TestSendResult = {
    success: boolean;
    error?: string;
};

export type SaveWebhookConfigInput = Pick<WebhookConfig, "platform"> & Partial<Omit<WebhookConfig, "platform">>;

export async function listWebhookConfigs() {
    const res = await apiClient.get("/admin/notifications/webhooks");
    return res.data.data as WebhookConfig[];
}

export async function saveWebhookConfig(input: SaveWebhookConfigInput) {
    const res = await apiClient.put("/admin/notifications/webhooks", input);
    return res.data.data as WebhookConfig;
}

export async function testWebhookSend(input: TestSendInput) {
    const res = await apiClient.post("/admin/notifications/webhooks/test", input);
    return res.data.data as TestSendResult;
}

export async function listWebhookLogs(limit?: number) {
    const res = await apiClient.get("/admin/notifications/webhook-logs", { params: { limit } });
    return res.data.data as WebhookLogItem[];
}
