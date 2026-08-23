import type { CanvasConnectionWire, CanvasImageRole, CanvasSnapshotWire } from "./schemas.js";

export type Position = { x: number; y: number };
export type Viewport = { x: number; y: number; k: number };
export type CanvasNodeType = "image" | "text" | "config" | "video" | "audio";
export type { CanvasImageRole };
export type CanvasNode = { id: string; type: CanvasNodeType; title?: string; position: Position; width: number; height: number; metadata?: Record<string, unknown> };
export type CanvasConnection = CanvasConnectionWire;
export type CanvasSnapshot = CanvasSnapshotWire;
export type AgentEmit = (type: string, payload: unknown) => void;
export type AgentAttachment = { name?: string; type?: string; dataUrl?: string };
