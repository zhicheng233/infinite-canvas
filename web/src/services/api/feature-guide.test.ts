import { afterEach, describe, expect, it, jest } from "bun:test";

import apiClient from "./client";
import { getAdminFeatureGuides, getFeatureGuide, saveAdminFeatureGuide, type FeatureGuide } from "./feature-guide";

const guide: FeatureGuide = { surface: "canvas", enabled: true, title: "画布引导", pages: ["第一页"], version: 3 };

afterEach(() => jest.restoreAllMocks());

describe("feature guide API", () => {
    it("returns a nullable guide from the exact public surface endpoint", async () => {
        const get = jest.spyOn(apiClient, "get").mockResolvedValueOnce({ data: { data: guide } }).mockResolvedValueOnce({ data: { data: null } });

        await expect(getFeatureGuide("canvas")).resolves.toEqual(guide);
        await expect(getFeatureGuide("video")).resolves.toBeNull();
        expect(get).toHaveBeenNthCalledWith(1, "/feature-guides/canvas");
        expect(get).toHaveBeenNthCalledWith(2, "/feature-guides/video");
    });

    it("lists and saves guides through the admin endpoints", async () => {
        const get = jest.spyOn(apiClient, "get").mockResolvedValue({ data: { data: [guide] } });
        const put = jest.spyOn(apiClient, "put").mockResolvedValue({ data: { data: guide } });
        const input = { enabled: true, title: "画布引导", pages: ["第一页"] };

        await expect(getAdminFeatureGuides()).resolves.toEqual([guide]);
        await expect(saveAdminFeatureGuide("canvas", input)).resolves.toEqual(guide);
        expect(get).toHaveBeenCalledWith("/admin/feature-guides");
        expect(put).toHaveBeenCalledWith("/admin/feature-guides/canvas", input);
    });
});
