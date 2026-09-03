import { describe, expect, it } from "bun:test";

import type { FeatureGuide } from "@/services/api/feature-guide";
import { getFeatureGuideCompletionKey, getFeatureGuideSurface, shouldPresentFeatureGuide } from "./feature-guide-state";

const guide: FeatureGuide = { surface: "canvas", enabled: true, title: "画布引导", pages: ["内容"], version: 4 };

describe("feature guide entry state", () => {
    it("maps only editor canvas routes and exact image or video routes", () => {
        expect(getFeatureGuideSurface("/canvas")).toBeNull();
        expect(getFeatureGuideSurface("/image")).toBe("image");
        expect(getFeatureGuideSurface("/video")).toBe("video");
        expect(getFeatureGuideSurface("/canvas/project-id")).toBe("canvas");
        expect(getFeatureGuideSurface("/canvas/")).toBeNull();
        expect(getFeatureGuideSurface("/canvas/project-id/extra")).toBeNull();
        expect(getFeatureGuideSurface("/image/extra")).toBeNull();
        expect(getFeatureGuideSurface("/video/")).toBeNull();
        expect(getFeatureGuideSurface("/admin/feature-guides")).toBeNull();
    });

    it("isolates completion keys by account and surface", () => {
        expect(getFeatureGuideCompletionKey("user-7", "image")).toBe("infinite-canvas:feature-guide:user-7:image:completed-version");
        expect(getFeatureGuideCompletionKey("user-8", "image")).not.toBe(getFeatureGuideCompletionKey("user-7", "image"));
        expect(getFeatureGuideCompletionKey("user-7", "video")).not.toBe(getFeatureGuideCompletionKey("user-7", "image"));
    });

    it("presents only enabled, non-empty, unread versions", () => {
        expect(shouldPresentFeatureGuide(guide, null)).toBe(true);
        expect(shouldPresentFeatureGuide(guide, "4")).toBe(false);
        expect(shouldPresentFeatureGuide({ ...guide, enabled: false }, null)).toBe(false);
        expect(shouldPresentFeatureGuide({ ...guide, pages: ["  \n"] }, null)).toBe(false);
        expect(shouldPresentFeatureGuide(null, null)).toBe(false);
    });
});
