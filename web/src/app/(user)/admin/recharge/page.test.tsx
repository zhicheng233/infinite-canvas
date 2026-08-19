import { afterEach, describe, expect, it } from "bun:test";
import type { ReactTestRenderer } from "react-test-renderer";
import { act, create } from "react-test-renderer";

import type { UserWithBalance } from "@/services/api/admin";
import { adjustCredits, form, listUsersWithBalance, message, MockButton, MockInputNumber, MockModalComponent, MockSearch, MockSegmented, MockTable, resetAdminPageMocks } from "../admin-page-test-mocks";

Object.defineProperty(globalThis, "IS_REACT_ACT_ENVIRONMENT", { configurable: true, value: true });

type UserListResult = { readonly items: readonly UserWithBalance[]; readonly total: number; readonly page: number; readonly page_size: number; readonly keyword: string };

function deferred<T>() {
    let resolve: (value: T) => void = () => undefined;
    let reject: (reason?: unknown) => void = () => undefined;
    const promise = new Promise<T>((resolvePromise, rejectPromise) => {
        resolve = resolvePromise;
        reject = rejectPromise;
    });
    return { promise, resolve, reject };
}

const user: UserWithBalance = { id: 7, username: "task5-user", display_name: "任务五用户", role: "user", status: "active", balance: 100 };
const newerUser: UserWithBalance = { ...user, id: 8, username: "latest-user" };

const { default: AdminRechargePage } = await import("./page");

async function renderPage() {
    resetAdminPageMocks();
    listUsersWithBalance.mockImplementation(async (page = 1, pageSize = 20, keyword = "") => ({ items: [user], total: 1, page, page_size: pageSize, keyword }));
    let renderer: ReactTestRenderer | undefined;
    await act(async () => {
        renderer = create(<AdminRechargePage />);
    });
    if (!renderer) throw new Error("recharge page did not render");
    return renderer;
}

async function openMode(renderer: ReactTestRenderer, label: string) {
    await act(async () => {
        renderer.root
            .findAllByType(MockButton)
            .find((button) => button.props.children === label)
            ?.props.onClick();
    });
}

async function setAmount(renderer: ReactTestRenderer, amount: number) {
    await act(async () => {
        renderer.root.findByType(MockInputNumber).props.onChange(amount);
    });
}

afterEach(() => resetAdminPageMocks());

describe("admin recharge page", () => {
    it("submits an explicit positive add request and retains the applied server keyword while refreshing", async () => {
        const renderer = await renderPage();
        await act(async () => {
            renderer.root.findByType(MockSearch).props.onSearch("  task5  ");
        });
        await openMode(renderer, "增加");
        await setAmount(renderer, 25);

        expect(renderer.root.findByType(MockModalComponent).props.okButtonProps).toEqual({ disabled: false });
        await act(async () => {
            await renderer.root.findByType(MockModalComponent).props.onOk();
        });
        expect(adjustCredits).toHaveBeenCalledWith({ user_id: 7, mode: "add", amount: 25, note: "测试调整" });
        expect(listUsersWithBalance).toHaveBeenLastCalledWith(1, 20, "task5");
        await act(async () => renderer.unmount());
    });

    it("submits deduct and target modes with their exact request bodies", async () => {
        const renderer = await renderPage();
        await openMode(renderer, "扣减");
        await setAmount(renderer, 25);
        await act(async () => {
            await renderer.root.findByType(MockModalComponent).props.onOk();
        });
        expect(adjustCredits).toHaveBeenLastCalledWith({ user_id: 7, mode: "deduct", amount: 25, note: "测试调整" });

        await openMode(renderer, "增加");
        await act(async () => {
            renderer.root.findByType(MockSegmented).props.onChange("set");
        });
        await setAmount(renderer, 60);
        await act(async () => {
            await renderer.root.findByType(MockModalComponent).props.onOk();
        });
        expect(adjustCredits).toHaveBeenLastCalledWith({ user_id: 7, mode: "set", amount: 0, target_balance: 60, note: "测试调整" });
        await act(async () => renderer.unmount());
    });

    it("keeps confirmation disabled for an overdraft deduction", async () => {
        const renderer = await renderPage();
        await openMode(renderer, "扣减");
        await setAmount(renderer, 101);

        expect(renderer.root.findByType(MockModalComponent).props.okButtonProps).toEqual({ disabled: true });
        await act(async () => {
            await renderer.root.findByType(MockModalComponent).props.onOk();
        });
        expect(adjustCredits).not.toHaveBeenCalled();
        await act(async () => renderer.unmount());
    });

    it("keeps the latest recharge search after an older page response arrives", async () => {
        const renderer = await renderPage();
        const older = deferred<UserListResult>();
        const newer = deferred<UserListResult>();
        listUsersWithBalance.mockReset();
        listUsersWithBalance.mockImplementationOnce(() => older.promise).mockImplementationOnce(() => newer.promise);

        await act(async () => {
            renderer.root.findByType(MockTable).props.pagination.onChange(2, 20);
            renderer.root.findByType(MockSearch).props.onSearch(" latest ");
        });
        await act(async () => {
            newer.resolve({ items: [newerUser], total: 1, page: 1, page_size: 20, keyword: "latest" });
            await newer.promise;
        });
        expect(renderer.root.findByType(MockTable).props.dataSource).toEqual([newerUser]);
        expect(renderer.root.findByType(MockTable).props.pagination.current).toBe(1);

        await act(async () => {
            older.resolve({ items: [user], total: 21, page: 2, page_size: 20, keyword: "" });
            await older.promise;
        });
        expect(renderer.root.findByType(MockTable).props.dataSource).toEqual([newerUser]);
        expect(renderer.root.findByType(MockTable).props.pagination.current).toBe(1);
        expect(renderer.root.findByType(MockTable).props.loading).toBe(false);
        await act(async () => renderer.unmount());
    });

    it("keeps recharge loading until the newest request settles", async () => {
        const renderer = await renderPage();
        const older = deferred<UserListResult>();
        const newer = deferred<UserListResult>();
        listUsersWithBalance.mockReset();
        listUsersWithBalance.mockImplementationOnce(() => older.promise).mockImplementationOnce(() => newer.promise);

        await act(async () => {
            renderer.root.findByType(MockTable).props.pagination.onChange(2, 20);
            renderer.root.findByType(MockSearch).props.onSearch(" latest ");
        });
        await act(async () => {
            older.resolve({ items: [user], total: 21, page: 2, page_size: 20, keyword: "" });
            await older.promise;
        });
        expect(renderer.root.findByType(MockTable).props.loading).toBe(true);

        await act(async () => {
            newer.resolve({ items: [newerUser], total: 1, page: 1, page_size: 20, keyword: "latest" });
            await newer.promise;
        });
        expect(renderer.root.findByType(MockTable).props.loading).toBe(false);
        await act(async () => renderer.unmount());
    });

    it("ignores errors from an older recharge request", async () => {
        const renderer = await renderPage();
        const older = deferred<UserListResult>();
        const newer = deferred<UserListResult>();
        listUsersWithBalance.mockReset();
        listUsersWithBalance.mockImplementationOnce(() => older.promise).mockImplementationOnce(() => newer.promise);

        await act(async () => {
            renderer.root.findByType(MockTable).props.pagination.onChange(2, 20);
            renderer.root.findByType(MockSearch).props.onSearch(" latest ");
        });
        await act(async () => {
            newer.resolve({ items: [newerUser], total: 1, page: 1, page_size: 20, keyword: "latest" });
            await newer.promise;
        });
        await act(async () => {
            older.reject(new Error("stale failure"));
            await older.promise.catch(() => undefined);
        });
        expect(message.error).not.toHaveBeenCalled();
        expect(renderer.root.findByType(MockTable).props.loading).toBe(false);
        await act(async () => renderer.unmount());
    });

    it("falls back to the generic recharge list error message for a non-Error rejection", async () => {
        const renderer = await renderPage();
        listUsersWithBalance.mockReset();
        listUsersWithBalance.mockRejectedValueOnce("network down");

        await act(async () => {
            renderer.root.findByType(MockSearch).props.onSearch(" broken ");
        });

        expect(message.error).toHaveBeenCalledWith("获取用户列表失败");
        await act(async () => renderer.unmount());
    });
});
