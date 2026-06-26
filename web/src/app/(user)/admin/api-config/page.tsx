"use client";

import { useEffect, useMemo, useState } from "react";
import { Alert, App, Button, Card, Form, Input, InputNumber, Select, Table, Tag } from "antd";
import { Plus, Settings, Trash2 } from "lucide-react";

import { saveApiConfig, type ApiConfigInfo } from "@/services/api/api-config";
import { deletePricing, listAdminPricing, savePricing, type PricingItem } from "@/services/api/pricing";
import { useConfigStore } from "@/stores/use-config-store";
import type { ColumnsType } from "antd/es/table";

type ModelCapability = "image" | "video" | "text" | "audio";

type ModelRow = {
    key: string;
    model: string;
    enabled: boolean;
    capabilities: ModelCapability[];
    credits_per_unit?: number;
    unit_type: string;
    video_route: string;
    pricing_id?: number;
};

const capabilityOptions = [
    { label: "图片", value: "image" },
    { label: "视频", value: "video" },
    { label: "文本", value: "text" },
    { label: "音频", value: "audio" },
];

const unitTypeOptions = [
    { label: "按图片 (per_image)", value: "per_image" },
    { label: "按视频 (per_video)", value: "per_video" },
    { label: "按 Token (per_token)", value: "per_token" },
];

const videoRouteOptions = [
    { label: "自动判断", value: "auto" },
    { label: "/v1/videos", value: "openai" },
    { label: "/v1/videos/generations", value: "xai" },
    { label: "/v1/video/generations", value: "newapi" },
    { label: "Seedance /contents/generations/tasks", value: "seedance" },
];

export default function AdminApiConfigPage() {
    const { message } = App.useApp();
    const applyServerModelCatalog = useConfigStore((state) => state.applyServerModelCatalog);
    const [apiConfig, setApiConfig] = useState<ApiConfigInfo | null>(null);
    const [pricingItems, setPricingItems] = useState<PricingItem[]>([]);
    const [rows, setRows] = useState<ModelRow[]>([]);
    const [loading, setLoading] = useState(false);
    const [saving, setSaving] = useState(false);
    const [form] = Form.useForm();

    const fetchApiConfig = async () => {
        setLoading(true);
        try {
            const { apiConfig: config, pricing } = await listAdminPricing();
            setApiConfig(config);
            setPricingItems(pricing);
            form.setFieldsValue({
                base_url: config?.base_url || "",
                api_key: "",
            });
            applyServerModelCatalog({
                models: config?.models || [],
                imageModels: config?.image_models || [],
                videoModels: config?.video_models || [],
                textModels: config?.text_models || [],
                audioModels: config?.audio_models || [],
                modelRoutes: config?.model_routes || {},
            });
            setRows(buildRows(config, pricing));
        } catch {
            setApiConfig(null);
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        void fetchApiConfig();
    }, []);

    const handleSave = async (values: {
        base_url: string;
        api_key: string;
    }) => {
        setSaving(true);
        try {
            const normalizedRows = normalizeRows(rows);
            if (!normalizedRows.length) throw new Error("请至少配置一个模型");

            const duplicates = findDuplicateModels(normalizedRows);
            if (duplicates.length) throw new Error(`模型名称重复：${duplicates.join("、")}`);

            const nextApiConfig = {
                base_url: values.base_url,
                api_key: values.api_key,
                models: normalizedRows.map((item) => item.model),
                image_models: normalizedRows.filter((item) => item.enabled && item.capabilities.includes("image")).map((item) => item.model),
                video_models: normalizedRows.filter((item) => item.enabled && item.capabilities.includes("video")).map((item) => item.model),
                text_models: normalizedRows.filter((item) => item.enabled && item.capabilities.includes("text")).map((item) => item.model),
                audio_models: normalizedRows.filter((item) => item.enabled && item.capabilities.includes("audio")).map((item) => item.model),
                model_routes: Object.fromEntries(normalizedRows.filter((item) => item.enabled && item.capabilities.includes("video") && item.video_route && item.video_route !== "auto").map((item) => [item.model, item.video_route])),
            };

            await saveApiConfig(nextApiConfig);

            for (const row of normalizedRows) {
                if (!row.enabled || !row.credits_per_unit || row.credits_per_unit <= 0) continue;
                await savePricing({
                    id: row.pricing_id,
                    model: row.model,
                    credits_per_unit: row.credits_per_unit,
                    unit_type: row.unit_type,
                });
            }

            for (const item of pricingItems) {
                const matched = normalizedRows.find((row) => row.pricing_id === item.id || row.model === item.model);
                if (!matched || !matched.enabled || !matched.credits_per_unit || matched.credits_per_unit <= 0) {
                    if (item.id) await deletePricing(item.id);
                }
            }

            applyServerModelCatalog({
                models: normalizedRows.filter((item) => item.enabled && Number(item.credits_per_unit) > 0).map((item) => item.model),
                imageModels: normalizedRows.filter((item) => item.enabled && item.capabilities.includes("image") && Number(item.credits_per_unit) > 0).map((item) => item.model),
                videoModels: normalizedRows.filter((item) => item.enabled && item.capabilities.includes("video") && Number(item.credits_per_unit) > 0).map((item) => item.model),
                textModels: normalizedRows.filter((item) => item.enabled && item.capabilities.includes("text") && Number(item.credits_per_unit) > 0).map((item) => item.model),
                audioModels: normalizedRows.filter((item) => item.enabled && item.capabilities.includes("audio") && Number(item.credits_per_unit) > 0).map((item) => item.model),
                modelRoutes: Object.fromEntries(normalizedRows.filter((item) => item.enabled && item.capabilities.includes("video") && item.video_route && item.video_route !== "auto" && Number(item.credits_per_unit) > 0).map((item) => [item.model, item.video_route])),
            });
            message.success("API、模型与计费配置已保存");
            await fetchApiConfig();
        } catch (err: any) {
            message.error(err?.message || "保存失败");
        } finally {
            setSaving(false);
        }
    };

    const enabledCount = useMemo(() => rows.filter((item) => item.enabled && Number(item.credits_per_unit) > 0).length, [rows]);
    const disabledRows = useMemo(() => rows.filter((item) => !item.enabled).map((item) => item.model).filter(Boolean), [rows]);

    const updateRow = (key: string, patch: Partial<ModelRow>) => {
        setRows((current) => current.map((item) => (item.key === key ? { ...item, ...patch } : item)));
    };

    const removeRow = (key: string) => {
        setRows((current) => current.filter((item) => item.key !== key));
    };

    const addRow = () => {
        setRows((current) => [
            ...current,
            {
                key: `new-${Date.now()}-${current.length}`,
                model: "",
                enabled: true,
                capabilities: ["image"],
                credits_per_unit: undefined,
                unit_type: "per_image",
                video_route: "auto",
            },
        ]);
    };

    const columns: ColumnsType<ModelRow> = [
        {
            title: "模型名称",
            dataIndex: "model",
            key: "model",
            render: (_, record) => (
                <Input
                    value={record.model}
                    placeholder="例如 gpt-image-2"
                    onChange={(event) => updateRow(record.key, { model: event.target.value })}
                />
            ),
        },
        {
            title: "能力",
            dataIndex: "capabilities",
            key: "capabilities",
            width: 240,
            render: (_, record) => (
                <Select
                    mode="multiple"
                    value={record.capabilities}
                    options={capabilityOptions}
                    className="w-full"
                    onChange={(value) => updateRow(record.key, { capabilities: value as ModelCapability[] })}
                />
            ),
        },
        {
            title: "计费单位",
            dataIndex: "unit_type",
            key: "unit_type",
            width: 180,
            render: (_, record) => (
                <Select
                    value={record.unit_type}
                    options={unitTypeOptions}
                    className="w-full"
                    disabled={!record.enabled}
                    onChange={(value) => updateRow(record.key, { unit_type: value })}
                />
            ),
        },
        {
            title: "视频接口",
            dataIndex: "video_route",
            key: "video_route",
            width: 240,
            render: (_, record) => (
                <Select
                    value={record.video_route || "auto"}
                    options={videoRouteOptions}
                    className="w-full"
                    disabled={!record.enabled || !record.capabilities.includes("video")}
                    onChange={(value) => updateRow(record.key, { video_route: value })}
                />
            ),
        },
        {
            title: "每次消耗积分",
            dataIndex: "credits_per_unit",
            key: "credits_per_unit",
            width: 180,
            render: (_, record) => (
                <InputNumber
                    min={0}
                    className="w-full"
                    value={record.credits_per_unit}
                    disabled={!record.enabled}
                    placeholder="0 表示未定价"
                    onChange={(value) => updateRow(record.key, { credits_per_unit: Number(value || 0) })}
                />
            ),
        },
        {
            title: "状态",
            key: "status",
            width: 180,
            render: (_, record) => (
                <div className="space-y-1">
                    <div className="inline-flex overflow-hidden rounded-lg border border-stone-200 bg-white dark:border-stone-700 dark:bg-stone-900">
                        <button
                            type="button"
                            className={[
                                "px-3 py-1.5 text-sm transition",
                                record.enabled
                                    ? "bg-emerald-500 text-white"
                                    : "bg-transparent text-stone-500 hover:bg-stone-100 dark:text-stone-300 dark:hover:bg-stone-800",
                            ].join(" ")}
                            onClick={() => updateRow(record.key, { enabled: true })}
                        >
                            启用
                        </button>
                        <button
                            type="button"
                            className={[
                                "border-l border-stone-200 px-3 py-1.5 text-sm transition dark:border-stone-700",
                                !record.enabled
                                    ? "bg-rose-500 text-white"
                                    : "bg-transparent text-stone-500 hover:bg-stone-100 dark:text-stone-300 dark:hover:bg-stone-800",
                            ].join(" ")}
                            onClick={() => updateRow(record.key, { enabled: false })}
                        >
                            禁用
                        </button>
                    </div>
                    {!record.enabled ? (
                        <Tag>已禁用</Tag>
                    ) : Number(record.credits_per_unit) > 0 ? (
                        <Tag color="green">已开放</Tag>
                    ) : (
                        <Tag color="gold">待定价</Tag>
                    )}
                </div>
            ),
        },
        {
            title: "操作",
            key: "actions",
            width: 90,
            render: (_, record) => (
                <Button danger type="text" icon={<Trash2 className="size-4" />} onClick={() => removeRow(record.key)} />
            ),
        },
    ];

    return (
        <div>
            <h2 className="mb-4 text-xl font-semibold text-stone-950 dark:text-stone-100">
                <Settings className="mr-2 inline size-5" />
                API 与模型配置
            </h2>
            <Alert
                className="mb-6"
                type="info"
                showIcon
                message="这里统一管理上游 API、模型目录和积分定价"
                description="只有配置了积分价格的模型，才会开放给用户使用。未定价模型会保留在目录里，但不会出现在用户侧。"
            />
            <Card loading={loading}>
                <Form
                    form={form}
                    layout="vertical"
                    onFinish={handleSave}
                    initialValues={{
                        base_url: apiConfig?.base_url || "",
                        api_key: "",
                    }}
                    key={apiConfig?.base_url || "empty"}
                >
                    <Form.Item
                        name="base_url"
                        label="上游 API 地址"
                        rules={[{ required: true, message: "请输入 API 基础地址" }]}
                        extra="请输入 OpenAI 兼容 API 根地址，例如: https://api.openai.com 或 http://8.219.243.189:3000；系统会自动拼接 /v1"
                    >
                        <Input placeholder="https://api.openai.com" />
                    </Form.Item>
                    <Form.Item
                        name="api_key"
                        label="API Key"
                        extra={apiConfig?.has_key ? "已保存 API Key；留空表示继续使用当前 Key，填写则覆盖" : "首次配置需要输入 API Key"}
                    >
                        <Input.Password placeholder="sk-..." />
                    </Form.Item>
                    <div className="mb-4 grid gap-4 md:grid-cols-3">
                        <Card size="small">
                            <div className="text-sm text-stone-500">模型总数</div>
                            <div className="mt-1 text-2xl font-semibold">{rows.length}</div>
                        </Card>
                        <Card size="small">
                            <div className="text-sm text-stone-500">已启用模型</div>
                            <div className="mt-1 text-2xl font-semibold">{enabledCount}</div>
                        </Card>
                        <Card size="small">
                            <div className="text-sm text-stone-500">已禁用模型</div>
                            <div className="mt-1 text-2xl font-semibold">{disabledRows.length}</div>
                        </Card>
                    </div>
                    {disabledRows.length ? (
                        <Alert
                            className="mb-4"
                            type="warning"
                            showIcon
                            message="以下模型当前已禁用，不会开放给用户"
                            description={disabledRows.join("、")}
                        />
                    ) : null}
                    <div className="mb-3 flex items-center justify-between">
                        <div className="text-base font-semibold text-stone-900 dark:text-stone-100">模型目录与计费</div>
                        <Button icon={<Plus className="size-4" />} onClick={addRow}>
                            新增模型
                        </Button>
                    </div>
                    <Table rowKey="key" columns={columns} dataSource={rows} pagination={false} scroll={{ x: 1320 }} />
                    <Button type="primary" htmlType="submit" loading={saving}>
                        保存全部配置
                    </Button>
                </Form>
                {apiConfig ? (
                    <div className="mt-4 text-sm text-stone-500">
                        当前已配置 API 地址: {apiConfig.base_url}
                        {apiConfig.has_key ? "（已设置 Key）" : "（未设置 Key）"}
                    </div>
                ) : null}
            </Card>
        </div>
    );
}

function buildRows(config: ApiConfigInfo, pricing: PricingItem[]): ModelRow[] {
    const orderedModels = Array.from(new Set([...(config.models || []), ...pricing.map((item) => item.model)]));
    const pricingMap = new Map(pricing.map((item) => [item.model, item]));
    return orderedModels.map((model, index) => {
        const pricingItem = pricingMap.get(model);
        return {
            key: `${model}-${index}`,
            model,
            enabled: Number(pricingItem?.credits_per_unit) > 0,
            capabilities: collectCapabilities(model, config),
            credits_per_unit: pricingItem?.credits_per_unit,
            unit_type: pricingItem?.unit_type || inferUnitType(model, config),
            video_route: config.model_routes?.[model] || "auto",
            pricing_id: pricingItem?.id,
        };
    });
}

function collectCapabilities(model: string, config: ApiConfigInfo): ModelCapability[] {
    const capabilities: ModelCapability[] = [];
    if ((config.image_models || []).includes(model)) capabilities.push("image");
    if ((config.video_models || []).includes(model)) capabilities.push("video");
    if ((config.text_models || []).includes(model)) capabilities.push("text");
    if ((config.audio_models || []).includes(model)) capabilities.push("audio");
    return capabilities.length ? capabilities : ["image"];
}

function inferUnitType(model: string, config: ApiConfigInfo): string {
    if ((config.video_models || []).includes(model)) return "per_video";
    if ((config.text_models || []).includes(model) || (config.audio_models || []).includes(model)) return "per_token";
    return "per_image";
}

function normalizeRows(rows: ModelRow[]) {
    return rows
        .map((item) => ({
            ...item,
            model: item.model.trim(),
            enabled: item.enabled !== false,
            capabilities: Array.from(new Set(item.capabilities)),
            video_route: item.capabilities.includes("video") ? item.video_route || "auto" : "auto",
        }))
        .filter((item) => item.model);
}

function findDuplicateModels(rows: Array<{ model: string }>) {
    const seen = new Set<string>();
    const duplicates = new Set<string>();
    for (const row of rows) {
        if (seen.has(row.model)) duplicates.add(row.model);
        seen.add(row.model);
    }
    return Array.from(duplicates);
}
