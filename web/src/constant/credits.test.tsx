import { afterAll, beforeEach, describe, expect, it, jest } from "bun:test";
import type { ReactTestRenderer } from "react-test-renderer";
import { act, create } from "react-test-renderer";
import { renderToStaticMarkup } from "react-dom/server";

import * as creditsApi from "@/services/api/credits";

type EstimateResponse = { total_cost?: number; credits_per_unit?: number; unit_type?: string };

function deferred<T>() {
    let resolve!: (value: T) => void;
    let reject!: (reason?: unknown) => void;
    const promise = new Promise<T>((resolvePromise, rejectPromise) => {
        resolve = resolvePromise;
        reject = rejectPromise;
    });
    return { promise, resolve, reject };
}

const estimateRequests: Array<ReturnType<typeof deferred<EstimateResponse>>> = [];
const estimateCost = jest.spyOn(creditsApi, "estimateCost").mockImplementation(() => {
    const request = deferred<EstimateResponse>();
    estimateRequests.push(request);
    return request.promise as ReturnType<typeof creditsApi.estimateCost>;
});

const { buildCreditEstimateRequest, creditEstimateRequestKey, CreditCostHint, creditEstimateButtonText, resolveCreditEstimate, useEstimatedCreditCost } = await import("./credits");

const storage = new Map<string, string>([["infinite-canvas:auth_token", "token"]]);
Object.defineProperty(globalThis, "window", {
    configurable: true,
    value: {
        localStorage: {
            getItem: (key: string) => storage.get(key) ?? null,
            setItem: (key: string, value: string) => storage.set(key, value),
            removeItem: (key: string) => storage.delete(key),
        },
    },
});
Object.defineProperty(globalThis, "IS_REACT_ACT_ENVIRONMENT", { configurable: true, value: true });

afterAll(() => {
    estimateCost.mockRestore();
    Reflect.deleteProperty(globalThis, "window");
    Reflect.deleteProperty(globalThis, "IS_REACT_ACT_ENVIRONMENT");
});

function EstimateProbe({ model, count = 1 }: { model: string; count?: number }) {
    const estimate = useEstimatedCreditCost(model, count, { type: "video", seconds: 5, resolution: "720p", size: "16:9" });
    return <span>{`${estimate.status}:${estimate.credits}`}</span>;
}

beforeEach(() => {
    estimateCost.mockClear();
    estimateRequests.length = 0;
});

describe("credit estimate contract", () => {
    it("sends both IDs for an encoded normal model", () => {
        expect(buildCreditEstimateRequest("2::22::same-model", 3)).toEqual({
            model: "same-model",
            params: { count: 3, channel_id: 2, channel_model_id: 22 },
        });
    });

    it("handles Auto and merge selections explicitly", () => {
        expect(buildCreditEstimateRequest("0::0::same-model", 1)).toEqual({
            model: "same-model",
            params: { count: 1, channel_id: 0 },
        });
        expect(buildCreditEstimateRequest("merge://7::gpt-4o", 1)).toEqual({
            model: "gpt-4o",
            params: { count: 1, channel_id: 7, fuzzy_group_name: "gpt-4o" },
        });
    });

    it("keys every normalized routing and generation parameter", () => {
        const base = buildCreditEstimateRequest("2::22::same-model", 2, { type: "video", seconds: 5, resolution: "720p", size: "16:9" });
        const keys = [
            base,
            buildCreditEstimateRequest("2::22::other-model", 2, { type: "video", seconds: 5, resolution: "720p", size: "16:9" }),
            buildCreditEstimateRequest("3::22::same-model", 2, { type: "video", seconds: 5, resolution: "720p", size: "16:9" }),
            buildCreditEstimateRequest("2::23::same-model", 2, { type: "video", seconds: 5, resolution: "720p", size: "16:9" }),
            buildCreditEstimateRequest("merge://2::same-model", 2, { type: "video", seconds: 5, resolution: "720p", size: "16:9" }),
            buildCreditEstimateRequest("2::22::same-model", 3, { type: "video", seconds: 5, resolution: "720p", size: "16:9" }),
            buildCreditEstimateRequest("2::22::same-model", 2, { type: "video", seconds: 6, resolution: "720p", size: "16:9" }),
            buildCreditEstimateRequest("2::22::same-model", 2, { type: "video", seconds: 5, resolution: "1080p", size: "16:9" }),
            buildCreditEstimateRequest("2::22::same-model", 2, { type: "video", seconds: 5, resolution: "720p", size: "9:16" }),
            buildCreditEstimateRequest("2::22::same-model", 2, { type: "image", seconds: 5, resolution: "720p", size: "16:9" }),
        ].map(creditEstimateRequestKey);

        expect(new Set(keys).size).toBe(keys.length);
    });

    it("preserves a nonzero channel-scoped total cost", () => {
        expect(resolveCreditEstimate({ total_cost: 12, credits_per_unit: 4, unit_type: "per_image" }, 3)).toEqual({ status: "ready", credits: 12 });
    });

    it("does not display request failures as zero or missing pricing", () => {
        const estimate = { status: "error", credits: 0 } as const;
        const markup = renderToStaticMarkup(<CreditCostHint estimate={estimate} balance={null} compact />);

        expect(markup).toContain("计费预估失败");
        expect(markup).not.toContain("未配置");
        expect(creditEstimateButtonText(estimate)).toBe("计费预估失败");
        expect(creditEstimateButtonText(estimate)).not.toContain("0");
    });

    it("keeps genuine missing pricing distinct from request failures", () => {
        const estimate = { status: "missing", credits: 0 } as const;
        const markup = renderToStaticMarkup(<CreditCostHint estimate={estimate} balance={null} compact />);

        expect(markup).toContain("未配置计费");
        expect(markup).not.toContain("预估失败");
    });
});

describe("useEstimatedCreditCost request coordination", () => {
    it("shares an identical in-flight request between mounted consumers", async () => {
        let root!: ReactTestRenderer;
        await act(async () => {
            root = create(
                <>
                    <EstimateProbe model="2::22::same-model" count={2} />
                    <EstimateProbe model="2::22::same-model" count={2} />
                </>,
            );
        });

        expect(estimateCost).toHaveBeenCalledTimes(1);
        expect(estimateCost).toHaveBeenCalledWith("same-model", {
            channel_id: 2,
            channel_model_id: 22,
            count: 2,
            resolution: "720p",
            seconds: 5,
            size: "16:9",
            type: "video",
        });

        await act(async () => {
            estimateRequests[0].resolve({ total_cost: 8 });
            await estimateRequests[0].promise;
        });
        expect(root.toJSON()).toEqual([
            { type: "span", props: {}, children: ["ready:8"] },
            { type: "span", props: {}, children: ["ready:8"] },
        ]);
        await act(async () => root.unmount());
    });

    it("does not let an older response overwrite a newer selection", async () => {
        let root!: ReactTestRenderer;
        await act(async () => {
            root = create(<EstimateProbe model="2::21::model-a" />);
        });
        await act(async () => {
            root.update(<EstimateProbe model="2::22::model-b" />);
        });

        expect(estimateCost).toHaveBeenCalledTimes(2);
        await act(async () => {
            estimateRequests[1].resolve({ total_cost: 22 });
            await estimateRequests[1].promise;
        });
        expect(root.toJSON()).toEqual({ type: "span", props: {}, children: ["ready:22"] });

        await act(async () => {
            estimateRequests[0].resolve({ total_cost: 11 });
            await estimateRequests[0].promise;
        });
        expect(root.toJSON()).toEqual({ type: "span", props: {}, children: ["ready:22"] });
        await act(async () => root.unmount());
    });
});
