import apiClient from "./client";
import type { ApiConfigInfo } from "./api-config";

export type PricingItem = {
  id?: number;
  model: string;
  credits_per_unit: number;
  unit_type: string;
};

export async function listPricing() {
  const res = await apiClient.get("/credits/pricing");
  return res.data.data as PricingItem[];
}

export async function listAdminPricing() {
  const [pricingRes, configRes] = await Promise.all([
    apiClient.get("/credits/pricing"),
    apiClient.get("/api-config"),
  ]);
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

export type RechargeResult = {
  user_id: number;
  amount: number;
  balance: number;
  message: string;
};

export async function rechargeCredits(input: {
  user_id: number;
  amount: number;
  note?: string;
}) {
  const res = await apiClient.post("/credits/recharge", input);
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
