"use client";

import { Button, Descriptions, Modal } from "antd";
import { useMemo, useState } from "react";

import { creditTransactionDetailItems, creditTransactionModel, creditTransactionSummary, parseCreditMetadata, type CreditDisplayTransaction } from "@/lib/credit-display";

type Props = {
  record: CreditDisplayTransaction & {
    id?: number;
    type?: string;
    amount?: number;
    balance_before?: number;
    balance_after?: number;
    ref_type?: string;
    created_at?: string;
  };
};

export function CreditTransactionDetailButton({ record }: Props) {
  const [open, setOpen] = useState(false);
  const detailItems = useMemo(() => creditTransactionDetailItems(record), [record]);
  const metadata = useMemo(() => parseCreditMetadata(record.metadata), [record.metadata]);
  const summary = creditTransactionSummary(record);
  const model = creditTransactionModel(record);
  const signedAmount = typeof record.amount === "number" ? `${record.type === "spend" ? "-" : "+"}${Math.abs(record.amount)}` : "-";

  const items = [
    { key: "id", label: "流水 ID", children: record.id || "-" },
    { key: "type", label: "类型", children: record.type || "-" },
    { key: "amount", label: "积分变动", children: signedAmount },
    { key: "balance", label: "余额", children: typeof record.balance_before === "number" ? `${record.balance_before} → ${record.balance_after}` : record.balance_after ?? "-" },
    { key: "source", label: "来源", children: record.ref_type || "-" },
    { key: "model", label: "模型", children: model },
    { key: "time", label: "时间", children: record.created_at ? new Date(record.created_at).toLocaleString("zh-CN") : "-" },
    ...detailItems.map((item) => ({ key: item.label, label: item.label, children: item.value })),
  ];

  return (
    <>
      <div className="flex min-w-0 items-center gap-2">
        <span className="min-w-0 flex-1 truncate text-stone-700 dark:text-stone-300" title={summary}>
          {summary}
        </span>
        <Button size="small" type="link" className="shrink-0 !px-0" onClick={() => setOpen(true)}>
          查看
        </Button>
      </div>
      <Modal title="积分流水详情" open={open} footer={null} width={760} onCancel={() => setOpen(false)}>
        <Descriptions bordered size="small" column={1} items={items} />
        {record.metadata ? (
          <details className="mt-4 rounded-lg border border-stone-200 p-3 text-xs dark:border-stone-700">
            <summary className="cursor-pointer text-stone-500 dark:text-stone-400">原始 metadata</summary>
            <pre className="mt-3 max-h-60 overflow-auto whitespace-pre-wrap break-all text-stone-600 dark:text-stone-300">{JSON.stringify(metadata, null, 2)}</pre>
          </details>
        ) : null}
      </Modal>
    </>
  );
}
