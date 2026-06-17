"use client";

import { useEffect, useState, useCallback } from "react";
import { Table, Tag, Typography, App } from "antd";
import type { ColumnsType } from "antd/es/table";
import { listTransactions, type TransactionItem } from "@/services/api/admin";

const { Title } = Typography;

const typeLabels: Record<string, string> = {
  earn: "收入",
  spend: "支出",
  refund: "退款",
  adjust: "调整",
  recharge: "充值",
  welcome: "注册赠送",
};

const typeColors: Record<string, string> = {
  earn: "green",
  spend: "red",
  refund: "orange",
  adjust: "blue",
  recharge: "green",
  welcome: "cyan",
};

export default function AdminTransactionsPage() {
  const { message } = App.useApp();
  const [transactions, setTransactions] = useState<TransactionItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [pagination, setPagination] = useState({ current: 1, pageSize: 20, total: 0 });

  const fetch = useCallback(async (page = 1, pageSize = 20) => {
    setLoading(true);
    try {
      const data = await listTransactions(page, pageSize);
      setTransactions(data.items);
      setPagination({ current: data.page, pageSize: data.page_size, total: data.total });
    } catch (err: any) {
      message.error(err?.message || "获取流水失败");
    } finally {
      setLoading(false);
    }
  }, [message]);

  useEffect(() => { fetch(); }, [fetch]);

  const columns: ColumnsType<TransactionItem> = [
    { title: "ID", dataIndex: "id", key: "id", width: 80 },
    {
      title: "类型",
      dataIndex: "type",
      key: "type",
      width: 100,
      render: (v: string) => <Tag color={typeColors[v] || "default"}>{typeLabels[v] || v}</Tag>,
    },
    {
      title: "金额",
      dataIndex: "amount",
      key: "amount",
      width: 100,
      render: (v: number, record) => (
        <span className={record.type === "spend" ? "text-red-500" : "text-green-500"}>
          {record.type === "spend" ? "-" : "+"}{v}
        </span>
      ),
    },
    { title: "余额", dataIndex: "balance_after", key: "balance_after", width: 100 },
    { title: "来源", dataIndex: "ref_type", key: "ref_type", width: 100 },
    { title: "备注", dataIndex: "note", key: "note" },
    {
      title: "时间",
      dataIndex: "created_at",
      key: "created_at",
      width: 180,
      render: (v: string) => new Date(v).toLocaleString("zh-CN"),
    },
  ];

  return (
    <div>
      <Title level={4} className="!mb-4">积分流水</Title>
      <Table
        rowKey="id"
        columns={columns}
        dataSource={transactions}
        loading={loading}
        pagination={{
          ...pagination,
          showSizeChanger: true,
          showTotal: (total) => `共 ${total} 条`,
          onChange: (page, pageSize) => fetch(page, pageSize),
        }}
      />
    </div>
  );
}
