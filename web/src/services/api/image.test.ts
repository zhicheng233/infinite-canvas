import axios from "axios";
import { afterEach, describe, expect, it, jest } from "bun:test";

import { defaultConfig, type AiConfig } from "@/stores/use-config-store";
import type { ReferenceImage } from "@/types/image";
import { requestEdit } from "./image";

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
});
