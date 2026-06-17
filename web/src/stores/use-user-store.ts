"use client";

import { create } from "zustand";
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

export const useUserStore = create<UserStore>()((set) => ({
  user: null,
  loading: false,
  fetchUser: async () => {
    if (!getStoredToken()) return;
    set({ loading: true });
    try {
      const apiUser = await getMe();
      set({ user: fromApiUser(apiUser), loading: false });
    } catch {
      clearStoredToken();
      set({ user: null, loading: false });
    }
  },
  clearSession: () => {
    clearStoredToken();
    set({ user: null });
  },
}));