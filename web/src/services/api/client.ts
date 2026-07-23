import axios from "axios";

const AUTH_TOKEN_KEY = "infinite-canvas:auth_token";
export const AUTH_TOKEN_CHANGE_EVENT = "infinite-canvas:auth-token-change";

export function resolveApiBaseUrl(): string {
    const configured = process.env.NEXT_PUBLIC_API_URL?.trim();
    if (configured) return configured.replace(/\/+$/, "");
    if (typeof window !== "undefined") {
        return `${window.location.origin}/backend-api`;
    }
    return "http://localhost:18080/backend-api";
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
            if (typeof window !== "undefined") clearStoredToken();
        }
        return Promise.reject(error);
    },
);

export default apiClient;

export function getStoredToken(): string | null {
    if (typeof window === "undefined") return null;
    return localStorage.getItem(AUTH_TOKEN_KEY);
}

export function setStoredToken(token: string): void {
    if (typeof window === "undefined") return;
    localStorage.setItem(AUTH_TOKEN_KEY, token);
    window.dispatchEvent(new CustomEvent(AUTH_TOKEN_CHANGE_EVENT, { detail: token }));
}

export function clearStoredToken(): void {
    if (typeof window === "undefined") return;
    localStorage.removeItem(AUTH_TOKEN_KEY);
    window.dispatchEvent(new CustomEvent(AUTH_TOKEN_CHANGE_EVENT));
}
