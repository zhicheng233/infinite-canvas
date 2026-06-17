import apiClient from "./client";

export type ApiUser = {
  id: number;
  username: string;
  display_name: string;
  avatar_url: string;
  tenant_id: number;
  role: string;
};

type AuthResponse = { token: string; user: ApiUser };

export async function register(input: { tenant_name?: string; username: string; password: string }) {
  const res = await apiClient.post("/auth/register", input);
  return res.data.data as AuthResponse;
}

export async function login(input: { username: string; password: string }) {
  const res = await apiClient.post("/auth/login", input);
  return res.data.data as AuthResponse;
}

export async function getMe() {
  const res = await apiClient.get("/auth/me");
  return res.data.data as ApiUser;
}