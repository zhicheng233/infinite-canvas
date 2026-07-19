import apiClient from "./client";

type BalanceData = { balance: number; total_earned: number; total_spent: number };
type PricingData = {
    model: string;
    credits_per_unit: number;
    unit_type: string;
    pricing_mode?: string;
    pricing_rule?: string;
    total_cost?: number;
    unit_cost?: number;
    units?: number;
    seconds?: number;
    resolution?: string;
    formula?: string;
};
export type CreditTransactionMetadata = {
    scene?: string;
    generation?: string;
    model?: string;
    path?: string;
    unit_type?: string;
    unit_label?: string;
    unit_cost?: number;
    units?: number;
    seconds?: number;
    resolution?: string;
    formula?: string;
    total_cost?: number;
    operator_user_id?: number;
    target_user_id?: number;
    recharge_order_id?: number;
    credits?: number;
    adjustment?: number;
};
export type TransactionItem = { id: number; type: string; amount: number; balance_before?: number; balance_after: number; ref_type: string; ref_id?: string; note: string; metadata?: string; created_at: string };
type PageData<T> = { items: T[]; total: number; page: number; page_size: number };
type ApiResult<T> = { code: number; data: T; msg: string };

export async function getBalance() {
    const res = await apiClient.get<ApiResult<BalanceData>>("/credits/balance");
    return res.data.data;
}

export async function getTransactions(page = 1, pageSize = 20) {
    const res = await apiClient.get<ApiResult<PageData<TransactionItem>>>("/credits/transactions", { params: { page, page_size: pageSize } });
    return res.data.data;
}

export async function estimateCost(model: string, params?: { type?: string; count?: string | number; seconds?: string | number; resolution?: string; size?: string }) {
    const res = await apiClient.get<ApiResult<PricingData>>("/credits/estimate", { params: { model, ...params } });
    return res.data.data;
}
