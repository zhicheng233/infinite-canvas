import type { CustomVideoConfig } from "@/lib/custom-video-config";
import apiClient from "./client";

export type VideoConfigPreset = {
    readonly id: number;
    readonly name: string;
    readonly config: CustomVideoConfig;
    readonly created_at: string;
    readonly updated_at: string;
};

export type CreateVideoConfigPresetInput = {
    readonly name: string;
    readonly config: CustomVideoConfig;
};

export type DeleteVideoConfigPresetResult = {
    readonly deleted: true;
};

export async function listVideoConfigPresets(): Promise<VideoConfigPreset[]> {
    const response = await apiClient.get("/api-config/video-presets");
    return response.data.data;
}

export async function createVideoConfigPreset(input: CreateVideoConfigPresetInput): Promise<VideoConfigPreset> {
    const response = await apiClient.post("/api-config/video-presets", input);
    return response.data.data;
}

export async function deleteVideoConfigPreset(presetId: number): Promise<DeleteVideoConfigPresetResult> {
    const response = await apiClient.delete(`/api-config/video-presets/${presetId}`);
    return response.data.data;
}
