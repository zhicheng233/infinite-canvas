import type { CreditTransactionMetadata } from "@/services/api/credits";

export type CreditDisplayTransaction = {
  note?: string;
  metadata?: string;
  ref_type?: string;
  ref_id?: string;
};

export function parseCreditMetadata(value?: string): CreditTransactionMetadata {
  if (!value) return {};
  try {
    const parsed = JSON.parse(value) as CreditTransactionMetadata;
    return parsed && typeof parsed === "object" ? parsed : {};
  } catch {
    return {};
  }
}

export function creditTransactionModel(record: CreditDisplayTransaction) {
  const detail = parseCreditMetadata(record.metadata);
  if (detail.model) return detail.model;
  return record.ref_id && ["image", "video", "audio", "text"].includes(record.ref_type || "") ? record.ref_id : "-";
}

export function creditTransactionDetail(record: CreditDisplayTransaction) {
  const detail = parseCreditMetadata(record.metadata);
  const items = creditTransactionDetailItems(record);
  return items.length ? items.map((item) => item.value).join(" · ") : "-";
}

export function creditTransactionDetailItems(record: CreditDisplayTransaction) {
  const detail = parseCreditMetadata(record.metadata);
  return [
    { label: "备注", value: record.note },
    { label: "分辨率", value: detail.resolution },
    { label: "时长", value: detail.seconds ? `${detail.seconds} 秒` : "" },
    { label: "计费单位", value: detail.unit_label && detail.unit_cost ? `${detail.unit_label} ${detail.unit_cost} 积分` : "" },
    { label: "数量", value: detail.units && detail.units > 1 ? String(detail.units) : "" },
    { label: "计费公式", value: detail.formula },
    { label: "合计", value: detail.total_cost ? `${detail.total_cost} 积分` : "" },
    { label: "接口", value: detail.path },
    { label: "操作人", value: detail.operator_user_id ? `#${detail.operator_user_id}` : "" },
    { label: "用户", value: detail.target_user_id ? `#${detail.target_user_id}` : "" },
    { label: "订单", value: detail.recharge_order_id ? `#${detail.recharge_order_id}` : "" },
  ].filter((item): item is { label: string; value: string } => Boolean(item.value));
}

export function creditTransactionSummary(record: CreditDisplayTransaction) {
  const detail = parseCreditMetadata(record.metadata);
  const items = [
    record.note,
    detail.total_cost ? `合计 ${detail.total_cost} 积分` : "",
    detail.formula ? detail.formula : "",
  ].filter(Boolean);
  return items.length ? items.join(" · ") : "-";
}
