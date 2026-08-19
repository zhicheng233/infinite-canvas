import { describe, expect, test } from "bun:test";

import { getPasswordPolicyError, getUsernamePolicyError } from "./auth-policy";

describe("username policy", () => {
    test("accepts a trimmed ASCII username", () => {
        expect(getUsernamePolicyError("  User_01-name  ")).toBeNull();
    });

    test.each(["", "   ", "bad name", "x!", "a".repeat(65), "用户名"])("rejects %p", (username: string) => {
        expect(getUsernamePolicyError(username)).not.toBeNull();
    });
});

describe("password policy", () => {
    test("accepts eight or more characters with a letter and digit", () => {
        expect(getPasswordPolicyError("Password1")).toBeNull();
    });

    test("enforces the ASCII boundary at eight characters", () => {
        expect(getPasswordPolicyError("Passwo1")).toBe("密码至少需要8个字符");
        expect(getPasswordPolicyError("Passwor1")).toBeNull();
    });

    test("counts multibyte passwords by Unicode code point", () => {
        expect(getPasswordPolicyError("a1密码")).toBe("密码至少需要8个字符");
        expect(getPasswordPolicyError("a1密码密码密码")).toBeNull();
        expect(getPasswordPolicyError("a1😀😀😀")).toBe("密码至少需要8个字符");
    });

    test.each(["Pass1", "12345678", "Password"])("rejects %p", (password: string) => {
        expect(getPasswordPolicyError(password)).not.toBeNull();
    });
});
