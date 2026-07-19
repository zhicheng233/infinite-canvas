import apiClient from "./client";

type ApiResult<T> = { code: number; data: T; msg: string };

type GenerationRecordDTO<T> = {
    record_id: string;
    type: "image" | "video";
    status: string;
    payload: T;
    created_at: string;
    updated_at: string;
};

export async function saveGenerationRecord<T extends { id: string; status?: string }>(type: "image" | "video", record: T) {
    await apiClient.post("/generation-records/save", {
        record_id: record.id,
        type,
        status: record.status || "",
        payload: record,
    });
}

export async function listGenerationRecords<T>(type: "image" | "video"): Promise<T[]> {
    const res = await apiClient.get<ApiResult<GenerationRecordDTO<T>[]>>("/generation-records", { params: { type } });
    return (res.data.data || []).map((item) => item.payload);
}

export async function deleteGenerationRecord(type: "image" | "video", id: string) {
    await apiClient.delete(`/generation-records/${id}`, { params: { type } });
}

export async function deleteGenerationRecords(type: "image" | "video", ids: string[]) {
    await apiClient.post("/generation-records/delete-batch", { type, ids });
}
