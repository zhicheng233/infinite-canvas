import apiClient from "./client";

export type ApiConfigTransferEnvelope = {
    format: string;
    version: number;
    cipher: string;
    kdf: {
        name: string;
        time: number;
        memory_kib: number;
        parallelism: number;
    };
    salt: string;
    nonce: string;
    ciphertext: string;
};

export type ApiConfigTransferChangeStats = {
    create: number;
    update: number;
    skip: number;
};

export type ApiConfigTransferStats = {
    channels: ApiConfigTransferChangeStats;
    models: ApiConfigTransferChangeStats;
    pricing: ApiConfigTransferChangeStats;
    merge_groups: ApiConfigTransferChangeStats;
    video_config_presets: ApiConfigTransferChangeStats;
    auto_routing_pools: ApiConfigTransferChangeStats;
};

export type ApiConfigTransferConflict = {
    resource: string;
    identifier: string;
    reason: string;
};

export type ApiConfigTransferResult = {
    stats: ApiConfigTransferStats;
    conflicts: ApiConfigTransferConflict[];
    applied: boolean;
};

export type ApiConfigTransferExportResult = {
    file_name: string;
    envelope: ApiConfigTransferEnvelope;
    summary: ApiConfigTransferStats;
    warnings: ApiConfigTransferConflict[];
};

const MAX_CONFIG_FILE_BYTES = 16 * 1024 * 1024;

export async function exportApiConfig(password: string): Promise<ApiConfigTransferExportResult> {
    const response = await apiClient.post("/admin/api-config/export", { password });
    return response.data.data;
}

export async function previewApiConfigImport(password: string, envelope: ApiConfigTransferEnvelope): Promise<ApiConfigTransferResult> {
    const response = await apiClient.post("/admin/api-config/import/preview", { password, envelope });
    return response.data.data;
}

export async function importApiConfig(password: string, envelope: ApiConfigTransferEnvelope): Promise<ApiConfigTransferResult> {
    const response = await apiClient.post("/admin/api-config/import", { password, envelope });
    return response.data.data;
}

export async function readApiConfigTransferFile(file: File): Promise<ApiConfigTransferEnvelope> {
    if (file.size > MAX_CONFIG_FILE_BYTES) throw new Error("配置文件不能超过 16 MiB");
    let value: unknown;
    try {
        value = JSON.parse(await file.text());
    } catch {
        throw new Error("配置文件不是有效的 JSON 文件");
    }
    if (!isEnvelope(value)) throw new Error("配置文件格式无效");
    return value;
}

function isEnvelope(value: unknown): value is ApiConfigTransferEnvelope {
    if (!value || typeof value !== "object" || Array.isArray(value)) return false;
    const envelope = value as Record<string, unknown>;
    const kdf = envelope.kdf;
    return (
        envelope.format === "infinite-canvas-model-api-config" &&
        typeof envelope.version === "number" &&
        typeof envelope.cipher === "string" &&
        typeof envelope.salt === "string" &&
        typeof envelope.nonce === "string" &&
        typeof envelope.ciphertext === "string" &&
        Boolean(kdf) &&
        typeof kdf === "object" &&
        !Array.isArray(kdf)
    );
}
