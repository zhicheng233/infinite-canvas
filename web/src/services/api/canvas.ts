import apiClient from "./client";
import type { CanvasProject } from "@/app/(user)/canvas/stores/use-canvas-store";

type CanvasProjectDTO = {
  project_id: string;
  title: string;
  nodes: string;
  connections: string;
  chat_sessions: string;
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

function safeParse<T>(raw: string, fallback: T): T {
  try {
    return JSON.parse(raw) as T;
  } catch {
    return fallback;
  }
}

function dtoToProject(dto: CanvasProjectDTO): CanvasProject {
  return {
    id: dto.project_id,
    title: dto.title,
    createdAt: dto.created_at,
    updatedAt: dto.updated_at,
    nodes: safeParse(dto.nodes, []),
    connections: safeParse(dto.connections, []),
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
    title: project.title,
    nodes: JSON.stringify(project.nodes),
    connections: JSON.stringify(project.connections),
    chat_sessions: JSON.stringify(project.chatSessions),
    active_chat_id: project.activeChatId || "",
    background_mode: project.backgroundMode,
    show_image_info: project.showImageInfo,
    viewport_x: project.viewport.x,
    viewport_y: project.viewport.y,
    viewport_k: project.viewport.k,
  };
}

export async function saveCanvas(project: CanvasProject): Promise<void> {
  const payload = projectToSavePayload(project);
  await apiClient.post("/canvas/save", payload);
}

export async function loadCanvas(id: string): Promise<CanvasProject | null> {
  const res = await apiClient.get<ApiResult<CanvasProjectDTO | null>>(`/canvas/${id}`);
  const dto = res.data.data;
  return dto ? dtoToProject(dto) : null;
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
