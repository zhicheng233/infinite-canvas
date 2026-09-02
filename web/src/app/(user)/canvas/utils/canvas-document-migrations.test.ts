import { expect, test } from "bun:test";
import type { CanvasProject } from "../stores/use-canvas-store";
import { CURRENT_CANVAS_SCHEMA_VERSION, migrateCanvasProject } from "./canvas-document-migrations";

function project(overrides: Partial<CanvasProject> = {}) {
    return {
        id: "legacy", title: "legacy", createdAt: "2026-01-01", updatedAt: "2026-01-01",
        nodes: [], connections: [], chatSessions: [], activeChatId: null, backgroundMode: "lines" as const,
        showImageInfo: false, viewport: { x: 0, y: 0, k: 1 }, ...overrides,
    };
}

test("unversioned canvas migrates from v1 through the current schema", () => {
    const migrated = migrateCanvasProject(project() as CanvasProject);
    expect(migrated.schemaVersion).toBe(CURRENT_CANVAS_SCHEMA_VERSION);
    expect(migrated.revision).toBe(0);
});

test("v1 canvas normalizes image role aliases during migration", () => {
    const migrated = migrateCanvasProject(project({
        schemaVersion: 1,
        revision: 4,
        connections: [
            { id: "images", fromNodeId: "a", toNodeId: "b", targetImageRole: "images" },
            { id: "legacy", fromNodeId: "a", toNodeId: "b" },
        ],
    }));
    expect(migrated.connections).toHaveLength(1);
    expect(migrated.connections[0].targetImageRole).toBeUndefined();
    expect(migrated.revision).toBe(4);
});

test("future canvas versions are preserved for a newer client", () => {
    const migrated = migrateCanvasProject(project({ schemaVersion: 3, revision: 1 }));
    expect(migrated.schemaVersion).toBe(3);
});
