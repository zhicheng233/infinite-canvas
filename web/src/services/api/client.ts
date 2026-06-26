import axios from "axios";

export function resolveApiBaseUrl(): string {
  const configured = process.env.NEXT_PUBLIC_API_URL?.trim();
  if (configured) return configured.replace(/\/+$/, "");
  if (typeof window !== "undefined") {
    return `${window.location.protocol}//${window.location.hostname}:18080/api`;
  }
  return "http://localhost:18080/api";
}

export const API_BASE = resolveApiBaseUrl();

const apiClient = axios.create({ baseURL: API_BASE });

apiClient.interceptors.request.use((config) => {
  if (typeof window !== "undefined") {
    const token = localStorage.getItem("infinite-canvas:auth_token");
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
  }
  return config;
});

apiClient.interceptors.response.use(
  (response) => {
    const data = response.data;
    if (data && typeof data.code === "number" && data.code !== 0) {
      return Promise.reject(new Error(data.msg || "request failed"));
    }
    return response;
  },
  (error) => {
    if (axios.isAxiosError(error) && error.response?.status === 401) {
      if (typeof window !== "undefined") {
        localStorage.removeItem("infinite-canvas:auth_token");
      }
    }
    return Promise.reject(error);
  }
);

export default apiClient;

export function getStoredToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem("infinite-canvas:auth_token");
}

export function setStoredToken(token: string): void {
  localStorage.setItem("infinite-canvas:auth_token", token);
}

export function clearStoredToken(): void {
  localStorage.removeItem("infinite-canvas:auth_token");
}
