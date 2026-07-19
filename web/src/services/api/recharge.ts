import apiClient from "./client";

export type CreditPayout = {
    id: string;
    name: string;
    credits: number;
    price: string;
};

export type RechargeOrder = {
    id: number;
    tenant_id: number;
    user_id: number;
    amount: number;
    credits: number;
    status: string;
    payment_ref: string;
    note: string;
    created_at: string;
    updated_at: string;
};

type ApiResult<T> = { code: number; data: T; msg: string };
type PageData<T> = { items: T[]; total: number; page: number; page_size: number };

export async function listPayouts(): Promise<CreditPayout[]> {
    const res = await apiClient.get<ApiResult<CreditPayout[]>>("/recharge/payouts");
    return res.data.data || [];
}

export async function createRechargeOrder(payoutId: string): Promise<RechargeOrder> {
    const res = await apiClient.post<ApiResult<RechargeOrder>>("/recharge/order", {
        payout_id: payoutId,
    });
    return res.data.data;
}

export async function listMyOrders(page = 1, pageSize = 20): Promise<PageData<RechargeOrder>> {
    const res = await apiClient.get<ApiResult<PageData<RechargeOrder>>>("/recharge/orders", {
        params: { page, page_size: pageSize },
    });
    return res.data.data;
}
