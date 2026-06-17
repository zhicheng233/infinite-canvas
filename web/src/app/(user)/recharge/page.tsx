"use client";

import { useEffect, useState, useCallback } from "react";
import {
  App,
  Button,
  Card,
  Modal,
  Table,
  Tag,
  Spin,
} from "antd";
import { ShoppingCart, Zap, CheckCircle2, Clock, XCircle } from "lucide-react";
import { listPayouts, createRechargeOrder, listMyOrders, type CreditPayout, type RechargeOrder } from "@/services/api/recharge";
import { getBalance } from "@/services/api/credits";
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
  const { message } = App.useApp();
  const [payouts, setPayouts] = useState<CreditPayout[]>([]);
  const [loading, setLoading] = useState(true);
  const [balance, setBalance] = useState<number | null>(null);
  const [purchasing, setPurchasing] = useState<string | null>(null);
  const [orders, setOrders] = useState<RechargeOrder[]>([]);
  const [ordersLoading, setOrdersLoading] = useState(false);
  const [confirmModal, setConfirmModal] = useState<{ open: boolean; payout: CreditPayout | null }>({
    open: false,
    payout: null,
  });

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const [payoutData, bal] = await Promise.all([
        listPayouts(),
        getBalance().catch(() => null),
      ]);
      setPayouts(payoutData);
      if (bal) setBalance(bal.balance);
    } catch {
      // ignore
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

  useEffect(() => {
    fetchData();
    fetchOrders();
  }, [fetchData, fetchOrders]);

  const handlePurchase = async () => {
    const payout = confirmModal.payout;
    if (!payout) return;
    setPurchasing(payout.id);
    try {
      const order = await createRechargeOrder(payout.id);
      if (order.status === "completed") {
        message.success(`购买成功！${payout.credits} 积分已到账`);
        fetchData();
        fetchOrders();
      } else {
        message.info("订单已创建，请完成支付");
      }
      setConfirmModal({ open: false, payout: null });
    } catch (err: any) {
      message.error(err?.message || "购买失败，请重试");
    } finally {
      setPurchasing(null);
    }
  };

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

  if (loading) {
    return (
      <main className="flex h-full items-center justify-center">
        <Spin size="large" />
      </main>
    );
  }

  return (
    <main className="mx-auto max-w-5xl overflow-y-auto px-6 py-8">
      {/* Header with balance */}
      <div className="mb-8 flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold text-stone-950 dark:text-stone-100">
            积分充值
          </h1>
          <p className="mt-1 text-sm text-stone-500 dark:text-stone-400">
            选择套餐购买积分，用于 AI 生成服务
          </p>
        </div>
        {balance !== null && (
          <div className="flex items-center gap-2 rounded-xl border border-amber-200 bg-amber-50 px-4 py-2.5 dark:border-amber-800 dark:bg-amber-950/60">
            <Zap className="size-5 text-amber-500" />
            <span className="text-sm text-stone-600 dark:text-stone-300">当前积分</span>
            <span className="text-xl font-bold text-amber-700 dark:text-amber-400">
              {balance}
            </span>
          </div>
        )}
      </div>

      {/* Credit packages */}
      <h2 className="mb-4 text-lg font-medium text-stone-800 dark:text-stone-200">
        选择套餐
      </h2>
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
                <span className={cn("rounded-full px-2.5 py-0.5 text-xs font-medium", colors.badge)}>
                  {payout.name}
                </span>
              </div>
              <div className="mb-1 flex items-baseline gap-1">
                <span className="text-3xl font-bold text-stone-900 dark:text-stone-100">
                  {payout.credits}
                </span>
                <span className="text-sm text-stone-500 dark:text-stone-400">积分</span>
              </div>
              <div className="mb-4 text-lg font-semibold text-stone-700 dark:text-stone-300">
                {payout.price}
              </div>
              <Button
                type="primary"
                icon={<ShoppingCart className="size-4" />}
                block
                loading={purchasing === payout.id}
                onClick={() => setConfirmModal({ open: true, payout })}
                className="mt-auto"
              >
                立即购买
              </Button>
            </div>
          );
        })}
      </div>

      {/* Order history */}
      <h2 className="mb-4 text-lg font-medium text-stone-800 dark:text-stone-200">
        充值记录
      </h2>
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

      {/* Confirmation modal */}
      <Modal
        title="确认购买"
        open={confirmModal.open}
        onOk={handlePurchase}
        onCancel={() => setConfirmModal({ open: false, payout: null })}
        okText="确认支付"
        cancelText="取消"
        confirmLoading={purchasing === confirmModal.payout?.id}
      >
        {confirmModal.payout && (
          <div className="py-2">
            <p className="text-base">
              确认购买 <strong>{confirmModal.payout.name}</strong>？
            </p>
            <div className="mt-3 rounded-lg bg-stone-50 p-4 dark:bg-stone-800">
              <div className="flex justify-between">
                <span className="text-stone-500">积分</span>
                <span className="font-mono font-semibold">{confirmModal.payout.credits}</span>
              </div>
              <div className="mt-2 flex justify-between">
                <span className="text-stone-500">价格</span>
                <span className="font-semibold">{confirmModal.payout.price}</span>
              </div>
            </div>
            <p className="mt-3 text-xs text-stone-400">
              * 当前为模拟支付，点击确认后积分将立即到账
            </p>
          </div>
        )}
      </Modal>
    </main>
  );
}
