import axios from "axios";
import { afterEach, beforeEach, describe, expect, it, jest } from "bun:test";

import { defaultConfig } from "@/stores/use-config-store";
import { createVideoGenerationTask, pollVideoGenerationTask } from "./video";

const originalWindow = Object.getOwnPropertyDescriptor(globalThis, "window");
const originalLocalStorage = Object.getOwnPropertyDescriptor(globalThis, "localStorage");
const originalCustomEvent = Object.getOwnPropertyDescriptor(globalThis, "CustomEvent");
const originalFetch = Object.getOwnPropertyDescriptor(globalThis, "fetch");

function memoryStorage() {
    const values = new Map<string, string>();
    return {
        get length() {
            return values.size;
        },
        clear: () => values.clear(),
        getItem: (key: string) => values.get(key) ?? null,
        key: (index: number) => [...values.keys()][index] ?? null,
        removeItem: (key: string) => values.delete(key),
        setItem: (key: string, value: string) => values.set(key, value),
    } as Storage;
}

function videoConfig(route: string) {
    const model = "0::0::video-model";
    return {
        ...defaultConfig,
        model,
        videoModel: model,
        videoModels: [model],
        videoChannelId: 0,
        size: "9:16",
        videoGenerateAudio: "false",
        modelRoutes: { [`video:${model}`]: route },
    };
}

beforeEach(() => {
    const localStorage = memoryStorage();
    localStorage.setItem("infinite-canvas:auth_token", "token");
    const sessionStorage = memoryStorage();
    Object.defineProperty(globalThis, "window", { configurable: true, value: { dispatchEvent: () => true, localStorage, sessionStorage } });
    Object.defineProperty(globalThis, "localStorage", { configurable: true, value: localStorage });
    Object.defineProperty(globalThis, "CustomEvent", { configurable: true, value: class CustomEvent {} });
});

afterEach(() => {
    jest.restoreAllMocks();
    for (const [key, descriptor] of [
        ["window", originalWindow],
        ["localStorage", originalLocalStorage],
        ["CustomEvent", originalCustomEvent],
        ["fetch", originalFetch],
    ] as const) {
        if (descriptor) Object.defineProperty(globalThis, key, descriptor);
        else Reflect.deleteProperty(globalThis, key);
    }
});

describe("video aspect ratio routing", () => {
    it("sends protocol-specific 9:16 fields and keeps the resolved channel", async () => {
        const post = jest.spyOn(axios, "post").mockResolvedValue({
            data: { id: "task_123" },
            headers: { "x-resolved-channel-id": "2", "x-resolved-channel-model-id": "62" },
        });

        for (const route of ["openai", "veo_json", "waninter", "xai", "newapi", "seedance", "binghuo"] as const) {
            post.mockClear();
            const task = await createVideoGenerationTask(videoConfig(route), "test prompt");
            const [requestUrl, body] = post.mock.calls[0];
            const query = new URL(String(requestUrl)).searchParams;
            expect(query.get("routing_video_route")).toBe(route);
            expect(task).toMatchObject({ channelId: 2, channelModelId: 62 });

            if (route === "openai") {
                expect((body as FormData).get("size")).toBe("720x1280");
                expect((body as FormData).get("aspect_ratio")).toBeNull();
            } else if (route === "veo_json" || route === "xai") {
                expect(body).toMatchObject({ aspect_ratio: "9:16" });
            } else if (route === "waninter") {
                expect(body).toMatchObject({ size: "720x1280", aspect_ratio: "9:16" });
            } else if (route === "newapi") {
                expect(body).toMatchObject({ width: 720, height: 1280 });
            } else if (route === "seedance") {
                expect(body).toMatchObject({ ratio: "9:16" });
            } else {
                expect(body).toMatchObject({ ratio: "9:16", resolution: "720P", generate_audio: false, n: 1 });
                for (const field of ["size", "width", "height", "aspect_ratio", "image"]) expect(body).not.toHaveProperty(field);
            }
        }
    });

    it("maps Binghuo reference media to canonical fields", async () => {
        const post = jest.spyOn(axios, "post").mockResolvedValue({ data: { id: "task_binghuo" }, headers: {} });
        const config = { ...videoConfig("binghuo"), videoReferenceMode: "first_last" as const, vquality: "4K" };
        await createVideoGenerationTask(
            config,
            "test prompt",
            [
                { id: "first", name: "first.png", type: "image/png", dataUrl: "", url: "https://media.example.com/first.png" },
                { id: "last", name: "last.png", type: "image/png", dataUrl: "", url: "https://media.example.com/last.png" },
            ],
            [{ id: "video", name: "video.mp4", type: "video/mp4", url: "https://media.example.com/reference.mp4" }],
            [{ id: "audio", name: "audio.mp3", type: "audio/mpeg", url: "https://media.example.com/reference.mp3" }],
        );
        const body = post.mock.calls[0][1];
        expect(body).toMatchObject({
            start_frame: ["https://media.example.com/first.png"],
            end_frame: ["https://media.example.com/last.png"],
            reference_videos: ["https://media.example.com/reference.mp4"],
            reference_audios: ["https://media.example.com/reference.mp3"],
            resolution: "4K",
            n: 1,
        });
        expect(body).not.toHaveProperty("images");
    });

    it("requires exactly two ordered images in Binghuo first-last mode", async () => {
        await expect(createVideoGenerationTask({ ...videoConfig("binghuo"), videoReferenceMode: "first_last" }, "test prompt", [])).rejects.toThrow("恰好两张参考图");
    });

    it("pins polling to the physical channel that created the task", async () => {
        const get = jest.spyOn(axios, "get").mockResolvedValue({ data: { status: "processing" } });
        const state = await pollVideoGenerationTask(videoConfig("waninter"), {
            id: "task_123",
            provider: "openai",
            model: "0::0::video-model",
            channelId: 2,
            channelModelId: 62,
        });

        expect(state).toEqual({ status: "pending" });
        const query = new URL(String(get.mock.calls[0][0])).searchParams;
        expect(query.get("channel_id")).toBe("2");
        expect(query.get("channel_model_id")).toBe("62");
        expect(query.get("routing_model")).toBe("video-model");
        expect(query.has("routing_video_route")).toBe(false);
    });

    it("polls Binghuo tasks through the New API endpoint on the resolved channel", async () => {
        const get = jest.spyOn(axios, "get").mockResolvedValue({ data: { status: "processing" } });
        const state = await pollVideoGenerationTask(videoConfig("binghuo"), {
            id: "task_binghuo",
            provider: "binghuo",
            model: "0::0::video-model",
            channelId: 2,
            channelModelId: 62,
        });

        expect(state).toEqual({ status: "pending" });
        const url = new URL(String(get.mock.calls[0][0]));
        expect(url.searchParams.get("path")).toBe("/video/generations/task_binghuo");
        expect(url.searchParams.get("channel_id")).toBe("2");
        expect(url.searchParams.get("channel_model_id")).toBe("62");
    });

    it("pins proxied video content downloads to the task channel", async () => {
        const get = jest
            .spyOn(axios, "get")
            .mockResolvedValueOnce({ data: { status: "completed", url: "https://upstream.test/v1/videos/task_123/content" } })
            .mockResolvedValueOnce({ data: new Blob(["video"], { type: "video/mp4" }) });
        const state = await pollVideoGenerationTask(videoConfig("waninter"), {
            id: "task_123",
            provider: "openai",
            model: "0::0::video-model",
            channelId: 2,
            channelModelId: 62,
        });

        expect(state.status).toBe("completed");
        const query = new URL(String(get.mock.calls[1][0]), "https://app.test").searchParams;
        expect(query.get("channel_id")).toBe("2");
        expect(query.get("channel_model_id")).toBe("62");
        expect(query.get("routing_model")).toBe("video-model");
    });

    it("normalizes binary MP4 downloads to a playable video MIME type", async () => {
        const mp4Header = new Uint8Array([0, 0, 0, 24, 0x66, 0x74, 0x79, 0x70, 0x69, 0x73, 0x6f, 0x6d]);
        jest.spyOn(axios, "get")
            .mockResolvedValueOnce({ data: { status: "completed", url: "https://upstream.test/v1/videos/task_123/content" } })
            .mockResolvedValueOnce({ data: new Blob([mp4Header], { type: "application/octet-stream" }) });

        const state = await pollVideoGenerationTask(videoConfig("waninter"), {
            id: "task_123",
            provider: "openai",
            model: "0::0::video-model",
            channelId: 2,
            channelModelId: 62,
        });

        expect(state.status).toBe("completed");
        if (state.status === "completed") expect(state.result.blob?.type).toBe("video/mp4");
    });

    it("falls back to the Next.js proxy when the backend proxy returns a non-video body", async () => {
        const mp4Header = new Uint8Array([0, 0, 0, 24, 0x66, 0x74, 0x79, 0x70, 0x69, 0x73, 0x6f, 0x6d]);
        jest.spyOn(axios, "get")
            .mockResolvedValueOnce({ data: { status: "completed", url: "https://upstream.test/v1/videos/task_123/content" } })
            .mockResolvedValueOnce({ data: new Blob([JSON.stringify({ code: 500, msg: "proxy failed" })], { type: "application/json" }) });
        const fetchMock = jest.fn().mockResolvedValue(new Response(new Blob([mp4Header], { type: "application/octet-stream" }), { status: 200 }));
        Object.defineProperty(globalThis, "fetch", { configurable: true, value: fetchMock });

        const state = await pollVideoGenerationTask(videoConfig("waninter"), {
            id: "task_123",
            provider: "openai",
            model: "0::0::video-model",
            channelId: 2,
            channelModelId: 62,
        });

        expect(state.status).toBe("completed");
        if (state.status === "completed") expect(state.result.blob?.type).toBe("video/mp4");
        expect(fetchMock.mock.calls[0][0]).toBe("/webdav-proxy");
        expect(fetchMock.mock.calls[0][1].headers["x-webdav-target"]).toBe("https://upstream.test/v1/videos/task_123/content");
    });

    it("uses Binghuo result_url first and skips the backend proxy for signed video links", async () => {
        const mp4Header = new Uint8Array([0, 0, 0, 24, 0x66, 0x74, 0x79, 0x70, 0x69, 0x73, 0x6f, 0x6d]);
        const get = jest.spyOn(axios, "get").mockResolvedValueOnce({
            data: { status: "succeeded", result_url: "https://cdn.example.com/result.mp4?sig=1", url: "https://cdn.example.com/fallback.mp4?sig=2" },
        });
        const fetchMock = jest.fn().mockResolvedValue(new Response(new Blob([mp4Header], { type: "application/octet-stream" }), { status: 200 }));
        Object.defineProperty(globalThis, "fetch", { configurable: true, value: fetchMock });

        const state = await pollVideoGenerationTask(videoConfig("binghuo"), {
            id: "task_binghuo",
            provider: "binghuo",
            model: "0::0::video-model",
            channelId: 2,
            channelModelId: 62,
        });

        expect(state.status).toBe("completed");
        expect(get).toHaveBeenCalledTimes(1);
        expect(fetchMock.mock.calls[0][1].headers["x-webdav-target"]).toBe("https://cdn.example.com/result.mp4?sig=1");
        if (state.status === "completed") expect(state.result.blob?.type).toBe("video/mp4");
    });

    it("falls back to direct playback when the Next.js proxy cannot download a signed video link", async () => {
        const resultUrl = "https://cdn.example.com/result.mp4?sig=1";
        jest.spyOn(axios, "get").mockResolvedValueOnce({ data: { status: "succeeded", result_url: resultUrl } });
        Object.defineProperty(globalThis, "fetch", { configurable: true, value: jest.fn().mockResolvedValue(new Response("bad gateway", { status: 502 })) });

        const state = await pollVideoGenerationTask(videoConfig("binghuo"), {
            id: "task_binghuo",
            provider: "binghuo",
            model: "0::0::video-model",
            channelId: 2,
            channelModelId: 62,
        });

        expect(state.status).toBe("completed");
        if (state.status === "completed") expect(state.result).toEqual({ url: resultUrl, mimeType: "video/mp4" });
    });

    it("keeps Binghuo failed task error strings", async () => {
        const upstreamError = "视频生成失败：该模型不支持真人脸 / 真实人物照片作为参考图，请更换为非真人参考图。";
        jest.spyOn(axios, "get").mockResolvedValueOnce({ data: { status: "failed", error: upstreamError } });

        const state = await pollVideoGenerationTask(videoConfig("binghuo"), {
            id: "task_binghuo",
            provider: "binghuo",
            model: "0::0::video-model",
            channelId: 2,
            channelModelId: 62,
        });

        expect(state).toEqual({ status: "failed", error: upstreamError });
    });

    it("fails instead of returning an unverified remote video URL", async () => {
        jest.spyOn(axios, "get")
            .mockResolvedValueOnce({ data: { status: "completed", url: "https://upstream.test/v1/videos/task_123/content" } })
            .mockResolvedValueOnce({ data: new Blob([JSON.stringify({ code: 500, msg: "proxy failed" })], { type: "application/json" }) });
        Object.defineProperty(globalThis, "fetch", { configurable: true, value: jest.fn().mockResolvedValue(new Response("bad gateway", { status: 502 })) });

        await expect(
            pollVideoGenerationTask(videoConfig("waninter"), {
                id: "task_123",
                provider: "openai",
                model: "0::0::video-model",
                channelId: 2,
                channelModelId: 62,
            }),
        ).rejects.toThrow("视频成片下载失败");
    });
});
