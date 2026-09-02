import type { CanvasProject } from "../stores/use-canvas-store";
import { normalizeCanvasConnections } from "./canvas-connections";

export const CURRENT_CANVAS_SCHEMA_VERSION = 2;

type CanvasDocument = Omit<CanvasProject, "schemaVersion" | "revision"> & { schemaVersion?: number; revision?: number };

export function migrateCanvasProject(source: CanvasDocument): CanvasProject {
    let project = { ...source, schemaVersion: normalizeSchemaVersion(source.schemaVersion), revision: source.revision || 0 } as CanvasProject;
    while (project.schemaVersion < CURRENT_CANVAS_SCHEMA_VERSION) {
        if (project.schemaVersion === 1) {
            project = migrateCanvasV1ToV2(project);
            continue;
        }
        throw new Error(`不支持的画布数据版本：${project.schemaVersion}`);
    }
    return project;
}

function migrateCanvasV1ToV2(project: CanvasProject): CanvasProject {
    return { ...project, schemaVersion: 2, connections: normalizeCanvasConnections(project.connections) };
}

function normalizeSchemaVersion(value?: number) {
    if (value === undefined || value === null || value === 0) return 1;
    if (!Number.isSafeInteger(value) || value < 1) throw new Error("画布数据版本无效");
    return value;
}
