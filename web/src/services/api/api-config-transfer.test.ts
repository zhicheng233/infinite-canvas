import { afterEach, describe, expect, it, jest } from "bun:test";

import apiClient from "./client";
import { exportApiConfig, importApiConfig, previewApiConfigImport, readApiConfigTransferFile, type ApiConfigTransferEnvelope } from "./api-config-transfer";

const envelope: ApiConfigTransferEnvelope = {
    format: "infinite-canvas-model-api-config",
    version: 1,
    cipher: "aes-256-gcm",
    kdf: { name: "argon2id", time: 3, memory_kib: 65536, parallelism: 1 },
    salt: "salt",
    nonce: "nonce",
    ciphertext: "ciphertext",
};

describe("API config transfer service", () => {
    afterEach(() => jest.restoreAllMocks());

    it("calls export, preview, and import endpoints with passwords in request bodies", async () => {
        const post = jest.spyOn(apiClient, "post").mockResolvedValue({ data: { data: { envelope, stats: {}, conflicts: [], applied: false } } });

        await exportApiConfig("password-123");
        await previewApiConfigImport("password-123", envelope);
        await importApiConfig("password-123", envelope);

        expect(post).toHaveBeenNthCalledWith(1, "/admin/api-config/export", { password: "password-123" });
        expect(post).toHaveBeenNthCalledWith(2, "/admin/api-config/import/preview", { password: "password-123", envelope });
        expect(post).toHaveBeenNthCalledWith(3, "/admin/api-config/import", { password: "password-123", envelope });
    });

    it("reads a valid encrypted JSON envelope", async () => {
        const file = new File([JSON.stringify(envelope)], "config.json", { type: "application/json" });
        await expect(readApiConfigTransferFile(file)).resolves.toEqual(envelope);
    });

    it("rejects invalid and oversized files before sending them", async () => {
        await expect(readApiConfigTransferFile(new File(["not-json"], "bad.json"))).rejects.toThrow("不是有效的 JSON");
        await expect(readApiConfigTransferFile(new File([JSON.stringify({ format: "other" })], "bad.json"))).rejects.toThrow("格式无效");
        await expect(readApiConfigTransferFile(new File([new Uint8Array(16 * 1024 * 1024 + 1)], "large.json"))).rejects.toThrow("不能超过 16 MiB");
    });
});
