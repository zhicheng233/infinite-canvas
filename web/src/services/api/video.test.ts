import axios from "axios";
import { afterEach, beforeEach, describe, expect, it, jest } from "bun:test";

import type { CustomVideoConfig } from "@/lib/custom-video-config";
import { createEmptyCustomVideoMediaState, type CustomVideoRuntimeSnapshot } from "@/lib/custom-video-runtime";
import { defaultConfig, useConfigStore } from "@/stores/use-config-store";
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

function customVideoConfig(overrides: Partial<CustomVideoConfig> = {}) {
    const model = "0::0::video-model";
    const customConfig: CustomVideoConfig = {
        seconds: { enabled: true, key: "duration", mode: "range", min: 3, max: 10, step: 1, default: 6 },
        dimensions: { enabled: true, mode: "size", key: "resolution", options: ["1280x720", "720x1280"], default: "1280x720" },
        images: { enabled: true, key: "image", max_count: 1 },
        input_reference: { enabled: true, key: "firstFrame", max_count: 1 },
        style_references: { enabled: true, key: "styleReferences", max_count: 4 },
        element_references: { enabled: true, key: "elementReferences", max_count: 3 },
        reference_images: { enabled: true, key: "referenceImages", max_count: 4 },
        reference_mode: { enabled: true, key: "referenceMode", options: ["frame", "style", "element"], default: "element" },
        input_video: { enabled: true, key: "sourceVideo", max_count: 1 },
        audio: { enabled: true, key: "generateAudio", mode: "user", value: false },
        n: { enabled: true, key: "count", value: 1 },
        ...overrides,
    };
    return {
        ...videoConfig("custom"),
        modelCustomVideoConfigs: { [model]: customConfig },
    };
}

function customRuntime(values: CustomVideoRuntimeSnapshot["values"] = {}, media: Partial<CustomVideoRuntimeSnapshot["media"]> = {}): CustomVideoRuntimeSnapshot {
    return { values, media: { ...createEmptyCustomVideoMediaState(), ...media } };
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
    useConfigStore.getState().invalidateServerCatalogRefresh();
    useConfigStore.setState({ config: defaultConfig });
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

describe("custom OpenAI video serialization", () => {
    it("sends only enabled aliases with single and multi-value media", async () => {
        const post = jest.spyOn(axios, "post").mockResolvedValue({ data: { task_id: "task_custom" }, headers: {} });
        const runtime = customRuntime(
            { seconds: 8, dimension: "720x1280", reference_mode: "style", audio: true },
            {
                images: ["https://media.example.com/image.png"],
                input_reference: ["https://media.example.com/first.png"],
                style_references: ["https://media.example.com/style-1.png", "https://media.example.com/style-2.png"],
                reference_images: ["https://media.example.com/reference.png"],
                input_video: ["https://media.example.com/source.mp4"],
            },
        );

        const task = await createVideoGenerationTask(customVideoConfig(), "test prompt", [], [], [], { customVideoRuntime: runtime });

        const [requestUrl, body, requestConfig] = post.mock.calls[0];
        const url = new URL(String(requestUrl));
        expect(url.searchParams.get("path")).toBe("/videos");
        expect(url.searchParams.get("routing_video_route")).toBe("custom");
        expect(requestConfig?.headers).toEqual({ Authorization: "Bearer token", "Content-Type": "application/json" });
        expect(body).toEqual({
            model: "video-model",
            prompt: "test prompt",
            duration: 8,
            resolution: "720x1280",
            image: "https://media.example.com/image.png",
            firstFrame: "https://media.example.com/first.png",
            styleReferences: ["https://media.example.com/style-1.png", "https://media.example.com/style-2.png"],
            referenceImages: "https://media.example.com/reference.png",
            referenceMode: "style",
            sourceVideo: "https://media.example.com/source.mp4",
            generateAudio: true,
            count: 1,
        });
        for (const canonical of ["seconds", "size", "aspect_ratio", "images", "input_reference", "style_references", "reference_images", "reference_mode", "input_video", "audio", "n"]) expect(body).not.toHaveProperty(canonical);
        expect(task).toEqual({ id: "task_custom", provider: "openai", model: "0::0::video-model" });
    });

    it("omits disabled and empty values and emits only the configured dimension mode", async () => {
        const post = jest.spyOn(axios, "post").mockResolvedValue({ data: { id: "task_ratio" }, headers: {} });
        const config = customVideoConfig({
            seconds: { enabled: false, key: "duration", mode: "range", min: 3, max: 10, step: 1, default: 6 },
            dimensions: { enabled: true, mode: "aspect_ratio", key: "ratio", options: ["16:9", "9:16"], default: "16:9" },
            images: { enabled: false, key: "image", max_count: 1 },
            reference_mode: { enabled: false, key: "referenceMode", options: [], default: "" },
            audio: { enabled: false, key: "generateAudio", mode: "fixed", value: true },
            n: { enabled: false, key: "count", value: 1 },
        });

        await createVideoGenerationTask(config, "test prompt", [], [], [], {
            customVideoRuntime: customRuntime({ dimension: "9:16" }, { style_references: [], element_references: [""], reference_images: [] }),
        });

        expect(post.mock.calls[0][1]).toEqual({ model: "video-model", prompt: "test prompt", ratio: "9:16" });
        expect(post.mock.calls[0][1]).not.toHaveProperty("size");
        expect(post.mock.calls[0][1]).not.toHaveProperty("aspect_ratio");
    });

    it("sends fixed audio and n values but omits reference mode without reference images", async () => {
        const post = jest.spyOn(axios, "post").mockResolvedValue({ data: { id: "task_fixed" }, headers: {} });
        const config = customVideoConfig({ audio: { enabled: true, key: "sound", mode: "fixed", value: true }, n: { enabled: true, key: "outputs", value: 2 } });

        await createVideoGenerationTask(config, "test prompt", [], [], [], {
            customVideoRuntime: customRuntime({ seconds: 6, dimension: "1280x720", reference_mode: "frame", audio: false }),
        });

        expect(post.mock.calls[0][1]).toMatchObject({ sound: true, outputs: 2 });
        expect(post.mock.calls[0][1]).not.toHaveProperty("referenceMode");
    });

    it("rejects invalid config and runtime states before any network request", async () => {
        const post = jest.spyOn(axios, "post").mockResolvedValue({ data: { id: "unexpected" }, headers: {} });
        await expect(createVideoGenerationTask(videoConfig("custom"), "test prompt", [], [], [], { customVideoRuntime: customRuntime() })).rejects.toThrow("配置无效");
        await expect(createVideoGenerationTask(customVideoConfig(), "test prompt")).rejects.toThrow("运行参数缺失");
        const cases = [
            {
                config: customVideoConfig({ dimensions: { enabled: true, mode: "size", key: "duration", options: ["1280x720"], default: "1280x720" } }),
                runtime: customRuntime({ seconds: 6, dimension: "1280x720" }),
                error: "重复",
            },
            { config: customVideoConfig(), runtime: customRuntime({ seconds: 11, dimension: "1280x720" }), error: "seconds" },
            { config: customVideoConfig(), runtime: customRuntime({ seconds: 6, dimension: "640x640" }), error: "dimensions" },
            {
                config: customVideoConfig(),
                runtime: customRuntime(
                    { seconds: 6, dimension: "1280x720" },
                    { style_references: ["https://media.example.com/1.png", "https://media.example.com/2.png", "https://media.example.com/3.png", "https://media.example.com/4.png", "https://media.example.com/5.png"] },
                ),
                error: "style_references",
            },
            {
                config: customVideoConfig(),
                runtime: customRuntime({ seconds: 6, dimension: "1280x720" }, { images: ["not-a-media-url"] }),
                error: "素材地址无效",
            },
        ];

        for (const item of cases) {
            await expect(createVideoGenerationTask(item.config, "test prompt", [], [], [], { customVideoRuntime: item.runtime })).rejects.toThrow(item.error);
        }
        expect(post).not.toHaveBeenCalled();
    });

    it("polls custom-created tasks through the OpenAI videos endpoint", async () => {
        jest.spyOn(axios, "post").mockResolvedValue({ data: { task_id: "task_poll" }, headers: {} });
        const get = jest.spyOn(axios, "get").mockResolvedValue({ data: { status: "processing" }, headers: {} });
        const config = customVideoConfig();
        const task = await createVideoGenerationTask(config, "test prompt", [], [], [], {
            customVideoRuntime: customRuntime({ seconds: 6, dimension: "1280x720" }),
        });

        const state = await pollVideoGenerationTask(config, task);

        expect(state).toEqual({ status: "pending" });
        expect(new URL(String(get.mock.calls[0][0])).searchParams.get("path")).toBe("/videos/task_poll");
    });
});

describe("video aspect ratio routing", () => {
    it("sends protocol-specific 9:16 fields and keeps the resolved channel", async () => {
        const post = jest.spyOn(axios, "post").mockResolvedValue({
            data: { id: "task_123" },
            headers: { "x-resolved-channel-id": "2", "x-resolved-channel-model-id": "62" },
        });

        for (const route of ["openai", "veo_json", "waninter", "yijia", "xai", "newapi", "seedance", "binghuo"] as const) {
            post.mockClear();
            const task = await createVideoGenerationTask(videoConfig(route), "test prompt");
            const [requestUrl, body] = post.mock.calls[0];
            const query = new URL(String(requestUrl)).searchParams;
            expect(query.get("routing_video_route")).toBe(route);
            expect(task).toMatchObject({ channelId: 2, channelModelId: 62 });

            if (route === "openai") {
                const headers = (post.mock.calls[0][2]?.headers || {}) as Record<string, string>;
                expect(headers["Content-Type"]).toBe("application/json");
                expect(typeof (body as { get?: unknown }).get).toBe("undefined");
                expect(body).toEqual({ model: "video-model", prompt: "test prompt", input_reference: "", size: "720x1280" });
            } else if (route === "yijia") {
                expect(typeof (body as { get?: unknown }).get).toBe("undefined");
                expect(body).toEqual({ model: "video-model", prompt: "test prompt", input_reference: "", size: "720x1280" });
                expect(body).not.toHaveProperty("seconds");
                expect(body).not.toHaveProperty("n");
                expect(body).not.toHaveProperty("resolution_name");
                expect(body).not.toHaveProperty("preset");
            } else if (route === "veo_json" || route === "xai") {
                expect(body).toMatchObject({ aspect_ratio: "9:16" });
            } else if (route === "waninter") {
                expect(body).toMatchObject({ size: "720x1280", aspect_ratio: "9:16" });
            } else if (route === "newapi") {
                expect(body).toMatchObject({ width: 720, height: 1280 });
            } else if (route === "seedance") {
                expect(body).toMatchObject({ ratio: "9:16" });
            } else {
                expect(body).toMatchObject({ ratio: "9:16", aspect_ratio: "9:16", resolution: "720P", generate_audio: false, n: 1 });
                for (const field of ["size", "width", "height", "image"]) expect(body).not.toHaveProperty(field);
            }
        }
    });

    it("uses yijia JSON when a raw model resolves to a selected yijia channel model", async () => {
        useConfigStore.getState().applyServerChannelCatalog(
            [{ id: 7, name: "Yijia", enabled: true, video_api_standard: "default" }],
            {
                7: [
                    {
                        id: 77,
                        channel_id: 7,
                        model_name: "omni_flash",
                        capabilities: ["video"],
                        enabled: true,
                        image_generate_route: "auto",
                        image_edit_route: "auto",
                        video_route: "yijia",
                        video_durations: [],
                        video_customizable: false,
                        sort_order: 0,
                    },
                ],
            },
        );
        const post = jest.spyOn(axios, "post").mockResolvedValue({ data: { id: "task_yijia" }, headers: {} });
        const config = {
            ...defaultConfig,
            model: "omni_flash",
            videoModel: "omni_flash",
            videoModels: ["omni_flash"],
            videoChannelId: 7,
            channelModelId: 77,
            size: "720x1280",
            modelRoutes: {},
        };

        await createVideoGenerationTask(config, "生成视频");

        const [requestUrl, body, requestConfig] = post.mock.calls[0];
        const query = new URL(String(requestUrl)).searchParams;
        const headers = (requestConfig?.headers || {}) as Record<string, string>;
        expect(query.get("channel_id")).toBe("7");
        expect(query.get("channel_model_id")).toBe("77");
        expect(headers["Content-Type"]).toBe("application/json");
        expect(typeof (body as { get?: unknown }).get).toBe("undefined");
        expect(body).toEqual({ model: "omni_flash", prompt: "生成视频", input_reference: "", size: "720x1280" });
        expect(body).not.toHaveProperty("seconds");
        expect(body).not.toHaveProperty("n");
        expect(body).not.toHaveProperty("resolution_name");
        expect(body).not.toHaveProperty("preset");
    });

    it("uses yijia JSON when a default-standard merge model inherits a yijia physical route", async () => {
        useConfigStore.setState({ config: { ...defaultConfig, model: "merge://7::omni_flash", videoModel: "merge://7::omni_flash", videoChannelId: 7, size: "720x1280" } });
        useConfigStore.getState().applyServerChannelCatalog(
            [{ id: 7, name: "混合渠道1", enabled: true, video_api_standard: "default" }],
            {
                7: [
                    {
                        id: 77,
                        channel_id: 7,
                        model_name: "omni_flash",
                        capabilities: ["video"],
                        enabled: true,
                        image_generate_route: "auto",
                        image_edit_route: "auto",
                        video_route: "yijia",
                        video_durations: [6],
                        video_customizable: true,
                        sort_order: 0,
                    },
                ],
            },
        );
        useConfigStore.getState().applyServerOptionMetadata([{ model: "omni_flash", credits_per_unit: 1, unit_type: "per_video" }], null);
        useConfigStore.getState().applyServerMergeGroups(7, [
            {
                id: 1,
                channel_id: 7,
                group_name: "omni_flash",
                pattern: "omni_flash",
                enabled: true,
                created_at: "",
                updated_at: "",
            },
        ]);
        const post = jest.spyOn(axios, "post").mockResolvedValue({ data: { id: "task_yijia_merge" }, headers: {} });

        await createVideoGenerationTask(useConfigStore.getState().config, "生成视频");

        const [requestUrl, body, requestConfig] = post.mock.calls[0];
        const query = new URL(String(requestUrl)).searchParams;
        const headers = (requestConfig?.headers || {}) as Record<string, string>;
        expect(query.get("channel_id")).toBe("7");
        expect(query.get("fuzzy_group_name")).toBe("omni_flash");
        expect(headers["Content-Type"]).toBe("application/json");
        expect(typeof (body as { get?: unknown }).get).toBe("undefined");
        expect(body).toEqual({ model: "omni_flash", prompt: "生成视频", input_reference: "", size: "720x1280" });
        expect(Object.hasOwn(body as object, "input_reference[]")).toBe(false);
        expect(body).not.toHaveProperty("resolution_name");
        expect(body).not.toHaveProperty("preset");
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

    it("refreshes credits when failed polling reports an async refund", async () => {
        const dispatchEvent = jest.fn();
        (globalThis.window as unknown as { dispatchEvent: typeof dispatchEvent }).dispatchEvent = dispatchEvent;
        jest.spyOn(axios, "get").mockResolvedValue({ data: { status: "failed", error: { message: "upstream failed" } }, headers: { "X-Credits-Refund": "12" } });

        const state = await pollVideoGenerationTask(videoConfig("waninter"), {
            id: "task_123",
            provider: "openai",
            model: "0::0::video-model",
            channelId: 2,
            channelModelId: 62,
        });

        expect(state).toEqual({ status: "failed", error: "upstream failed" });
        expect(dispatchEvent).toHaveBeenCalledTimes(1);
    });

    it("refreshes credits when axios headers expose refund through get()", async () => {
        const dispatchEvent = jest.fn();
        (globalThis.window as unknown as { dispatchEvent: typeof dispatchEvent }).dispatchEvent = dispatchEvent;
        jest.spyOn(axios, "get").mockResolvedValue({
            data: { status: "failed", error: { message: "upstream failed" } },
            headers: { get: (key: string) => (key.toLowerCase() === "x-credits-refund" ? "12" : undefined) },
        });

        await pollVideoGenerationTask(videoConfig("waninter"), {
            id: "task_123",
            provider: "openai",
            model: "0::0::video-model",
            channelId: 2,
            channelModelId: 62,
        });

        expect(dispatchEvent).toHaveBeenCalledTimes(1);
    });

    it("refreshes credits when polling unwraps a failed envelope with a refund header", async () => {
        const dispatchEvent = jest.fn();
        (globalThis.window as unknown as { dispatchEvent: typeof dispatchEvent }).dispatchEvent = dispatchEvent;
        jest.spyOn(axios, "get").mockResolvedValue({
            data: { code: "fail_to_fetch_task", message: "{\"error\":{\"message\":\"task failed\"}}" },
            headers: { "x-credits-refund": "8" },
        });

        await expect(
            pollVideoGenerationTask(videoConfig("binghuo"), {
                id: "task_binghuo",
                provider: "binghuo",
                model: "0::0::video-model",
                channelId: 2,
                channelModelId: 62,
            }),
        ).rejects.toThrow("task failed");
        expect(dispatchEvent).toHaveBeenCalledTimes(1);
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
