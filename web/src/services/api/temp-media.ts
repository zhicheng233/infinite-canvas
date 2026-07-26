import apiClient from "./client";

export type TempMediaUploadResult = {
    url: string;
    filename: string;
    expires_at: string;
};

export async function uploadTempMedia(file: File): Promise<TempMediaUploadResult> {
    const body = new FormData();
    body.append("file", file);
    const res = await apiClient.post("/media/tmp", body);
    return res.data.data as TempMediaUploadResult;
}

export const uploadTempImage = uploadTempMedia;
