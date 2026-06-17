import axios from "axios";

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api";

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