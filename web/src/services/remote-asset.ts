"use client";

function isRemoteHttpUrl(value: string) {
    return /^https?:\/\//i.test(value.trim());
}

function isSameOriginUrl(value: string) {
    if (typeof window === "undefined") return false;
    try {
        return new URL(value, window.location.origin).origin === window.location.origin;
    } catch {
        return false;
    }
}

export async function fetchAssetBlob(input: string | Blob): Promise<Blob> {
    if (input instanceof Blob) return input;
    const url = input.trim();
    if (!url) throw new Error("资源地址不能为空");
    if (!isRemoteHttpUrl(url) || isSameOriginUrl(url)) return await (await fetch(url)).blob();

    const response = await fetch("/webdav-proxy", {
        method: "POST",
        headers: {
            "x-webdav-target": url,
            "x-webdav-method": "GET",
        },
    });
    if (!response.ok) throw new Error(`拉取远程资源失败：${response.status}`);
    return await response.blob();
}
