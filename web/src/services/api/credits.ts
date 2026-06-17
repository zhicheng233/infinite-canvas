import apiClient from "./client";

type BalanceData = { balance: number; total_earned: number; total_spent: number };
type PricingData = { model: string; credits_per_unit: number; unit_type: string };
type TransactionItem = { id: number; type: string; amount: number; balance_after: number; ref_type: string; note: string; created_at: string };
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

export async function estimateCost(model: string) {
  const res = await apiClient.get<ApiResult<PricingData>>("/credits/estimate", { params: { model } });
  return res.data.data;
}