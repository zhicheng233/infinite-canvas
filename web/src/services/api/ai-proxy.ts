import axios from "axios";
import { API_BASE, getStoredToken } from "./client";

export function isLoggedIn(): boolean {
    return !!getStoredToken();
}

function proxyHeaders() {
    const token = getStoredToken();
    return token ? { Authorization: `Bearer ${token}` } : {};
}

export async function proxyAiPost(path: string, body: any, config?: { signal?: AbortSignal; headers?: Record<string, string>; responseType?: string }) {
    const url = `${API_BASE}/proxy?path=${encodeURIComponent(path)}`;
    return axios({
        method: "POST",
        url,
        data: body,
        headers: { ...proxyHeaders(), ...(config?.headers || {}) },
        signal: config?.signal,
        responseType: config?.responseType as any,
    });
}

export async function proxyAiGet(path: string, config?: { signal?: AbortSignal; responseType?: string }) {
    const url = `${API_BASE}/proxy?path=${encodeURIComponent(path)}`;
    return axios({
        method: "GET",
        url,
        headers: proxyHeaders(),
        signal: config?.signal,
        responseType: config?.responseType as any,
    });
}

export async function proxyAiGetPath(path: string, config?: { signal?: AbortSignal; responseType?: string }) {
    const url = `${API_BASE}/proxy${path}`;
    return axios({
        method: "GET",
        url,
        headers: proxyHeaders(),
        signal: config?.signal,
        responseType: config?.responseType as any,
    });
}
