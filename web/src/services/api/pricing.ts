import apiClient from "./client";
import type { ApiConfigInfo } from "./api-config";

export type PricingItem = {
    id?: number;
    model: string;
    credits_per_unit: number;
    unit_type: string;
    pricing_mode?: string;
    pricing_rule?: string;
    channel_id?: number;
};

export async function listPricing() {
    const res = await apiClient.get("/credits/pricing");
    return res.data.data as PricingItem[];
}

export async function listAdminPricing() {
    const [pricingRes, configRes] = await Promise.all([apiClient.get("/credits/pricing"), apiClient.get("/api-config")]);
    return {
        pricing: pricingRes.data.data as PricingItem[],
        apiConfig: configRes.data.data as ApiConfigInfo,
    };
}

export async function savePricing(input: PricingItem) {
    const res = await apiClient.post("/credits/pricing", input);
    return res.data.data as PricingItem;
}

export async function deletePricing(id: number) {
    const res = await apiClient.delete(`/credits/pricing/${id}`);
    return res.data;
}

export type ComparePricingResult = {
    channels: Array<{
        channel_id: number;
        channel_name: string;
        has_model: boolean;
    }>;
};

export async function comparePricing(modelName: string) {
    const res = await apiClient.get("/credits/pricing/compare", { params: { model: modelName } });
    return res.data.data as ComparePricingResult;
}

export type RechargeResult = {
    user_id: number;
    amount: number;
    balance: number;
    message: string;
};

export type AdjustMode = "add" | "deduct" | "set";

export async function rechargeCredits(input: { user_id: number; amount: number; note?: string }) {
    const res = await apiClient.post("/credits/recharge", input);
    return res.data.data as RechargeResult;
}

export type AdjustCreditsInput = { user_id: number; mode: Exclude<AdjustMode, "set">; amount: number; note?: string } | { user_id: number; mode: "set"; amount: 0; target_balance: number; note?: string };

export type AdjustmentPreview = {
    readonly valid: boolean;
    readonly text: string;
};

export function getCreditAdjustmentPreview(mode: AdjustMode, balance: number, value: number | null | undefined): AdjustmentPreview {
    if (typeof value !== "number" || !Number.isInteger(value)) return { valid: false, text: "请输入整数积分" };

    if (mode === "add") return { valid: value > 0, text: `原余额 ${balance} + 输入值 ${value} = 最终余额 ${balance + value}` };
    if (mode === "deduct") return { valid: value > 0 && value <= balance, text: `原余额 ${balance} - 输入值 ${value} = 最终余额 ${balance - value}` };
    return { valid: value >= 0 && value !== balance, text: `原余额 ${balance} 调整为目标值 ${value} = 最终余额 ${value}` };
}

export function buildCreditAdjustmentRequest(mode: AdjustMode, userId: number, value: number, note?: string): AdjustCreditsInput {
    if (mode === "set") return { user_id: userId, mode, amount: 0, target_balance: value, note };
    return { user_id: userId, mode, amount: value, note };
}

export async function adjustCredits(input: AdjustCreditsInput) {
    const res = await apiClient.post("/credits/adjust", input);
    return res.data.data as RechargeResult;
}

export type UserItem = {
    id: number;
    username: string;
    display_name: string;
    role: string;
    status: string;
};

export type UserListResult = {
    items: UserItem[];
    total: number;
    page: number;
    page_size: number;
};

export async function listUsers(page = 1, pageSize = 20) {
    const res = await apiClient.get("/users", { params: { page, page_size: pageSize } });
    return res.data.data as UserListResult;
}

export async function listAllUsers(page = 1, pageSize = 20) {
    const res = await apiClient.get("/admin/users", { params: { page, page_size: pageSize } });
    return res.data.data as UserListResult;
}
