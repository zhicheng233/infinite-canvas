import { describe, expect, test } from "bun:test";
import { formatVideoGenerationError, isBalanceError } from "../error-helper";
import { ApiError } from "../api-error";

describe("isBalanceError", () => {
    test('returns true for "积分不足，需要 10 积分，当前余额 5"', () => {
        expect(isBalanceError("积分不足，需要 10 积分，当前余额 5")).toBe(true);
    });

    test('returns true for "渠道 XX 因上游余额不足已被自动禁用"', () => {
        expect(isBalanceError("渠道 XX 因上游余额不足已被自动禁用")).toBe(true);
    });

    test('returns true for "insufficient balance"', () => {
        expect(isBalanceError("insufficient balance")).toBe(true);
    });

    test('returns true for "quota exceeded"', () => {
        expect(isBalanceError("quota exceeded")).toBe(true);
    });

    test('returns true for "billing failed"', () => {
        expect(isBalanceError("billing failed")).toBe(true);
    });

    test('returns true for "扣费额度失败"', () => {
        expect(isBalanceError("扣费额度失败")).toBe(true);
    });

    test('returns false for "Rate limit exceeded"', () => {
        expect(isBalanceError("Rate limit exceeded")).toBe(false);
    });

    test('returns false for "Content filter triggered"', () => {
        expect(isBalanceError("Content filter triggered")).toBe(false);
    });

    test("returns false for empty string", () => {
        expect(isBalanceError("")).toBe(false);
    });

    test("returns false for null/undefined message", () => {
        // @ts-expect-error testing edge case with non-string
        expect(isBalanceError(null)).toBe(false);
        // @ts-expect-error testing edge case with non-string
        expect(isBalanceError(undefined)).toBe(false);
    });

    test("is case insensitive", () => {
        expect(isBalanceError("INSUFFICIENT BALANCE")).toBe(true);
        expect(isBalanceError("Quota Exceeded")).toBe(true);
        expect(isBalanceError("INSUFFICIENT_QUOTA")).toBe(true);
    });
});

describe("formatVideoGenerationError", () => {
    test("shows upstream image-to-video reference guidance", () => {
        const formatted = formatVideoGenerationError(new Error(`{"ok":false,"error":"视频生成失败：该模型为图生视频，必须提供 1 张参考图（reference_images / image）。"}`));
        expect(formatted.message).toContain("视频生成失败：该模型为图生视频，必须提供 1 张参考图");
        expect(formatted.message).toContain("添加 1 张参考图后重试");
    });

    test("shows proxied upstream image-to-video reference guidance", () => {
        const formatted = formatVideoGenerationError(new ApiError("上游请求失败", `{"ok":false,"error":"视频生成失败：该模型为图生视频，必须提供 1 张参考图（reference_images / image）。"}`));
        expect(formatted.message).toContain("视频生成失败：该模型为图生视频，必须提供 1 张参考图");
        expect(formatted.message).toContain("添加 1 张参考图后重试");
        expect(formatted.rawDetail).toBeUndefined();
    });

    test("shows upstream route unavailable guidance", () => {
        const formatted = formatVideoGenerationError(new Error(`{"ok":false,"error":"视频生成失败：该模型线路暂时不可用，请稍后重试或改用其它模型。"}`));
        expect(formatted.message).toContain("视频生成失败：该模型线路暂时不可用");
        expect(formatted.message).toContain("切换其它模型/渠道");
    });

    test("hides upstream balance details", () => {
        const formatted = formatVideoGenerationError(new ApiError("上游请求失败", `{"error":{"message":"用户额度不足, 剩余额度: ¥0.000000","code":"insufficient_user_quota"}}`));
        expect(formatted.message).toBe("因上游问题被禁用");
        expect(formatted.message).not.toContain("用户额度不足");
        expect(formatted.message).not.toContain("¥0.000000");
        expect(formatted.rawDetail).toBeUndefined();
    });

    test("extracts nested json error detail", () => {
        const formatted = formatVideoGenerationError(new ApiError("上游请求失败", `{"message":"{\\"error\\":\\"视频生成失败：该模型线路暂时不可用，请稍后重试或改用其它模型。\\"}"}`));
        expect(formatted.message).toContain("视频生成失败：该模型线路暂时不可用");
    });
});
