import { describe, expect, test } from "bun:test";
import { isBalanceError } from "../error-helper";

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
