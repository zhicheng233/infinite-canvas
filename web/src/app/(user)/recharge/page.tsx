"use client";

import { useEffect, useState, useCallback } from "react";
import Link from "next/link";
import { Button, Card, Table, Tag, Spin } from "antd";
import { Zap, CheckCircle2, Clock, XCircle, ArrowDownLeft, ArrowUpRight, ReceiptText } from "lucide-react";
import { listPayouts, listMyOrders, type CreditPayout, type RechargeOrder } from "@/services/api/recharge";
import { getBalance, getTransactions } from "@/services/api/credits";
import { CreditTransactionDetailButton } from "@/components/credits/credit-transaction-detail-button";
import { creditTransactionModel } from "@/lib/credit-display";
import { cn } from "@/lib/utils";

const payoutColors: Record<string, { bg: string; border: string; badge: string }> = {
  basic: {
    bg: "from-stone-50 to-stone-100 dark:from-stone-900 dark:to-stone-800",
    border: "border-stone-300 dark:border-stone-700",
    badge: "bg-stone-100 text-stone-700 dark:bg-stone-800 dark:text-stone-300",
  },
  standard: {
    bg: "from-blue-50 to-sky-100 dark:from-blue-950 dark:to-sky-900",
    border: "border-blue-300 dark:border-blue-700",
    badge: "bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300",
  },
  pro: {
    bg: "from-purple-50 to-violet-100 dark:from-purple-950 dark:to-violet-900",
    border: "border-purple-300 dark:border-purple-700",
    badge: "bg-purple-100 text-purple-700 dark:bg-purple-900 dark:text-purple-300",
  },
  premium: {
    bg: "from-amber-50 to-orange-100 dark:from-amber-950 dark:to-orange-900",
    border: "border-amber-300 dark:border-amber-700",
    badge: "bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300",
  },
  ultimate: {
    bg: "from-rose-50 to-red-100 dark:from-rose-950 dark:to-red-900",
    border: "border-rose-300 dark:border-rose-700",
    badge: "bg-rose-100 text-rose-700 dark:bg-rose-900 dark:text-rose-300",
  },
};

const statusConfig: Record<string, { color: string; icon: React.ReactNode; label: string }> = {
  pending: { color: "processing", icon: <Clock className="size-3.5" />, label: "待支付" },
  paid: { color: "blue", icon: <CheckCircle2 className="size-3.5" />, label: "已支付" },
  completed: { color: "success", icon: <CheckCircle2 className="size-3.5" />, label: "已完成" },
  failed: { color: "error", icon: <XCircle className="size-3.5" />, label: "失败" },
};

export default function RechargePage() {
  const [payouts, setPayouts] = useState<CreditPayout[]>([]);
  const [loading, setLoading] = useState(true);
  const [balance, setBalance] = useState<number | null>(null);
  const [totalEarned, setTotalEarned] = useState<number>(0);
  const [totalSpent, setTotalSpent] = useState<number>(0);
  const [orders, setOrders] = useState<RechargeOrder[]>([]);
  const [ordersLoading, setOrdersLoading] = useState(false);
  const [transactions, setTransactions] = useState<Array<{ id: number; type: string; amount: number; balance_before?: number; balance_after: number; ref_type: string; ref_id?: string; note: string; metadata?: string; created_at: string }>>([]);
  const [transactionsLoading, setTransactionsLoading] = useState(false);

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const [payoutData, bal] = await Promise.all([listPayouts(), getBalance().catch(() => null)]);
      setPayouts(payoutData);
      if (bal) {
        setBalance(bal.balance);
        setTotalEarned(bal.total_earned);
        setTotalSpent(bal.total_spent);
      }
    } finally {
      setLoading(false);
    }
  }, []);

  const fetchOrders = useCallback(async () => {
    setOrdersLoading(true);
    try {
      const result = await listMyOrders(1, 50);
      setOrders(result.items || []);
    } catch {
      setOrders([]);
    } finally {
      setOrdersLoading(false);
    }
  }, []);

  const fetchTransactions = useCallback(async () => {
    setTransactionsLoading(true);
    try {
      const result = await getTransactions(1, 50);
      setTransactions(result.items || []);
    } catch {
      setTransactions([]);
    } finally {
      setTransactionsLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData();
    fetchOrders();
    fetchTransactions();
  }, [fetchData, fetchOrders, fetchTransactions]);

  const orderColumns = [
    {
      title: "套餐",
      dataIndex: "note",
      key: "note",
      render: (v: string) => <span className="font-medium">{v}</span>,
    },
    {
      title: "积分",
      dataIndex: "credits",
      key: "credits",
      render: (v: number) => (
        <span className="inline-flex items-center gap-1">
          <Zap className="size-3.5 text-amber-500" />
          <span className="font-mono text-sm">{v}</span>
        </span>
      ),
    },
    {
      title: "状态",
      dataIndex: "status",
      key: "status",
      render: (v: string) => {
        const cfg = statusConfig[v] || { color: "default", icon: null, label: v };
        return (
          <Tag color={cfg.color} icon={cfg.icon}>
            {cfg.label}
          </Tag>
        );
      },
    },
    {
      title: "时间",
      dataIndex: "created_at",
      key: "created_at",
      render: (v: string) => new Date(v).toLocaleString("zh-CN"),
    },
  ];

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

  const transactionColumns = [
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
      render: (value: number, record: { type: string }) => {
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
      render: (_: unknown, record: { balance_before?: number; balance_after: number }) => (
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
      width: 160,
      ellipsis: true,
      render: (_: unknown, record: { metadata?: string; ref_id?: string }) => creditTransactionModel(record),
    },
    {
      title: "详情",
      key: "detail",
      width: 340,
      render: (_: unknown, record: { id?: number; type?: string; amount?: number; balance_before?: number; balance_after?: number; ref_type?: string; ref_id?: string; note?: string; metadata?: string; created_at?: string }) => <CreditTransactionDetailButton record={record} />,
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
    <main className="mx-auto max-w-5xl overflow-y-auto px-6 py-8">
      <div className="mb-8 flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold text-stone-950 dark:text-stone-100">积分充值</h1>
          <p className="mt-1 text-sm text-stone-500 dark:text-stone-400">当前仅展示套餐，暂不开放在线购买</p>
        </div>
        <div className="flex items-center gap-3">
          {balance !== null && (
            <div className="flex items-center gap-2 rounded-xl border border-amber-200 bg-amber-50 px-4 py-2.5 dark:border-amber-800 dark:bg-amber-950/60">
              <Zap className="size-5 text-amber-500" />
              <span className="text-sm text-stone-600 dark:text-stone-300">当前积分</span>
              <span className="text-xl font-bold text-amber-700 dark:text-amber-400">{balance}</span>
            </div>
          )}
          <Link href="/credits">
            <Button>查看积分明细</Button>
          </Link>
        </div>
      </div>

      <div className="mb-8 grid gap-4 md:grid-cols-3">
        <Card>
          <div className="text-sm text-stone-500 dark:text-stone-400">当前积分</div>
          <div className="mt-2 text-2xl font-semibold text-stone-950 dark:text-stone-100">{balance ?? 0}</div>
        </Card>
        <Card>
          <div className="text-sm text-stone-500 dark:text-stone-400">累计收入</div>
          <div className="mt-2 inline-flex items-center gap-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-400">
            <ArrowDownLeft className="size-5" />
            {totalEarned}
          </div>
        </Card>
        <Card>
          <div className="text-sm text-stone-500 dark:text-stone-400">累计支出</div>
          <div className="mt-2 inline-flex items-center gap-2 text-2xl font-semibold text-rose-600 dark:text-rose-400">
            <ArrowUpRight className="size-5" />
            {totalSpent}
          </div>
        </Card>
      </div>

      <h2 className="mb-4 text-lg font-medium text-stone-800 dark:text-stone-200">选择套餐</h2>
      <div className="mb-10 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5">
        {payouts.map((payout) => {
          const colors = payoutColors[payout.id] || payoutColors.basic;
          return (
            <div
              key={payout.id}
              className={cn(
                "relative flex flex-col rounded-2xl border bg-gradient-to-b p-5 transition hover:shadow-lg",
                colors.bg,
                colors.border,
              )}
            >
              <div className="mb-3 flex items-center justify-between">
                <span className={cn("rounded-full px-2.5 py-0.5 text-xs font-medium", colors.badge)}>{payout.name}</span>
              </div>
              <div className="mb-1 flex items-baseline gap-1">
                <span className="text-3xl font-bold text-stone-900 dark:text-stone-100">{payout.credits}</span>
                <span className="text-sm text-stone-500 dark:text-stone-400">积分</span>
              </div>
              <div className="mb-4 text-lg font-semibold text-stone-700 dark:text-stone-300">{payout.price}</div>
              <Button type="primary" block disabled className="mt-auto">
                暂不开放购买
              </Button>
            </div>
          );
        })}
      </div>

      <h2 className="mb-4 text-lg font-medium text-stone-800 dark:text-stone-200">充值记录</h2>
      <Card>
        <Table
          dataSource={orders}
          columns={orderColumns}
          rowKey="id"
          loading={ordersLoading}
          pagination={false}
          locale={{ emptyText: "暂无充值记录" }}
        />
      </Card>

      <h2 className="mb-4 mt-10 flex items-center gap-2 text-lg font-medium text-stone-800 dark:text-stone-200">
        <ReceiptText className="size-5" />
        积分流水
      </h2>
      <Card>
        <Table
          dataSource={transactions}
          columns={transactionColumns}
          rowKey="id"
          loading={transactionsLoading}
          pagination={false}
          locale={{ emptyText: "暂无积分流水" }}
          scroll={{ x: 1080 }}
        />
      </Card>
    </main>
  );
}
