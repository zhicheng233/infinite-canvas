import axios from "axios";
import apiClient from "./client";
import type { CanvasProject } from "@/app/(user)/canvas/stores/use-canvas-store";
import { normalizeCanvasConnections } from "@/app/(user)/canvas/utils/canvas-connections";

type CanvasProjectDTO = {
    project_id: string;
    schema_version: number;
    revision: number;
    title: string;
    nodes: unknown;
    connections: unknown;
    chat_sessions: unknown;
    active_chat_id: string;
    background_mode: string;
    show_image_info: boolean;
    viewport_x: number;
    viewport_y: number;
    viewport_k: number;
    created_at: string;
    updated_at: string;
};

type ApiResult<T> = { code: number; data: T; msg: string };

function safeParse<T>(raw: unknown, fallback: T): T {
    try {
        if (typeof raw !== "string") {
            return (raw ?? fallback) as T;
        }
        const parsed = JSON.parse(raw) as unknown;
        if (typeof parsed === "string") {
            return JSON.parse(parsed) as T;
        }
        return parsed as T;
    } catch {
        return fallback;
    }
}

function dtoToProject(dto: CanvasProjectDTO): CanvasProject {
    return {
        id: dto.project_id,
        schemaVersion: dto.schema_version || 1,
        revision: dto.revision || 0,
        title: dto.title,
        createdAt: dto.created_at,
        updatedAt: dto.updated_at,
        nodes: safeParse(dto.nodes, []),
        connections: normalizeCanvasConnections(safeParse(dto.connections, [])),
        chatSessions: safeParse(dto.chat_sessions, []),
        activeChatId: dto.active_chat_id || null,
        backgroundMode: (dto.background_mode as CanvasProject["backgroundMode"]) || "lines",
        showImageInfo: dto.show_image_info || false,
        viewport: {
            x: dto.viewport_x || 0,
            y: dto.viewport_y || 0,
            k: dto.viewport_k || 1,
        },
    };
}

function projectToSavePayload(project: CanvasProject) {
    return {
        id: project.id,
        schema_version: project.schemaVersion,
        revision: project.revision,
        title: project.title,
        nodes: project.nodes,
        connections: normalizeCanvasConnections(project.connections),
        chat_sessions: project.chatSessions,
        active_chat_id: project.activeChatId || "",
        background_mode: project.backgroundMode,
        show_image_info: project.showImageInfo,
        viewport_x: project.viewport.x,
        viewport_y: project.viewport.y,
        viewport_k: project.viewport.k,
    };
}

export class CanvasConflictError extends Error {
    constructor(readonly latest: CanvasProject) {
        super("画布云端版本已更新，本地内容已保留为冲突副本");
    }
}

export async function saveCanvas(project: CanvasProject, signal?: AbortSignal): Promise<CanvasProject> {
    const payload = projectToSavePayload(project);
    try {
        const response = await apiClient.post<ApiResult<CanvasProjectDTO>>("/canvas/save", payload, { signal });
        return dtoToProject(response.data.data);
    } catch (error) {
        if (axios.isAxiosError<ApiResult<CanvasProjectDTO>>(error) && error.response?.status === 409 && error.response.data.data) {
            throw new CanvasConflictError(dtoToProject(error.response.data.data));
        }
        throw error;
    }
}

export async function loadCanvas(id: string): Promise<CanvasProject | null> {
    const res = await apiClient.get<ApiResult<CanvasProjectDTO | null>>(`/canvas/${id}`);
    const dto = res.data.data;
    return dto ? dtoToProject(dto) : null;
}

export async function loadCanvasList(): Promise<CanvasProject[]> {
    return listCanvases();
}

export async function listCanvases(): Promise<CanvasProject[]> {
    const res = await apiClient.get<ApiResult<CanvasProjectDTO[]>>("/canvas");
    return (res.data.data || []).map(dtoToProject);
}

export async function deleteCanvas(id: string): Promise<void> {
    await apiClient.delete(`/canvas/${id}`);
}

export async function deleteBatchCanvas(ids: string[]): Promise<void> {
    await apiClient.post("/canvas/delete-batch", { ids });
}
