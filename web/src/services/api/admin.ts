import apiClient from "./client";

export type AdminStats = {
  total_users: number;
  total_credits_earned: number;
  total_credits_spent: number;
  total_recharged: number;
};

export async function getAdminStats() {
  const res = await apiClient.get("/stats");
  return res.data.data as AdminStats;
}

export type UserWithBalance = {
  id: number;
  username: string;
  display_name: string;
  role: string;
  status: string;
  balance: number;
};

export type UserWithBalanceResult = {
  items: UserWithBalance[];
  total: number;
  page: number;
  page_size: number;
};

export async function listUsersWithBalance(page = 1, pageSize = 20) {
  const res = await apiClient.get("/users-with-balance", {
    params: { page, page_size: pageSize },
  });
  return res.data.data as UserWithBalanceResult;
}

export type TransactionItem = {
  id: number;
  type: string;
  amount: number;
  balance_after: number;
  ref_type: string;
  note: string;
  created_at: string;
};

export type TransactionResult = {
  items: TransactionItem[];
  total: number;
  page: number;
  page_size: number;
};

export async function listTransactions(page = 1, pageSize = 20) {
  const res = await apiClient.get("/transactions", {
    params: { page, page_size: pageSize },
  });
  return res.data.data as TransactionResult;
}
