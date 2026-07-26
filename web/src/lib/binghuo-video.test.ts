import { describe, expect, test } from "bun:test";

import { binghuoRatioOptions, binghuoResolutionOptions, normalizeBinghuoRatio, normalizeBinghuoResolution } from "./binghuo-video";

describe("Binghuo video parameter normalization", () => {
    test("keeps all supported ratios", () => {
        for (const value of binghuoRatioOptions) expect(normalizeBinghuoRatio(value)).toBe(value);
        expect(normalizeBinghuoRatio("720x1280")).toBe("9:16");
        expect(normalizeBinghuoRatio("unknown")).toBe("16:9");
    });

    test("keeps all supported resolutions and selects the nearest historic value", () => {
        for (const value of binghuoResolutionOptions) expect(normalizeBinghuoResolution(value)).toBe(value);
        expect(normalizeBinghuoResolution("1024p")).toBe("1080P");
        expect(normalizeBinghuoResolution("unknown")).toBe("720P");
    });
});
