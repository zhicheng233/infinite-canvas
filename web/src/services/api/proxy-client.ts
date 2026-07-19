import apiClient, { getStoredToken } from "./client";

export function shouldUseProxy(): boolean {
    return !!getStoredToken();
}

export function proxyPost(path: string, body: any, contentType?: string) {
    const headers: Record<string, string> = {};
    if (contentType) {
        headers["Content-Type"] = contentType;
    }
    return apiClient.post("/proxy?path=" + encodeURIComponent(path), body, { headers });
}

export function proxyGet(path: string) {
    return apiClient.get("/proxy?path=" + encodeURIComponent(path));
}

export function proxyGetPath(path: string) {
    return apiClient.get("/proxy" + path);
}
