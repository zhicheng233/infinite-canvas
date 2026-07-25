import axios from "axios";
import { afterEach, beforeEach, describe, expect, it, jest } from "bun:test";

import { defaultConfig } from "@/stores/use-config-store";
import { createVideoGenerationTask, pollVideoGenerationTask } from "./video";

const originalWindow = Object.getOwnPropertyDescriptor(globalThis, "window");
const originalLocalStorage = Object.getOwnPropertyDescriptor(globalThis, "localStorage");
const originalCustomEvent = Object.getOwnPropertyDescriptor(globalThis, "CustomEvent");

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

        for (const route of ["openai", "veo_json", "waninter", "xai", "newapi", "seedance"] as const) {
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
            } else {
                expect(body).toMatchObject({ ratio: "9:16" });
            }
        }
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
});
