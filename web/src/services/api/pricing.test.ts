import { afterEach, describe, expect, it, jest } from "bun:test";

import apiClient from "./client";
import { adjustCredits, buildCreditAdjustmentRequest, getCreditAdjustmentPreview } from "./pricing";

describe("credit adjustment preview", () => {
    it("renders live Chinese arithmetic for add, deduct, and target modes", () => {
        expect(getCreditAdjustmentPreview("add", 100, 25)).toEqual({ valid: true, text: "原余额 100 + 输入值 25 = 最终余额 125" });
        expect(getCreditAdjustmentPreview("deduct", 100, 25)).toEqual({ valid: true, text: "原余额 100 - 输入值 25 = 最终余额 75" });
        expect(getCreditAdjustmentPreview("set", 100, 60)).toEqual({ valid: true, text: "原余额 100 调整为目标值 60 = 最终余额 60" });
    });

    it("blocks empty, noninteger, no-op target, negative target, and overdraft values", () => {
        expect(getCreditAdjustmentPreview("add", 100, undefined).valid).toBe(false);
        expect(getCreditAdjustmentPreview("add", 100, 1.5).valid).toBe(false);
        expect(getCreditAdjustmentPreview("set", 100, 100).valid).toBe(false);
        expect(getCreditAdjustmentPreview("set", 100, -1).valid).toBe(false);
        expect(getCreditAdjustmentPreview("deduct", 100, 101).valid).toBe(false);
    });
});

describe("credit adjustment request", () => {
    it("builds explicit positive add and deduct request bodies", () => {
        expect(buildCreditAdjustmentRequest("add", 7, 25, "奖励")).toEqual({ user_id: 7, mode: "add", amount: 25, note: "奖励" });
        expect(buildCreditAdjustmentRequest("deduct", 7, 25, "扣减")).toEqual({ user_id: 7, mode: "deduct", amount: 25, note: "扣减" });
    });

    it("builds a target request with amount zero instead of a client-calculated delta", () => {
        expect(buildCreditAdjustmentRequest("set", 7, 60, "设定")).toEqual({ user_id: 7, mode: "set", amount: 0, target_balance: 60, note: "设定" });
    });
});

describe("credit adjustment API", () => {
    afterEach(() => jest.restoreAllMocks());

    it("forwards explicit add and deduct request bodies unchanged", async () => {
        const post = jest.spyOn(apiClient, "post").mockResolvedValue({ data: { data: { user_id: 7, amount: 25, balance: 125, message: "积分调整成功" } } });

        await adjustCredits({ user_id: 7, mode: "add", amount: 25, note: "奖励" });
        await adjustCredits({ user_id: 7, mode: "deduct", amount: 25, note: "扣减" });

        expect(post).toHaveBeenNthCalledWith(1, "/credits/adjust", { user_id: 7, mode: "add", amount: 25, note: "奖励" });
        expect(post).toHaveBeenNthCalledWith(2, "/credits/adjust", { user_id: 7, mode: "deduct", amount: 25, note: "扣减" });
    });

    it("forwards target mode with zero amount and target balance", async () => {
        const response = { user_id: 7, amount: -40, balance: 60, message: "积分调整成功" };
        const post = jest.spyOn(apiClient, "post").mockResolvedValue({ data: { data: response } });

        await expect(adjustCredits({ user_id: 7, mode: "set", amount: 0, target_balance: 60, note: "设定" })).resolves.toEqual(response);
        expect(post).toHaveBeenCalledWith("/credits/adjust", { user_id: 7, mode: "set", amount: 0, target_balance: 60, note: "设定" });
    });
});
