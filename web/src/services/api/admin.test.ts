import { afterEach, describe, expect, it, jest } from "bun:test";

import apiClient from "./client";
import { deleteUser, listUsersWithBalance, resetUserPassword } from "./admin";

describe("tenant user balance list API", () => {
    afterEach(() => jest.restoreAllMocks());

    it("forwards a trimmed server-side keyword with pagination", async () => {
        const response = { items: [], total: 0, page: 3, page_size: 50 };
        const get = jest.spyOn(apiClient, "get").mockResolvedValue({ data: { data: response } });

        await expect(listUsersWithBalance(3, 50, "  ali  ")).resolves.toEqual(response);
        expect(get).toHaveBeenCalledWith("/users-with-balance", {
            params: { page: 3, page_size: 50, keyword: "ali" },
        });
    });

    it("keeps the existing request contract for a blank keyword", async () => {
        const response = { items: [], total: 0, page: 1, page_size: 20 };
        const get = jest.spyOn(apiClient, "get").mockResolvedValue({ data: { data: response } });

        await expect(listUsersWithBalance(1, 20, "   ")).resolves.toEqual(response);
        expect(get).toHaveBeenCalledWith("/users-with-balance", {
            params: { page: 1, page_size: 20 },
        });
    });
});

describe("administrator account lifecycle API", () => {
    afterEach(() => jest.restoreAllMocks());

    it("sends a new password only to the target user reset endpoint", async () => {
        const put = jest.spyOn(apiClient, "put").mockResolvedValue({ data: { data: undefined } });

        await expect(resetUserPassword(42, "ResetPass2")).resolves.toBeUndefined();
        expect(put).toHaveBeenCalledWith("/users/42/password", { new_password: "ResetPass2" });
    });

    it("deletes the requested user account", async () => {
        const response = { deleted: true };
        const remove = jest.spyOn(apiClient, "delete").mockResolvedValue({ data: { data: response } });

        await expect(deleteUser(42)).resolves.toEqual(response);
        expect(remove).toHaveBeenCalledWith("/users/42");
    });
});
