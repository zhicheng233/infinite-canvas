import { create } from "zustand";
import { persist, type PersistStorage, type StorageValue } from "zustand/middleware";

import { nanoid } from "nanoid";
import { localForageStorage } from "@/lib/localforage-storage";
import type { CanvasBackgroundMode } from "@/lib/canvas-theme";
import type { CanvasAssistantSession, CanvasConnection, CanvasNodeData, ViewportTransform } from "../types";
import { CanvasConflictError, saveCanvas, deleteBatchCanvas, listCanvases } from "@/services/api/canvas";
import { normalizeCanvasConnections } from "../utils/canvas-connections";
import { CURRENT_CANVAS_SCHEMA_VERSION, migrateCanvasProject } from "../utils/canvas-document-migrations";

export type CanvasProject = {
    id: string;
    schemaVersion: number;
    revision: number;
    title: string;
    createdAt: string;
    updatedAt: string;
    nodes: CanvasNodeData[];
    connections: CanvasConnection[];
    chatSessions: CanvasAssistantSession[];
    activeChatId: string | null;
    backgroundMode: CanvasBackgroundMode;
    showImageInfo: boolean;
    viewport: ViewportTransform;
};

export type CanvasProjectSaveState = {
    status: "idle" | "saving" | "saved" | "failed";
    lastSavedAt?: string;
    error?: string;
};

type CanvasStore = {
    hydrated: boolean;
    currentScope: string;
    projects: CanvasProject[];
    saveStates: Record<string, CanvasProjectSaveState>;
    createProject: (title?: string) => string;
    importProject: (project: Partial<CanvasProject>) => string;
    openProject: (id: string) => CanvasProject | null;
    renameProject: (id: string, title: string) => void;
    deleteProjects: (ids: string[]) => void;
    replaceProjects: (projects: CanvasProject[]) => void;
    updateProject: (id: string, patch: Partial<Pick<CanvasProject, "nodes" | "connections" | "chatSessions" | "activeChatId" | "backgroundMode" | "showImageInfo" | "viewport">>) => void;
    syncFromCloud: () => Promise<void>;
    flushProjectSave: (id: string) => Promise<void>;
    setStorageScope: (scope: string) => Promise<void>;
};

const initialViewport: ViewportTransform = { x: 0, y: 0, k: 1 };
const CANVAS_STORE_KEY_PREFIX = "infinite-canvas:canvas_store";
export const DEFAULT_CANVAS_SCOPE = "guest";
type PersistedCanvasState = Pick<CanvasStore, "projects">;
const saveTimers = new Map<string, ReturnType<typeof setTimeout>>();
const queuedPersistStates = new Map<string, PersistedCanvasState>();

// Cloud save debounce: 2 seconds after last update per project
const cloudSaveTimers = new Map<string, ReturnType<typeof setTimeout>>();
const cloudSaveControllers = new Map<string, AbortController>();

function cloudOperationKey(scope: string, projectId: string) {
    return `${scope}:${projectId}`;
}

function normalizeProjectConnections(project: CanvasProject): CanvasProject {
    return migrateCanvasProject(project);
}

function setProjectSaveState(scope: string, projectId: string, patch: Partial<CanvasProjectSaveState>) {
    useCanvasStore.setState((state) => {
        if (state.currentScope !== scope) return state;
        return {
            saveStates: {
                ...state.saveStates,
                [projectId]: { ...state.saveStates[projectId], status: state.saveStates[projectId]?.status || "idle", ...patch },
            },
        };
    });
}

function removeProjectSaveStates(projectIds: string[]) {
    if (!projectIds.length) return;
    useCanvasStore.setState((state) => {
        const saveStates = { ...state.saveStates };
        projectIds.forEach((projectId) => {
            delete saveStates[projectId];
        });
        return { saveStates };
    });
}

async function flushProjectToCloud(project: CanvasProject, scope: string) {
    if (useCanvasStore.getState().currentScope !== scope) return;
    const key = cloudOperationKey(scope, project.id);
    const controller = new AbortController();
    cloudSaveControllers.get(key)?.abort();
    cloudSaveControllers.set(key, controller);
    setProjectSaveState(scope, project.id, { status: "saving", error: undefined });
    try {
        const saved = await saveCanvas(project, controller.signal);
        if (useCanvasStore.getState().currentScope !== scope) return;
        useCanvasStore.setState((state) => ({ projects: state.projects.map((item) => (item.id === project.id ? { ...item, revision: saved.revision, schemaVersion: saved.schemaVersion } : item)) }));
        setProjectSaveState(scope, project.id, { status: "saved", lastSavedAt: new Date().toISOString(), error: undefined });
    } catch (error) {
        if (controller.signal.aborted || useCanvasStore.getState().currentScope !== scope) return;
        if (error instanceof CanvasConflictError) {
            const now = new Date().toISOString();
            const conflictCopy = { ...project, id: nanoid(), revision: 0, title: `${project.title}（本地冲突副本）`, createdAt: now, updatedAt: now };
            useCanvasStore.setState((state) => ({ projects: [conflictCopy, ...state.projects.map((item) => (item.id === project.id ? error.latest : item))] }));
            scheduleCloudSave(conflictCopy, scope);
        }
        setProjectSaveState(scope, project.id, { status: "failed", error: error instanceof Error ? error.message : "保存失败" });
        throw error;
    } finally {
        if (cloudSaveControllers.get(key) === controller) cloudSaveControllers.delete(key);
    }
}

function scheduleCloudSave(project: CanvasProject, scope = useCanvasStore.getState().currentScope) {
    const key = cloudOperationKey(scope, project.id);
    const existing = cloudSaveTimers.get(key);
    if (existing) clearTimeout(existing);
    setProjectSaveState(scope, project.id, { status: "saving", error: undefined });
    cloudSaveTimers.set(
        key,
        setTimeout(() => {
            cloudSaveTimers.delete(key);
            flushProjectToCloud(project, scope).catch((err) => console.error("[CanvasStore] Cloud save failed:", err));
        }, 2000),
    );
}

function cancelCloudOperations(scope: string) {
    const prefix = `${scope}:`;
    for (const [key, timer] of cloudSaveTimers) {
        if (!key.startsWith(prefix)) continue;
        clearTimeout(timer);
        cloudSaveTimers.delete(key);
    }
    for (const [key, controller] of cloudSaveControllers) {
        if (!key.startsWith(prefix)) continue;
        controller.abort();
        cloudSaveControllers.delete(key);
    }
}

function resolveCanvasStoreKey(scope: string) {
    return `${CANVAS_STORE_KEY_PREFIX}:${scope || DEFAULT_CANVAS_SCOPE}`;
}

const canvasStorage: PersistStorage<CanvasStore> = {
    getItem: async (name) => {
        const value = await localForageStorage.getItem(name);
        if (!value) return null;
        const parsed = JSON.parse(value) as StorageValue<CanvasStore>;
        const normalized = { ...parsed, state: { ...parsed.state, projects: Array.isArray(parsed.state.projects) ? parsed.state.projects.map(normalizeProjectConnections) : [] } };
        queuedPersistStates.set(name, normalized.state as PersistedCanvasState);
        return normalized;
    },
    setItem: (name, value) => {
        const nextState = value.state as PersistedCanvasState;
        const queuedPersistState = queuedPersistStates.get(name);
        if (queuedPersistState && queuedPersistState.projects === nextState.projects) return;
        queuedPersistStates.set(name, nextState);
        const saveTimer = saveTimers.get(name);
        if (saveTimer) clearTimeout(saveTimer);
        saveTimers.set(
            name,
            setTimeout(() => {
                saveTimers.delete(name);
                void localForageStorage.setItem(name, JSON.stringify(value));
            }, 400),
        );
    },
    removeItem: (name) => {
        const saveTimer = saveTimers.get(name);
        if (saveTimer) {
            clearTimeout(saveTimer);
            saveTimers.delete(name);
        }
        queuedPersistStates.delete(name);
        return localForageStorage.removeItem(name);
    },
};

export const useCanvasStore = create<CanvasStore>()(
    persist(
        (set, get) => ({
            hydrated: false,
            currentScope: DEFAULT_CANVAS_SCOPE,
            projects: [],
            saveStates: {},
            createProject: (title = "未命名画布") => {
                const now = new Date().toISOString();
                const id = nanoid();
                const project: CanvasProject = {
                    id,
                    schemaVersion: CURRENT_CANVAS_SCHEMA_VERSION,
                    revision: 0,
                    title,
                    createdAt: now,
                    updatedAt: now,
                    nodes: [],
                    connections: [],
                    chatSessions: [],
                    activeChatId: null,
                    backgroundMode: "lines",
                    showImageInfo: false,
                    viewport: initialViewport,
                };
                set((state) => ({ projects: [project, ...state.projects] }));
                flushProjectToCloud(project, get().currentScope).catch((err) => console.error("[CanvasStore] Create cloud save failed:", err));
                return id;
            },
            importProject: (source) => {
                const now = new Date().toISOString();
                const project: CanvasProject = {
                    id: nanoid(),
                    schemaVersion: CURRENT_CANVAS_SCHEMA_VERSION,
                    revision: 0,
                    title: source.title || "导入画布",
                    createdAt: source.createdAt || now,
                    updatedAt: now,
                    nodes: source.nodes || [],
                    connections: normalizeCanvasConnections(source.connections),
                    chatSessions: source.chatSessions || [],
                    activeChatId: source.activeChatId || null,
                    backgroundMode: source.backgroundMode || "lines",
                    showImageInfo: source.showImageInfo || false,
                    viewport: source.viewport || initialViewport,
                };
                set((state) => ({ projects: [project, ...state.projects] }));
                flushProjectToCloud(project, get().currentScope).catch((err) => console.error("[CanvasStore] Import cloud save failed:", err));
                return project.id;
            },
            openProject: (id) => {
                return get().projects.find((item) => item.id === id) || null;
            },
            renameProject: (id, title) => {
                set((state) => ({
                    projects: state.projects.map((project) => (project.id === id ? { ...project, title: title.trim() || project.title, updatedAt: new Date().toISOString() } : project)),
                }));
                const project = get().projects.find((p) => p.id === id);
                if (project) scheduleCloudSave(project);
            },
            deleteProjects: (ids) => {
                set((state) => {
                    const projects = state.projects.filter((project) => !ids.includes(project.id));
                    return { projects };
                });
                removeProjectSaveStates(ids);
                deleteBatchCanvas(ids).catch((err) => console.error("[CanvasStore] Cloud delete failed:", err));
            },
            replaceProjects: (projects) => set({ projects: projects.map(normalizeProjectConnections) }),
            updateProject: (id, patch) => {
                const normalizedPatch = patch.connections ? { ...patch, connections: normalizeCanvasConnections(patch.connections) } : patch;
                set((state) => ({
                    projects: state.projects.map((project) => (project.id === id ? { ...project, ...normalizedPatch, updatedAt: new Date().toISOString() } : project)),
                }));
                const project = get().projects.find((p) => p.id === id);
                if (project) scheduleCloudSave(project);
            },
            syncFromCloud: async () => {
                if (typeof window === "undefined") return;
                const scope = get().currentScope;
                try {
                    const cloudProjects = await listCanvases();
                    if (get().currentScope !== scope) return;
                    const localProjects = get().projects;
                    const merged = new Map(localProjects.map((p) => [p.id, p]));
                    for (const cp of cloudProjects) {
                        const local = merged.get(cp.id);
                        const dirty = get().saveStates[cp.id]?.status === "saving" || get().saveStates[cp.id]?.status === "failed";
                        if (!local || (!dirty && Date.parse(cp.updatedAt) >= Date.parse(local.updatedAt))) merged.set(cp.id, normalizeProjectConnections(cp));
                    }
                    set({ projects: Array.from(merged.values()) });
                } catch (err) {
                    console.error("[CanvasStore] Cloud sync failed:", err);
                }
            },
            flushProjectSave: async (id) => {
                const scope = get().currentScope;
                const key = cloudOperationKey(scope, id);
                const saveTimer = cloudSaveTimers.get(key);
                if (saveTimer) {
                    clearTimeout(saveTimer);
                    cloudSaveTimers.delete(key);
                }
                const project = get().projects.find((item) => item.id === id);
                if (!project) return;
                try {
                    await flushProjectToCloud(project, scope);
                } catch (err) {
                    console.error("[CanvasStore] Flush cloud save failed:", err);
                }
            },
            setStorageScope: async (scope) => {
                const nextScope = scope || DEFAULT_CANVAS_SCOPE;
                if (get().currentScope === nextScope && get().hydrated) {
                    return;
                }
                cancelCloudOperations(get().currentScope);
                set({ currentScope: nextScope, projects: [], saveStates: {}, hydrated: false });
                useCanvasStore.persist.setOptions({ name: resolveCanvasStoreKey(nextScope) });
                await useCanvasStore.persist.rehydrate();
            },
        }),
        {
            name: resolveCanvasStoreKey(DEFAULT_CANVAS_SCOPE),
            storage: canvasStorage,
            partialize: (state) =>
                ({
                    projects: state.projects,
                }) as StorageValue<CanvasStore>["state"],
            onRehydrateStorage: () => () => {
                if (typeof window === "undefined") return;
                useCanvasStore.setState({ hydrated: true });
                void useCanvasStore.getState().syncFromCloud();
            },
        },
    ),
);
