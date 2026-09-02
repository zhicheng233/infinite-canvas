import { jest, mock } from "bun:test";
import { createElement, type ReactNode } from "react";

import type { ColumnsType } from "antd/es/table";
import type { UserWithBalance, UserWithBalanceResult } from "@/services/api/admin";

type MockProps = Readonly<Record<string, unknown>> & { readonly children?: ReactNode };
type Confirmation = { readonly onCancel?: () => void; readonly onOk: () => Promise<void> };
type TableProps = MockProps & { readonly columns?: ColumnsType<UserWithBalance>; readonly dataSource?: readonly UserWithBalance[] };

export function MockComponent({ children }: MockProps) {
    return createElement("mock-component", null, children);
}

export function MockButton(props: MockProps) {
    return createElement("admin-button", props, props.children);
}

export function MockModalComponent({ children, ...props }: MockProps) {
    return createElement("admin-modal", props, children);
}

export function MockSearch(props: MockProps) {
    return createElement("admin-search", props);
}

export function MockInputNumber(props: MockProps) {
    return createElement("admin-input-number", props);
}

export function MockSegmented(props: MockProps) {
    return createElement("admin-segmented", props);
}

export function MockTable({ columns = [], dataSource = [], ...props }: TableProps) {
    const actionColumn = columns.find((column) => column.key === "actions");
    return createElement(
        "admin-table",
        props,
        dataSource.map((record) => {
            const rendered = actionColumn?.render?.(undefined, record, 0);
            const children = rendered && typeof rendered === "object" && "children" in rendered ? rendered.children : rendered;
            return createElement("admin-actions", { key: record.id }, children as ReactNode);
        }),
    );
}

export const message = { error: jest.fn(), success: jest.fn() };
export const form = {
    resetFields: jest.fn(),
    validateFields: jest.fn(async () => ({ new_password: "ResetPass2", confirm_password: "ResetPass2", note: "测试调整" })),
};
export type MockUserListResult = UserWithBalanceResult & { keyword: string };
export const listUsersWithBalance = jest.fn(async (page = 1, pageSize = 20, keyword = ""): Promise<MockUserListResult> => ({ items: [], total: 0, page, page_size: pageSize, keyword }));
export const adjustCredits = jest.fn(async () => ({ user_id: 7, amount: 0, balance: 100, message: "积分调整成功" }));
export const resetUserPassword = jest.fn(async () => undefined);
export const deleteUser = jest.fn(async () => ({ deleted: true }));

let capturedConfirmation: Confirmation | undefined;
let currentUser: { readonly id: string; readonly username: string; readonly displayName: string; readonly avatarUrl: string; readonly role: string } | null = null;

export const MockForm = Object.assign(MockComponent, { Item: MockComponent, useForm: () => [form] });
export const MockInput = Object.assign(MockComponent, { Password: MockComponent, Search: MockSearch, TextArea: MockComponent });
export const MockModal = Object.assign(MockModalComponent, {
    confirm: (confirmation: Confirmation) => {
        capturedConfirmation = confirmation;
    },
});

mock.module("antd", () => ({
    App: { useApp: () => ({ message, modal: MockModal }) },
    Button: MockButton,
    Form: MockForm,
    Input: MockInput,
    InputNumber: MockInputNumber,
    Modal: MockModal,
    Segmented: MockSegmented,
    Table: MockTable,
    Tag: MockComponent,
    Typography: { Title: MockComponent },
}));
mock.module("@/services/api/admin", () => ({ listUsersWithBalance, resetUserPassword, deleteUser }));
mock.module("@/services/api/pricing", () => ({
    adjustCredits,
    buildCreditAdjustmentRequest: (mode: "add" | "deduct" | "set", userId: number, value: number, note?: string) => (mode === "set" ? { user_id: userId, mode, amount: 0, target_balance: value, note } : { user_id: userId, mode, amount: value, note }),
    getCreditAdjustmentPreview: (mode: "add" | "deduct" | "set", balance: number, value: number | null) => {
        if (typeof value !== "number" || !Number.isInteger(value)) return { valid: false, text: "请输入整数积分" };
        if (mode === "add") return { valid: value > 0, text: `原余额 ${balance} + 输入值 ${value} = 最终余额 ${balance + value}` };
        if (mode === "deduct") return { valid: value > 0 && value <= balance, text: `原余额 ${balance} - 输入值 ${value} = 最终余额 ${balance - value}` };
        return { valid: value >= 0 && value !== balance, text: `原余额 ${balance} 调整为目标值 ${value} = 最终余额 ${value}` };
    },
}));
mock.module("@/stores/use-user-store", () => ({
    useUserStore: <T,>(selector: (state: { readonly user: typeof currentUser }) => T) => selector({ user: currentUser }),
}));

export function setCurrentUser(user: typeof currentUser) {
    currentUser = user;
}

export function takeConfirmation() {
    return capturedConfirmation;
}

export function resetAdminPageMocks() {
    capturedConfirmation = undefined;
    currentUser = null;
    message.error.mockClear();
    message.success.mockClear();
    form.resetFields.mockClear();
    form.validateFields.mockReset();
    form.validateFields.mockResolvedValue({ new_password: "ResetPass2", confirm_password: "ResetPass2", note: "测试调整" });
    listUsersWithBalance.mockReset();
    adjustCredits.mockClear();
    resetUserPassword.mockClear();
    deleteUser.mockClear();
}
