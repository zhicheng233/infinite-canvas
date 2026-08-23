import type { CanvasImageRole } from "../types";

export type CanvasConnectedVideoMedia = {
    readonly nodeId: string;
    readonly title: string;
    readonly source: string;
};

export type CanvasConnectedVideoMediaByRole = Partial<Record<CanvasImageRole, readonly CanvasConnectedVideoMedia[]>>;
