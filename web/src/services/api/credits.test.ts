import { afterEach, describe, expect, it, jest } from "bun:test";

import apiClient from "./client";
import { estimateCost } from "./credits";

describe("credit estimate API", () => {
    afterEach(() => jest.restoreAllMocks());

    it("forwards the exact channel and channel model identity", async () => {
        const response = { total_cost: 9 };
        const get = jest.spyOn(apiClient, "get").mockResolvedValue({ data: { data: response } });

        await expect(estimateCost("same-model", { channel_id: 2, channel_model_id: 22, count: 3 })).resolves.toEqual(response);
        expect(get).toHaveBeenCalledWith("/credits/estimate", {
            params: { model: "same-model", channel_id: 2, channel_model_id: 22, count: 3 },
        });
    });
});
