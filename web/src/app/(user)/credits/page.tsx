"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { Card, Spin, Table, Tag, Button } from "antd";
import { ArrowDownLeft, ArrowUpRight, ReceiptText, WalletCards, Zap } from "lucide-react";

import { getBalance, getTransactions } from "@/services/api/credits";
import { CreditTransactionDetailButton } from "@/components/credits/credit-transaction-detail-button";
import { creditTransactionModel } from "@/lib/credit-display";
import { cn } from "@/lib/utils";

type CreditTransaction = {
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

export default function CreditsPage() {
    const [loading, setLoading] = useState(true);
    const [transactionsLoading, setTransactionsLoading] = useState(false);
    const [balance, setBalance] = useState(0);
    const [totalEarned, setTotalEarned] = useState(0);
    const [totalSpent, setTotalSpent] = useState(0);
    const [transactions, setTransactions] = useState<CreditTransaction[]>([]);
    const [page, setPage] = useState(1);
    const [pageSize, setPageSize] = useState(20);
    const [total, setTotal] = useState(0);

    const loadSummary = useCallback(async () => {
        setLoading(true);
        try {
            const data = await getBalance();
            setBalance(data.balance);
            setTotalEarned(data.total_earned);
            setTotalSpent(data.total_spent);
        } finally {
            setLoading(false);
        }
    }, []);

    const loadTransactions = useCallback(
        async (nextPage = page, nextPageSize = pageSize) => {
            setTransactionsLoading(true);
            try {
                const data = await getTransactions(nextPage, nextPageSize);
                setTransactions(data.items || []);
                setTotal(data.total || 0);
                setPage(data.page || nextPage);
                setPageSize(data.page_size || nextPageSize);
            } finally {
                setTransactionsLoading(false);
            }
        },
        [page, pageSize],
    );

    useEffect(() => {
        void loadSummary();
    }, [loadSummary]);

    useEffect(() => {
        void loadTransactions(page, pageSize);
    }, [loadTransactions, page, pageSize]);

    const transactionTypeConfig: Record<string, { color: string; label: string }> = {
        earn: { color: "success", label: "收入" },
        spend: { color: "error", label: "支出" },
        refund: { color: "processing", label: "退款" },
        adjust: { color: "warning", label: "调整" },
    };

    const refTypeLabel = (value: string) => {
        if (value === "image") return "图片生成";
        if (value === "video") return "视频生成";
        if (value === "audio") return "音频生成";
        if (value === "text") return "文本生成";
        if (value === "recharge") return "充值";
        return value || "-";
    };

    const columns = [
        {
            title: "类型",
            dataIndex: "type",
            key: "type",
            width: 100,
            render: (value: string) => {
                const cfg = transactionTypeConfig[value] || { color: "default", label: value || "-" };
                return <Tag color={cfg.color}>{cfg.label}</Tag>;
            },
        },
        {
            title: "变动",
            dataIndex: "amount",
            key: "amount",
            width: 110,
            render: (value: number, record: CreditTransaction) => {
                const positive = record.type === "earn" || record.type === "refund";
                return (
                    <span className={cn("inline-flex items-center gap-1 font-mono font-semibold", positive ? "text-emerald-600 dark:text-emerald-400" : "text-rose-600 dark:text-rose-400")}>
                        {positive ? <ArrowDownLeft className="size-3.5" /> : <ArrowUpRight className="size-3.5" />}
                        {positive ? "+" : "-"}
                        {Math.abs(value)}
                    </span>
                );
            },
        },
        {
            title: "余额",
            key: "balance",
            width: 150,
            render: (_: unknown, record: CreditTransaction) => (
                <span className="font-mono">
                    {typeof record.balance_before === "number" ? `${record.balance_before} → ` : ""}
                    {record.balance_after}
                </span>
            ),
        },
        {
            title: "来源",
            dataIndex: "ref_type",
            key: "ref_type",
            width: 120,
            render: (value: string) => refTypeLabel(value),
        },
        {
            title: "模型",
            key: "model",
            width: 190,
            ellipsis: true,
            render: (_: unknown, record: CreditTransaction) => creditTransactionModel(record),
        },
        {
            title: "详情",
            key: "detail",
            width: 360,
            render: (_: unknown, record: CreditTransaction) => <CreditTransactionDetailButton record={record} />,
        },
        {
            title: "时间",
            dataIndex: "created_at",
            key: "created_at",
            width: 180,
            render: (value: string) => new Date(value).toLocaleString("zh-CN"),
        },
    ];

    if (loading) {
        return (
            <main className="flex h-full items-center justify-center">
                <Spin size="large" />
            </main>
        );
    }

    return (
        <main className="mx-auto max-w-6xl overflow-y-auto px-6 py-8">
            <div className="mb-8 flex flex-wrap items-center justify-between gap-4">
                <div>
                    <h1 className="text-2xl font-semibold text-stone-950 dark:text-stone-100">积分明细</h1>
                    <p className="mt-1 text-sm text-stone-500 dark:text-stone-400">查看当前余额、累计收支和全部积分流水</p>
                </div>
                <div className="flex items-center gap-3">
                    <Link href="/recharge">
                        <Button>前往充值</Button>
                    </Link>
                </div>
            </div>

            <div className="mb-8 grid gap-4 md:grid-cols-3">
                <Card>
                    <div className="flex items-center gap-2 text-sm text-stone-500 dark:text-stone-400">
                        <WalletCards className="size-4" />
                        当前积分
                    </div>
                    <div className="mt-3 inline-flex items-center gap-2 text-2xl font-semibold text-stone-950 dark:text-stone-100">
                        <Zap className="size-5 text-amber-500" />
                        {balance}
                    </div>
                </Card>
                <Card>
                    <div className="text-sm text-stone-500 dark:text-stone-400">累计收入</div>
                    <div className="mt-3 inline-flex items-center gap-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-400">
                        <ArrowDownLeft className="size-5" />
                        {totalEarned}
                    </div>
                </Card>
                <Card>
                    <div className="text-sm text-stone-500 dark:text-stone-400">累计支出</div>
                    <div className="mt-3 inline-flex items-center gap-2 text-2xl font-semibold text-rose-600 dark:text-rose-400">
                        <ArrowUpRight className="size-5" />
                        {totalSpent}
                    </div>
                </Card>
            </div>

            <Card>
                <div className="mb-4 flex items-center gap-2 text-lg font-medium text-stone-900 dark:text-stone-100">
                    <ReceiptText className="size-5" />
                    积分流水
                </div>
                <Table
                    dataSource={transactions}
                    columns={columns}
                    rowKey="id"
                    loading={transactionsLoading}
                    scroll={{ x: 1140 }}
                    pagination={{
                        current: page,
                        pageSize,
                        total,
                        showSizeChanger: true,
                        showTotal: (value) => `共 ${value} 条`,
                        onChange: (nextPage, nextPageSize) => {
                            setPage(nextPage);
                            setPageSize(nextPageSize);
                        },
                    }}
                    locale={{ emptyText: "暂无积分流水" }}
                />
            </Card>
        </main>
    );
}
