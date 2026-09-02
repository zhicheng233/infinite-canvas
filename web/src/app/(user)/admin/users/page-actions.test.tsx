import { expect, test } from "bun:test";
import { Children } from "react";
import type { ReactTestRenderer } from "react-test-renderer";
import { act, create } from "react-test-renderer";

import type { UserWithBalance } from "@/services/api/admin";
import { deleteUser, form, listUsersWithBalance, message, MockButton, MockModalComponent, MockSearch, MockTable, resetAdminPageMocks, resetUserPassword, setCurrentUser, takeConfirmation, type MockUserListResult } from "../admin-page-test-mocks";

Object.defineProperty(globalThis, "IS_REACT_ACT_ENVIRONMENT", { configurable: true, value: true });

function deferred<T>() {
    let resolve: (value: T) => void = () => undefined;
    let reject: (reason?: unknown) => void = () => undefined;
    const promise = new Promise<T>((resolvePromise, rejectPromise) => {
        resolve = resolvePromise;
        reject = rejectPromise;
    });
    return { promise, resolve, reject };
}

const target: UserWithBalance = { id: 2, username: "target", display_name: "目标账号", role: "user", status: "active", balance: 0 };
const newerTarget: UserWithBalance = { ...target, id: 3, username: "latest-target" };

const { default: AdminUsersPage } = await import("./page");

test("user list resets another account password and refreshes the active server search", async () => {
    const renderer = await renderUsersPage();
    listUsersWithBalance.mockClear();

    await act(async () => {
        findActionButton(renderer, "重置密码").props.onClick();
    });
    const resetModal = renderer.root.findByType(MockModalComponent);
    expect(resetModal.props.open).toBe(true);

    await act(async () => {
        await resetModal.props.onOk();
    });
    expect(resetUserPassword).toHaveBeenCalledWith(2, "ResetPass2");
    expect(listUsersWithBalance).toHaveBeenLastCalledWith(1, 20, "");
    expect(message.success).toHaveBeenCalledWith("用户 target 的密码已重置");
    expect(renderer.root.findByType(MockModalComponent).props.open).toBe(false);

    await act(async () => renderer.unmount());
});

test("user list does not send a reset request after client validation fails", async () => {
    const renderer = await renderUsersPage();
    form.validateFields.mockRejectedValueOnce(new Error("两次输入的新密码不一致"));

    await act(async () => {
        findActionButton(renderer, "重置密码").props.onClick();
    });
    await act(async () => {
        await renderer.root.findByType(MockModalComponent).props.onOk();
    });
    expect(resetUserPassword).not.toHaveBeenCalled();
    expect(message.error).not.toHaveBeenCalled();

    await act(async () => renderer.unmount());
});

test("user list does not request deletion when the confirmation is cancelled", async () => {
    const renderer = await renderUsersPage();

    await act(async () => {
        findActionButton(renderer, "删除").props.onClick();
        takeConfirmation()?.onCancel?.();
    });
    expect(deleteUser).not.toHaveBeenCalled();

    await act(async () => renderer.unmount());
});

test("user list preserves the row when deletion fails", async () => {
    const renderer = await renderUsersPage();
    deleteUser.mockRejectedValueOnce(new Error("用户不存在"));
    listUsersWithBalance.mockClear();

    await act(async () => {
        findActionButton(renderer, "删除").props.onClick();
        try {
            await takeConfirmation()?.onOk();
        } catch (error) {
            if (!(error instanceof Error)) throw error;
        }
    });
    expect(message.error).toHaveBeenCalledWith("用户不存在");
    expect(listUsersWithBalance).not.toHaveBeenCalled();
    expect(renderer.root.findAllByType(MockButton)).toHaveLength(2);

    await act(async () => renderer.unmount());
});

test("user list hides account actions for the acting administrator", async () => {
    const renderer = await renderUsersPage("2");

    expect(renderer.root.findAllByType(MockButton)).toHaveLength(0);
    expect(resetUserPassword).not.toHaveBeenCalled();
    expect(deleteUser).not.toHaveBeenCalled();

    await act(async () => renderer.unmount());
});

test("user list returns to the previous server-filtered page after deleting its final row", async () => {
    const renderer = await renderUsersPage();
    await act(async () => {
        renderer.root.findByType(MockTable).props.pagination.onChange(2, 20);
    });
    listUsersWithBalance.mockClear();

    await act(async () => {
        findActionButton(renderer, "删除").props.onClick();
        await takeConfirmation()?.onOk();
    });
    expect(deleteUser).toHaveBeenCalledWith(2);
    expect(listUsersWithBalance).toHaveBeenLastCalledWith(1, 20, "");
    expect(renderer.root.findByType(MockTable).props.pagination.current).toBe(1);

    await act(async () => renderer.unmount());
});

test("user list keeps the latest search after an older page response arrives", async () => {
    const renderer = await renderUsersPage();
    const older = deferred<MockUserListResult>();
    const newer = deferred<MockUserListResult>();
    listUsersWithBalance.mockReset();
    listUsersWithBalance.mockImplementationOnce(() => older.promise).mockImplementationOnce(() => newer.promise);

    await act(async () => {
        renderer.root.findByType(MockTable).props.pagination.onChange(2, 20);
        renderer.root.findByType(MockSearch).props.onSearch(" latest ");
    });
    await act(async () => {
        newer.resolve({ items: [newerTarget], total: 1, page: 1, page_size: 20, keyword: "latest" });
        await newer.promise;
    });
    expect(renderer.root.findByType(MockTable).props.dataSource).toEqual([newerTarget]);
    expect(renderer.root.findByType(MockTable).props.pagination.current).toBe(1);

    await act(async () => {
        older.resolve({ items: [target], total: 21, page: 2, page_size: 20, keyword: "" });
        await older.promise;
    });
    expect(renderer.root.findByType(MockTable).props.dataSource).toEqual([newerTarget]);
    expect(renderer.root.findByType(MockTable).props.pagination.current).toBe(1);
    expect(renderer.root.findByType(MockTable).props.loading).toBe(false);
    await act(async () => renderer.unmount());
});

test("user list keeps loading until the newest request settles", async () => {
    const renderer = await renderUsersPage();
    const older = deferred<MockUserListResult>();
    const newer = deferred<MockUserListResult>();
    listUsersWithBalance.mockReset();
    listUsersWithBalance.mockImplementationOnce(() => older.promise).mockImplementationOnce(() => newer.promise);

    await act(async () => {
        renderer.root.findByType(MockTable).props.pagination.onChange(2, 20);
        renderer.root.findByType(MockSearch).props.onSearch(" latest ");
    });
    await act(async () => {
        older.resolve({ items: [target], total: 21, page: 2, page_size: 20, keyword: "" });
        await older.promise;
    });
    expect(renderer.root.findByType(MockTable).props.loading).toBe(true);

    await act(async () => {
        newer.resolve({ items: [newerTarget], total: 1, page: 1, page_size: 20, keyword: "latest" });
        await newer.promise;
    });
    expect(renderer.root.findByType(MockTable).props.loading).toBe(false);
    await act(async () => renderer.unmount());
});

test("user list ignores errors from an older request", async () => {
    const renderer = await renderUsersPage();
    const older = deferred<MockUserListResult>();
    const newer = deferred<MockUserListResult>();
    listUsersWithBalance.mockReset();
    listUsersWithBalance.mockImplementationOnce(() => older.promise).mockImplementationOnce(() => newer.promise);

    await act(async () => {
        renderer.root.findByType(MockTable).props.pagination.onChange(2, 20);
        renderer.root.findByType(MockSearch).props.onSearch(" latest ");
    });
    await act(async () => {
        newer.resolve({ items: [newerTarget], total: 1, page: 1, page_size: 20, keyword: "latest" });
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

test("user list falls back to the generic list error message for a non-Error rejection", async () => {
    const renderer = await renderUsersPage();
    listUsersWithBalance.mockClear();
    message.error.mockClear();
    listUsersWithBalance.mockRejectedValueOnce("network down");

    await act(async () => {
        renderer.root.findByType(MockSearch).props.onSearch(" broken ");
        await Promise.resolve();
    });

    expect(message.error).toHaveBeenCalledWith("获取用户列表失败");
    await act(async () => renderer.unmount());
});

async function renderUsersPage(userID = "1"): Promise<ReactTestRenderer> {
    resetAdminPageMocks();
    setCurrentUser({ id: userID, username: "operator", displayName: "操作管理员", avatarUrl: "", role: "tenant_admin" });
    listUsersWithBalance.mockImplementation(async (page = 1, pageSize = 20, keyword = "") => ({ items: [target], total: page === 2 ? 21 : 1, page, page_size: pageSize, keyword }));

    let renderer: ReactTestRenderer | undefined;
    await act(async () => {
        renderer = create(<AdminUsersPage />);
    });
    if (!renderer) throw new Error("user page did not render");
    return renderer;
}

function findActionButton(renderer: ReactTestRenderer, label: string) {
    const button = renderer.root.findAllByType(MockButton).find((candidate) => Children.toArray(candidate.props.children).includes(label));
    if (!button) throw new Error(`action button ${label} was not rendered`);
    return button;
}
