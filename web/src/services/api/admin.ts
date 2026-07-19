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

export type ModelHealthModel = {
    model: string;
    generation: string;
    failures: number;
    last_error: string;
};

export type ModelHealthRecentError = {
    id: number;
    created_at: string;
    user_id: number;
    username: string;
    display_name: string;
    generation: string;
    model: string;
    path: string;
    status_code: number;
    error_message: string;
};

export type ModelHealthSummary = {
    total_24h: number;
    total_7d: number;
    top_models: ModelHealthModel[];
    recent_errors: ModelHealthRecentError[];
};

export async function getModelHealth() {
    const res = await apiClient.get("/model-health");
    return res.data.data as ModelHealthSummary;
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
    balance_before?: number;
    balance_after: number;
    ref_type: string;
    ref_id?: string;
    note: string;
    metadata?: string;
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

export type ModelCallLogItem = {
    id: number;
    user_id: number;
    username: string;
    display_name: string;
    generation: string;
    model: string;
    method: string;
    path: string;
    status_code: number;
    error_message: string;
    error_body: string;
    created_at: string;
};

export type ModelCallLogResult = {
    items: ModelCallLogItem[];
    total: number;
    page: number;
    page_size: number;
};

export async function listModelCallLogs(params: { page?: number; pageSize?: number; userId?: number; model?: string; generation?: string; keyword?: string }) {
    const res = await apiClient.get("/model-call-logs", {
        params: {
            page: params.page || 1,
            page_size: params.pageSize || 20,
            user_id: params.userId || undefined,
            model: params.model || undefined,
            generation: params.generation || undefined,
            keyword: params.keyword || undefined,
        },
    });
    return res.data.data as ModelCallLogResult;
}
