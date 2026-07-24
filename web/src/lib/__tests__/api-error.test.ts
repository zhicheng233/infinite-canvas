import { describe, expect, test } from "bun:test";
import { ApiError } from "../api-error";

describe("ApiError", () => {
  test("has correct message", () => {
    const err = new ApiError("test message");
    expect(err.message).toBe("test message");
  });

  test("has errorDetail when provided", () => {
    const err = new ApiError("msg", "raw upstream error");
    expect(err.errorDetail).toBe("raw upstream error");
  });

  test("has undefined errorDetail when not provided", () => {
    const err = new ApiError("msg");
    expect(err.errorDetail).toBeUndefined();
  });

  test("is instanceof Error", () => {
    const err = new ApiError("msg");
    expect(err instanceof Error).toBe(true);
  });

  test("name is ApiError", () => {
    const err = new ApiError("msg");
    expect(err.name).toBe("ApiError");
  });
});
