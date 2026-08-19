import { expect, test } from "bun:test";
import type { ReactTestRenderer } from "react-test-renderer";
import { act, create } from "react-test-renderer";

import { listUsersWithBalance, MockSearch, MockTable, resetAdminPageMocks, setCurrentUser } from "./admin-page-test-mocks";

Object.defineProperty(globalThis, "IS_REACT_ACT_ENVIRONMENT", { configurable: true, value: true });

const { default: AdminUsersPage } = await import("./users/page");

test("user list resets pagination before a server keyword search", async () => {
    resetAdminPageMocks();
    setCurrentUser(null);
    listUsersWithBalance.mockImplementation(async (page = 1, pageSize = 20, keyword = "") => ({ items: [], total: 61, page, page_size: pageSize, keyword }));
    let renderer: ReactTestRenderer | undefined;
    await act(async () => {
        renderer = create(<AdminUsersPage />);
    });
    if (!renderer) throw new Error("user list did not render");
    listUsersWithBalance.mockClear();

    await act(async () => {
        renderer?.root.findByType(MockTable).props.pagination.onChange(3, 20);
    });
    expect(listUsersWithBalance).toHaveBeenLastCalledWith(3, 20, "");
    expect(renderer.root.findByType(MockTable).props.pagination.current).toBe(3);

    await act(async () => {
        renderer?.root.findByType(MockSearch).props.onSearch("  ali  ");
    });
    expect(listUsersWithBalance).toHaveBeenLastCalledWith(1, 20, "ali");
    expect(renderer.root.findByType(MockTable).props.pagination.current).toBe(1);

    await act(async () => renderer?.unmount());
});
