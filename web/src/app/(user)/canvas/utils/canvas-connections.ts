import { customVideoMediaFeatureNames } from "@/lib/custom-video-config";
import type { CanvasConnection, CanvasImageRole } from "../types";

export const canvasImageRoles: readonly CanvasImageRole[] = customVideoMediaFeatureNames.filter((role): role is CanvasImageRole => role !== "input_video");

export function normalizeCanvasImageRole(value: unknown): CanvasImageRole | undefined {
    return canvasImageRoles.find((role) => role === value);
}

export function sameCanvasConnectionIdentity(left: Pick<CanvasConnection, "fromNodeId" | "toNodeId" | "targetImageRole">, right: Pick<CanvasConnection, "fromNodeId" | "toNodeId" | "targetImageRole">) {
    return left.fromNodeId === right.fromNodeId && left.toNodeId === right.toNodeId && left.targetImageRole === right.targetImageRole;
}

export function normalizeCanvasConnection(value: unknown): CanvasConnection | null {
    if (!value || typeof value !== "object" || Array.isArray(value)) return null;
    const id = "id" in value ? value.id : undefined;
    const fromNodeId = "fromNodeId" in value ? value.fromNodeId : undefined;
    const toNodeId = "toNodeId" in value ? value.toNodeId : undefined;
    if (typeof id !== "string" || !id || typeof fromNodeId !== "string" || !fromNodeId || typeof toNodeId !== "string" || !toNodeId) return null;
    if (!("targetImageRole" in value) || value.targetImageRole === undefined) return { id, fromNodeId, toNodeId };
    const targetImageRole = normalizeCanvasImageRole(value.targetImageRole);
    return targetImageRole ? { id, fromNodeId, toNodeId, targetImageRole } : null;
}

export function normalizeCanvasConnections(value: unknown): CanvasConnection[] {
    if (!Array.isArray(value)) return [];
    return value.reduce<CanvasConnection[]>((connections, item) => {
        const connection = normalizeCanvasConnection(item);
        if (!connection || connections.some((existing) => sameCanvasConnectionIdentity(existing, connection))) return connections;
        return [...connections, connection];
    }, []);
}
