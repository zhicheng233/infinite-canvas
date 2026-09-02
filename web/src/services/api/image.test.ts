import axios from "axios";
import { afterEach, describe, expect, it, jest } from "bun:test";

import { defaultConfig, type AiConfig } from "@/stores/use-config-store";
import type { ReferenceImage } from "@/types/image";
import { requestEdit, requestGeneration } from "./image";

const referenceImage: ReferenceImage = {
    id: "reference-1",
    name: "reference.png",
    type: "image/png",
    dataUrl: "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=",
};

describe("requestEdit image route", () => {
    afterEach(() => jest.restoreAllMocks());

    it("uses the configured chat route for an encoded physical channel identity", async () => {
        // Given: a physical model selection and an edit route keyed by its encoded identity.
        const encodedModel = "7::42::image-model";
        const config: AiConfig = {
            ...defaultConfig,
            model: encodedModel,
            imageModel: encodedModel,
            imageModels: [encodedModel],
            imageChannelId: 7,
            modelRoutes: { [`image_edit:${encodedModel}`]: "chat" },
        };
        const post = jest.spyOn(axios, "post").mockResolvedValue({
            data: {
                data: [{ b64_json: referenceImage.dataUrl }],
                choices: [{ message: { content: `![result](${referenceImage.dataUrl})` } }],
            },
            headers: {},
        });

        // When: the image edit request is sent with a valid reference image.
        await requestEdit(config, "调整图片", [referenceImage]);

        // Then: the outbound request uses the chat completions endpoint.
        const requestUrl = String(post.mock.calls[0][0]);
        expect(requestUrl.endsWith("/chat/completions")).toBe(true);
        expect(requestUrl.endsWith("/images/edits")).toBe(false);
    });

    it("sends one reference as a JSON scalar to an explicit generations route", async () => {
        const config: AiConfig = {
            ...defaultConfig,
            model: "gpt-image-2",
            imageModel: "gpt-image-2",
            modelRoutes: { "image_edit:gpt-image-2": "generations" },
        };
        const post = jest.spyOn(axios, "post").mockResolvedValue({ data: { data: [{ b64_json: referenceImage.dataUrl }] }, headers: {} });

        await requestEdit(config, "调整图片", [referenceImage]);

        expect(String(post.mock.calls[0][0]).endsWith("/images/generations")).toBe(true);
        expect(post.mock.calls[0][1]).toMatchObject({ model: "gpt-image-2", image: referenceImage.dataUrl });
    });

    it("sends multiple references as a JSON array to an explicit generations route", async () => {
        const config: AiConfig = {
            ...defaultConfig,
            model: "image-model",
            imageModel: "image-model",
            modelRoutes: { "image_edit:image-model": "generations" },
        };
        const post = jest.spyOn(axios, "post").mockResolvedValue({ data: { data: [{ b64_json: referenceImage.dataUrl }] }, headers: {} });

        await requestEdit(config, "融合图片", [referenceImage, { ...referenceImage, id: "reference-2", name: "reference-2.png" }]);

        expect(post.mock.calls[0][1]).toMatchObject({ image: [referenceImage.dataUrl, referenceImage.dataUrl] });
    });

    it("keeps resolved routing and settlement metadata on generated images", async () => {
        jest.spyOn(axios, "post").mockResolvedValue({
            data: { data: [{ b64_json: referenceImage.dataUrl }] },
            headers: { "x-generation-request-id": "request-1", "x-credits-cost": "7", "x-resolved-channel-name": "渠道 4" },
        });

        const [image] = await requestGeneration({ ...defaultConfig, model: "gpt-image-2", imageModel: "gpt-image-2" }, "test");

        expect(image).toMatchObject({ generationRequestId: "request-1", generationCost: 7, resolvedChannelName: "渠道 4" });
    });

    it("does not invent zero-cost metadata when the response header is absent", async () => {
        jest.spyOn(axios, "post").mockResolvedValue({ data: { data: [{ b64_json: referenceImage.dataUrl }] }, headers: {} });

        const [image] = await requestGeneration({ ...defaultConfig, model: "gpt-image-2", imageModel: "gpt-image-2" }, "test");

        expect(image).not.toHaveProperty("generationCost");
    });
});
