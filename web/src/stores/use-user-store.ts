"use client";

import { create } from "zustand";
import { DEFAULT_CANVAS_SCOPE, useCanvasStore } from "@/app/(user)/canvas/stores/use-canvas-store";
import type { ApiUser } from "@/services/api/auth";
import { getMe } from "@/services/api/auth";
import { clearStoredToken, getStoredToken } from "@/services/api/client";

export type LocalUser = {
    id: string;
    username: string;
    displayName: string;
    avatarUrl: string;
    role: string;
};

type UserStore = {
    user: LocalUser | null;
    loading: boolean;
    fetchUser: () => Promise<void>;
    clearSession: () => void;
};

function fromApiUser(apiUser: ApiUser): LocalUser {
    return {
        id: String(apiUser.id),
        username: apiUser.username,
        displayName: apiUser.display_name || apiUser.username,
        avatarUrl: apiUser.avatar_url || "",
        role: apiUser.role || "user",
    };
}

function resolveCanvasScope(userId?: string) {
    return userId ? `user:${userId}` : DEFAULT_CANVAS_SCOPE;
}

export const useUserStore = create<UserStore>()((set) => ({
    user: null,
    loading: false,
    fetchUser: async () => {
        if (!getStoredToken()) {
            await useCanvasStore.getState().setStorageScope(DEFAULT_CANVAS_SCOPE);
            return;
        }
        set({ loading: true });
        try {
            const apiUser = await getMe();
            const user = fromApiUser(apiUser);
            set({ user, loading: false });
            await useCanvasStore.getState().setStorageScope(resolveCanvasScope(user.id));
        } catch {
            clearStoredToken();
            set({ user: null, loading: false });
            await useCanvasStore.getState().setStorageScope(DEFAULT_CANVAS_SCOPE);
        }
    },
    clearSession: () => {
        clearStoredToken();
        set({ user: null });
        void useCanvasStore.getState().setStorageScope(DEFAULT_CANVAS_SCOPE);
    },
}));
