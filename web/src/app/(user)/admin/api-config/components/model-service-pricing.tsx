"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { App, Button, Form, Input, InputNumber, Modal, Select, Table, Tag, Typography } from "antd";
import { Pencil, RefreshCw } from "lucide-react";

import { listModelConfigs, saveDefaultModelPricing, type ModelCapability, type ModelConfig, type SaveModelPricingInput } from "@/services/api/model-service";

const capabilityLabels: Record<ModelCapability, string> = { image: "图片", video: "视频", text: "文本", audio: "音频" };

type PricingRow = {
    key: string;
    catalogModelId: number;
    publicKey: string;
    displayName: string;
    capability: ModelCapability;
    implementations: ModelConfig[];
    defaultPricing?: SaveModelPricingInput;
    overrideCount: number;
};

export function ModelServicePricing({ refreshToken, onChanged }: { readonly refreshToken: number; readonly onChanged: () => void }) {
    const { message } = App.useApp();
    const [models, setModels] = useState<ModelConfig[]>([]);
    const [loading, setLoading] = useState(true);
    const [editing, setEditing] = useState<PricingRow>();
    const [saving, setSaving] = useState(false);
    const [form] = Form.useForm<SaveModelPricingInput>();

    const load = useCallback(async () => {
        setLoading(true);
        try {
            setModels(await listModelConfigs());
        } catch (error) {
            message.error(errorMessage(error, "读取模型定价失败"));
        } finally {
            setLoading(false);
        }
    }, [message]);

    useEffect(() => {
        void load();
    }, [load, refreshToken]);

    const rows = useMemo(() => buildPricingRows(models), [models]);

    const open = (row: PricingRow) => {
        setEditing(row);
        form.setFieldsValue(row.defaultPricing ?? defaultPricing(row.capability));
    };

    const save = async (values: SaveModelPricingInput) => {
        if (!editing) return;
        setSaving(true);
        try {
            await saveDefaultModelPricing(editing.catalogModelId, { ...values, capability: editing.capability });
            message.success("全局默认价已保存");
            setEditing(undefined);
            await load();
            onChanged();
        } catch (error) {
            message.error(errorMessage(error, "保存默认定价失败"));
        } finally {
            setSaving(false);
        }
    };

    return (
        <div className="space-y-4">
            <div className="flex flex-wrap items-center justify-between gap-3">
                <div>
                    <Typography.Title level={5} className="!mb-0">定价</Typography.Title>
                    <Typography.Text type="secondary">按公开模型和能力设置全局默认价；模型详情中的渠道覆盖价优先生效。</Typography.Text>
                </div>
                <Button icon={<RefreshCw className="size-4" />} loading={loading} onClick={() => void load()}>刷新</Button>
            </div>
            <Table
                rowKey="key"
                loading={loading}
                dataSource={rows}
                pagination={{ pageSize: 20, hideOnSinglePage: true }}
                columns={[
                    { title: "公开模型", render: (_, row) => <div><div className="font-medium">{row.displayName}</div><Typography.Text type="secondary" className="text-xs">{row.publicKey}</Typography.Text></div> },
                    { title: "能力", width: 90, render: (_, row) => capabilityLabels[row.capability] },
                    { title: "全局默认价", width: 170, render: (_, row) => row.defaultPricing ? pricingText(row.defaultPricing) : <Tag color="warning">未设置</Tag> },
                    { title: "渠道覆盖", width: 110, render: (_, row) => row.overrideCount ? <Tag color="blue">{row.overrideCount} 个</Tag> : <Tag>无</Tag> },
                    { title: "渠道实现", width: 110, render: (_, row) => row.implementations.length },
                    { title: "生效来源", width: 130, render: (_, row) => row.implementations.every((item) => item.pricing.some((price) => price.capability === row.capability)) ? <Tag color="green">全部有价格</Tag> : <Tag color="warning">存在未定价</Tag> },
                    { title: "操作", width: 80, render: (_, row) => <Button type="text" icon={<Pencil className="size-4" />} title="编辑默认价" onClick={() => open(row)} /> },
                ]}
            />

            <Modal destroyOnHidden open={Boolean(editing)} title={`默认定价：${editing?.displayName ?? ""} · ${editing ? capabilityLabels[editing.capability] : ""}`} okText="保存" confirmLoading={saving} onCancel={() => setEditing(undefined)} onOk={() => form.submit()}>
                <Form form={form} layout="vertical" preserve={false} onFinish={(values) => void save(values)}>
                    <Form.Item name="pricing_mode" label="计费模式"><Select options={editing?.capability === "video" ? [{ value: "per_unit", label: "固定单价" }, { value: "video_dynamic", label: "按视频参数动态计费" }] : [{ value: "per_unit", label: "固定单价" }]} /></Form.Item>
                    <Form.Item noStyle shouldUpdate={(previous, current) => previous.pricing_mode !== current.pricing_mode}>
                        {() => form.getFieldValue("pricing_mode") === "video_dynamic" ? (
                            <Form.Item name="pricing_rule" label="动态定价规则" rules={[{ required: true, whitespace: true, message: "请输入动态定价规则 JSON" }]}><Input.TextArea rows={7} className="font-mono" placeholder='{"default":10,"rules":[]}' /></Form.Item>
                        ) : (
                            <Form.Item name="credits_per_unit" label="积分单价" rules={[{ required: true, message: "请输入积分单价" }]}><InputNumber className="w-full" min={1} precision={0} /></Form.Item>
                        )}
                    </Form.Item>
                    <Form.Item name="unit_type" label="计费单位"><Select options={unitOptions(editing?.capability ?? "image")} /></Form.Item>
                </Form>
            </Modal>
        </div>
    );
}

export function buildPricingRows(models: ModelConfig[]): PricingRow[] {
    const rows = new Map<string, PricingRow>();
    for (const item of models) {
        const capabilities = [...new Set(item.operations.filter((operation) => operation.enabled).map((operation) => operation.capability))];
        for (const capability of capabilities) {
            const key = `${item.catalog_model_id}:${capability}`;
            const row = rows.get(key) ?? { key, catalogModelId: item.catalog_model_id, publicKey: item.public_key, displayName: item.display_name, capability, implementations: [], overrideCount: 0 };
            row.implementations.push(item);
            const price = item.pricing.find((value) => value.capability === capability);
            if (price?.effective_source === "default" && !row.defaultPricing) row.defaultPricing = { capability, credits_per_unit: price.credits_per_unit, unit_type: price.unit_type, pricing_mode: price.pricing_mode, pricing_rule: price.pricing_rule };
            if (price?.effective_source === "implementation") row.overrideCount++;
            rows.set(key, row);
        }
    }
    return [...rows.values()].sort((left, right) => left.displayName.localeCompare(right.displayName, "zh-CN") || left.capability.localeCompare(right.capability));
}

function defaultPricing(capability: ModelCapability): SaveModelPricingInput {
    return { capability, credits_per_unit: 1, unit_type: capability === "image" ? "per_image" : capability === "video" ? "per_video" : "per_token", pricing_mode: "per_unit", pricing_rule: "" };
}

function unitOptions(capability: ModelCapability) {
    if (capability === "image") return [{ value: "per_image", label: "每张图片" }];
    if (capability === "video") return [{ value: "per_video", label: "每个视频" }, { value: "per_video_second", label: "每视频秒" }];
    return [{ value: "per_token", label: "每 Token" }];
}

function pricingText(value: SaveModelPricingInput) {
    if (value.pricing_mode === "video_dynamic") return <Tag color="cyan">动态规则</Tag>;
    const units: Record<string, string> = { per_image: "张", per_video: "个视频", per_video_second: "视频秒", per_token: "Token" };
    return `${value.credits_per_unit} 积分 / ${units[value.unit_type] ?? value.unit_type}`;
}

function errorMessage(error: unknown, fallback: string) {
    return error instanceof Error && error.message ? error.message : fallback;
}
