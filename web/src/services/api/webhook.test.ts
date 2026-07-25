import apiClient from "@/services/api/client";
import { afterEach, describe, expect, it, jest } from "bun:test";

import {
    listWebhookConfigs,
    listWebhookLogs,
    saveWebhookConfig,
    testWebhookSend,
} from "./webhook";

/** Wraps data in the axios response envelope so apiClient returns `{ data: { data } }`. */
function mockResponse(data: unknown) {
    return { data: { data } };
}

describe("webhook API client", () => {
    afterEach(() => {
        jest.restoreAllMocks();
    });

    // ─── framework ────────────────────────────────────────────────────────────

    it("test framework works", () => {
        expect(1 + 1).toBe(2);
    });

    // ─── listWebhookConfigs ───────────────────────────────────────────────────

    it("listWebhookConfigs calls GET /admin/webhook/config and returns array", async () => {
        const spy = jest.spyOn(apiClient, "get").mockResolvedValue(mockResponse([]));
        const result = await listWebhookConfigs();
        expect(spy).toHaveBeenCalledWith("/admin/webhook/config");
        expect(Array.isArray(result)).toBe(true);
        expect(result).toEqual([]);
    });

    it("listWebhookConfigs returns populated config items", async () => {
        const configs = [
            { id: 1, platform: "feishu", webhook_url: "https://example.com/feishu", enabled: true },
            { id: 2, platform: "dtalk", webhook_url: "https://example.com/dingtalk", enabled: false },
        ];
        const spy = jest.spyOn(apiClient, "get").mockResolvedValue(mockResponse(configs));
        const result = await listWebhookConfigs();
        expect(spy).toHaveBeenCalledWith("/admin/webhook/config");
        expect(result).toHaveLength(2);
        expect(result[0].platform).toBe("feishu");
        expect(result[1].platform).toBe("dtalk");
    });

    it("listWebhookConfigs rejects when API call fails", async () => {
        jest.spyOn(apiClient, "get").mockRejectedValue(new Error("server error"));
        await expect(listWebhookConfigs()).rejects.toThrow("server error");
    });

    // ─── saveWebhookConfig ────────────────────────────────────────────────────

    it("saveWebhookConfig calls PUT /admin/webhook/config with body", async () => {
        const input = { platform: "feishu", webhook_url: "https://example.com", enabled: true };
        const spy = jest.spyOn(apiClient, "put").mockResolvedValue(mockResponse(input));
        const result = await saveWebhookConfig(input);
        expect(spy).toHaveBeenCalledWith("/admin/webhook/config", input);
        expect(result).toEqual(input);
    });

    it("saveWebhookConfig accepts partial update", async () => {
        const partial = { platform: "feishu", cooldown_minutes: 5 };
        const spy = jest.spyOn(apiClient, "put").mockResolvedValue(mockResponse(partial));
        await saveWebhookConfig(partial);
        expect(spy).toHaveBeenCalledWith("/admin/webhook/config", partial);
    });

    it("saveWebhookConfig rejects when API call fails", async () => {
        jest.spyOn(apiClient, "put").mockRejectedValue(new Error("validation failed"));
        await expect(saveWebhookConfig({ platform: "feishu", webhook_url: "bad", enabled: true })).rejects.toThrow("validation failed");
    });

    // ─── listWebhookLogs ──────────────────────────────────────────────────────

    it("listWebhookLogs calls GET /admin/webhook/logs with limit param", async () => {
        const spy = jest.spyOn(apiClient, "get").mockResolvedValue(mockResponse([]));
        await listWebhookLogs(10);
        expect(spy).toHaveBeenCalledWith("/admin/webhook/logs", { params: { limit: 10 } });
    });

    it("listWebhookLogs calls GET without limit when no arg provided", async () => {
        const spy = jest.spyOn(apiClient, "get").mockResolvedValue(mockResponse([]));
        await listWebhookLogs();
        // limit is undefined — axios strips it, but the JS call still passes it
        expect(spy).toHaveBeenCalledWith("/admin/webhook/logs", { params: { limit: undefined } });
    });

    it("listWebhookLogs returns log items matching WebhookLogItem shape", async () => {
        const logs = [
            {
                id: 1,
                platform: "feishu",
                model_name: "gpt-4",
                status: "sent",
                message: "ok",
                success: true,
                response_body: "{}",
                cooldown_skipped: false,
                created_at: "2026-01-01T00:00:00Z",
            },
        ];
        jest.spyOn(apiClient, "get").mockResolvedValue(mockResponse(logs));
        const result = await listWebhookLogs(5);
        expect(result).toHaveLength(1);
        expect(result[0].id).toBe(1);
        expect(result[0].success).toBe(true);
        expect(result[0].platform).toBe("feishu");
        expect(result[0].status).toBe("sent");
    });

    it("listWebhookLogs rejects when API call fails", async () => {
        jest.spyOn(apiClient, "get").mockRejectedValue(new Error("db error"));
        await expect(listWebhookLogs()).rejects.toThrow("db error");
    });

    // ─── testWebhookSend ──────────────────────────────────────────────────────

    it("testWebhookSend calls POST /admin/webhook/test with body", async () => {
        const spy = jest.spyOn(apiClient, "post").mockResolvedValue(mockResponse({ success: true }));
        const result = await testWebhookSend({ platform: "feishu", message: "hello" });
        expect(spy).toHaveBeenCalledWith("/admin/webhook/test", { platform: "feishu", message: "hello" });
        expect(result).toEqual({ success: true });
    });

    it("testWebhookSend returns fail result when send fails", async () => {
        jest.spyOn(apiClient, "post").mockResolvedValue(
            mockResponse({ success: false, error: "channel not configured" }),
        );
        const result = await testWebhookSend({ platform: "wecom", message: "test" });
        expect(result.success).toBe(false);
        expect(result.error).toBe("channel not configured");
    });

    it("testWebhookSend rejects when API call fails", async () => {
        jest.spyOn(apiClient, "post").mockRejectedValue(new Error("internal error"));
        await expect(testWebhookSend({ platform: "feishu", message: "x" })).rejects.toThrow("internal error");
    });

});
