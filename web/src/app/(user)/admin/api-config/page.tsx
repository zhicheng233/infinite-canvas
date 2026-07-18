"use client";

import { useEffect, useMemo, useState } from "react";
import { Alert, App, Button, Card, Form, Input, InputNumber, Modal, Select, Space, Table, Tag, Typography } from "antd";
import { PlayCircle, Plus, Settings, Trash2 } from "lucide-react";

import { saveApiConfig, testApiModel, type ApiConfigInfo, type ApiModelTestResult } from "@/services/api/api-config";
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
    image_generate_route: string;
    image_edit_route: string;
    video_route: string;
    video_durations: string;
    video_customizable: boolean;
    pricing_id?: number;
    pricing_mode: string;
    video_base_credits?: number;
    video_rate_480p?: number;
    video_rate_720p?: number;
    video_rate_1080p?: number;
    video_rate_2k?: number;
    video_rate_4k?: number;
};

type ModelTestState = {
    row: ModelRow;
    generation: ModelCapability;
    prompt: string;
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
    { label: "按视频秒数 (per_video_second)", value: "per_video_second" },
    { label: "按 Token (per_token)", value: "per_token" },
];

const pricingModeOptions = [
    { label: "按次/数量", value: "per_unit" },
    { label: "视频动态", value: "video_dynamic" },
];

const imageRouteOptions = [
    { label: "自动判断", value: "auto" },
    { label: "/v1/images/generations", value: "generations" },
    { label: "/v1/images/edits", value: "edits" },
    { label: "/v1/chat/completions（多模态生图）", value: "chat" },
    { label: "/v1/chat/completions（Banana 参数）", value: "banana" },
];

const videoRouteOptions = [
    { label: "默认 /v1/videos", value: "auto" },
    { label: "/v1/videos", value: "openai" },
    { label: "/v1/videos（JSON / veo）", value: "veo_json" },
    { label: "/v1/videos（JSON / yijia）", value: "yijia" },
    { label: "/v1/videos JSON / Waninter", value: "waninter" },
    { label: "/v1/videos/generations", value: "xai" },
    { label: "/v1/video/generations", value: "newapi" },
    { label: "Seedance /contents/generations/tasks", value: "seedance" },
];

const videoRateFields: Array<{
    label: string;
    key: "video_rate_480p" | "video_rate_720p" | "video_rate_1080p" | "video_rate_2k" | "video_rate_4k";
}> = [
    { label: "480p", key: "video_rate_480p" },
    { label: "720p", key: "video_rate_720p" },
    { label: "1080p", key: "video_rate_1080p" },
    { label: "2K", key: "video_rate_2k" },
    { label: "4K", key: "video_rate_4k" },
];

const defaultTestPrompts: Record<ModelCapability, string> = {
    image: "生成一张用于模型连通性测试的简洁图片：一只小猫坐在白色桌面上",
    video: "模型连通性测试：一只小猫在桌面上轻轻转头",
    text: "请回复：模型连通性测试成功。",
    audio: "这是一段音频模型连通性测试。",
};

export default function AdminApiConfigPage() {
    const { message } = App.useApp();
    const applyServerModelCatalog = useConfigStore((state) => state.applyServerModelCatalog);
    const [apiConfig, setApiConfig] = useState<ApiConfigInfo | null>(null);
    const [pricingItems, setPricingItems] = useState<PricingItem[]>([]);
    const [rows, setRows] = useState<ModelRow[]>([]);
    const [advancedRowKey, setAdvancedRowKey] = useState<string | null>(null);
    const [modelTest, setModelTest] = useState<ModelTestState | null>(null);
    const [modelTestResult, setModelTestResult] = useState<ApiModelTestResult | null>(null);
    const [testingModel, setTestingModel] = useState(false);
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
                modelVideoDurations: config?.model_video_durations || {},
                modelVideoCustomizable: config?.model_video_customizable || {},
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
            const invalidDynamicRows = normalizedRows.filter((item) => item.enabled && item.pricing_mode === "video_dynamic" && !hasVideoRate(item));
            if (invalidDynamicRows.length) throw new Error(`请为视频动态计费模型配置至少一个秒单价：${invalidDynamicRows.map((item) => item.model).join("、")}`);

            const nextApiConfig = {
                base_url: values.base_url,
                api_key: values.api_key,
                models: normalizedRows.map((item) => item.model),
                image_models: normalizedRows.filter((item) => item.enabled && item.capabilities.includes("image")).map((item) => item.model),
                video_models: normalizedRows.filter((item) => item.enabled && item.capabilities.includes("video")).map((item) => item.model),
                text_models: normalizedRows.filter((item) => item.enabled && item.capabilities.includes("text")).map((item) => item.model),
                audio_models: normalizedRows.filter((item) => item.enabled && item.capabilities.includes("audio")).map((item) => item.model),
                model_routes: Object.fromEntries(
                    normalizedRows.flatMap((item) => {
                        const entries: Array<[string, string]> = [];
                        if (item.enabled && item.capabilities.includes("image") && item.image_generate_route && item.image_generate_route !== "auto") {
                            entries.push([`image_generate:${item.model}`, item.image_generate_route]);
                        }
                        if (item.enabled && item.capabilities.includes("image") && item.image_edit_route && item.image_edit_route !== "auto") {
                            entries.push([`image_edit:${item.model}`, item.image_edit_route]);
                        }
                        if (item.enabled && item.capabilities.includes("video") && item.video_route && item.video_route !== "auto") {
                            entries.push([`video:${item.model}`, item.video_route]);
                        }
                        return entries;
                    }),
                ),
                model_video_durations: Object.fromEntries(
                    normalizedRows
                        .filter((item) => item.enabled && item.capabilities.includes("video"))
                        .map((item) => [item.model, parseDurationInput(item.video_durations)])
                        .filter(([, durations]) => durations.length > 0),
                ),
                model_video_customizable: Object.fromEntries(
                    normalizedRows
                        .filter((item) => item.enabled && item.capabilities.includes("video") && item.video_customizable)
                        .map((item) => [item.model, true]),
                ),
            };

            await saveApiConfig(nextApiConfig);

            for (const row of normalizedRows) {
                if (!rowHasValidPricing(row)) continue;
                await savePricing({
                    id: row.pricing_id,
                    model: row.model,
                    credits_per_unit: row.pricing_mode === "video_dynamic" ? 0 : Number(row.credits_per_unit || 0),
                    unit_type: row.pricing_mode === "video_dynamic" ? "per_video_second" : row.unit_type,
                    pricing_mode: row.pricing_mode,
                    pricing_rule: row.pricing_mode === "video_dynamic" ? buildVideoPricingRule(row) : "",
                });
            }

            for (const item of pricingItems) {
                const matched = normalizedRows.find((row) => row.pricing_id === item.id || row.model === item.model);
                if (!matched || !rowHasValidPricing(matched)) {
                    if (item.id) await deletePricing(item.id);
                }
            }

            applyServerModelCatalog({
                models: normalizedRows.filter(rowHasValidPricing).map((item) => item.model),
                imageModels: normalizedRows.filter((item) => rowHasValidPricing(item) && item.capabilities.includes("image")).map((item) => item.model),
                videoModels: normalizedRows.filter((item) => rowHasValidPricing(item) && item.capabilities.includes("video")).map((item) => item.model),
                textModels: normalizedRows.filter((item) => rowHasValidPricing(item) && item.capabilities.includes("text")).map((item) => item.model),
                audioModels: normalizedRows.filter((item) => rowHasValidPricing(item) && item.capabilities.includes("audio")).map((item) => item.model),
                modelRoutes: Object.fromEntries(
                    normalizedRows.flatMap((item) => {
                        const entries: Array<[string, string]> = [];
                        if (rowHasValidPricing(item) && item.capabilities.includes("image") && item.image_generate_route && item.image_generate_route !== "auto") {
                            entries.push([`image_generate:${item.model}`, item.image_generate_route]);
                        }
                        if (rowHasValidPricing(item) && item.capabilities.includes("image") && item.image_edit_route && item.image_edit_route !== "auto") {
                            entries.push([`image_edit:${item.model}`, item.image_edit_route]);
                        }
                        if (rowHasValidPricing(item) && item.capabilities.includes("video") && item.video_route && item.video_route !== "auto") {
                            entries.push([`video:${item.model}`, item.video_route]);
                        }
                        return entries;
                    }),
                ),
                modelVideoDurations: Object.fromEntries(
                    normalizedRows
                        .filter((item) => rowHasValidPricing(item) && item.capabilities.includes("video"))
                        .map((item) => [item.model, parseDurationInput(item.video_durations)])
                        .filter(([, durations]) => durations.length > 0),
                ),
                modelVideoCustomizable: Object.fromEntries(
                    normalizedRows
                        .filter((item) => rowHasValidPricing(item) && item.capabilities.includes("video") && item.video_customizable)
                        .map((item) => [item.model, true]),
                ),
            });
            message.success("API、模型与计费配置已保存");
            await fetchApiConfig();
        } catch (err: any) {
            message.error(err?.message || "保存失败");
        } finally {
            setSaving(false);
        }
    };

    const enabledCount = useMemo(() => rows.filter(rowHasValidPricing).length, [rows]);
    const disabledRows = useMemo(() => rows.filter((item) => !item.enabled).map((item) => item.model).filter(Boolean), [rows]);
    const advancedRow = useMemo(() => rows.find((item) => item.key === advancedRowKey) || null, [advancedRowKey, rows]);

    const openModelTest = (row: ModelRow) => {
        const generation = row.capabilities[0] || "image";
        setModelTest({ row, generation, prompt: defaultTestPrompts[generation] });
        setModelTestResult(null);
    };

    const runModelTest = async () => {
        if (!modelTest) return;
        const row = modelTest.row;
        const modelName = row.model.trim();
        if (!modelName) {
            message.error("请先填写模型名称");
            return;
        }
        setTestingModel(true);
        setModelTestResult(null);
        try {
            const result = await testApiModel({
                model: modelName,
                generation: modelTest.generation,
                route: modelTestRoute(row, modelTest.generation),
                prompt: modelTest.prompt,
            });
            setModelTestResult(result);
            if (result.success) {
                message.success("模型测试成功，已写入渠道状态日志");
            } else {
                message.error(result.error_message || "模型测试失败");
            }
        } catch (err: any) {
            message.error(err?.message || "模型测试失败");
        } finally {
            setTestingModel(false);
        }
    };

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
                image_generate_route: "auto",
                image_edit_route: "auto",
                video_route: "auto",
                video_durations: "",
                video_customizable: false,
                pricing_mode: "per_unit",
            },
        ]);
    };

    const columns: ColumnsType<ModelRow> = [
        {
            title: "模型名称",
            dataIndex: "model",
            key: "model",
            width: 260,
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
            width: 140,
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
            title: "计费方式",
            dataIndex: "pricing_mode",
            key: "pricing_mode",
            width: 150,
            render: (_, record) => (
                <Select
                    value={record.pricing_mode || "per_unit"}
                    options={pricingModeOptions}
                    className="w-full"
                    disabled={!record.enabled}
                    onChange={(value) => updateRow(record.key, { pricing_mode: value, unit_type: value === "video_dynamic" ? "per_video_second" : record.unit_type })}
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
                    disabled={!record.enabled || record.pricing_mode === "video_dynamic"}
                    onChange={(value) => updateRow(record.key, { unit_type: value })}
                />
            ),
        },
        {
            title: "按次积分",
            dataIndex: "credits_per_unit",
            key: "credits_per_unit",
            width: 180,
            render: (_, record) => (
                <InputNumber
                    min={0}
                    className="w-full"
                    value={record.credits_per_unit}
                    disabled={!record.enabled || record.pricing_mode === "video_dynamic"}
                    placeholder={record.pricing_mode === "video_dynamic" ? "动态模式不需要" : "0 表示未定价"}
                    onChange={(value) => updateRow(record.key, { credits_per_unit: Number(value || 0) })}
                />
            ),
        },
        {
            title: "高级配置",
            key: "advanced",
            width: 120,
            render: (_, record) => (
                <Button size="small" onClick={() => setAdvancedRowKey(record.key)}>
                    编辑
                </Button>
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
                    ) : rowHasValidPricing(record) ? (
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
            width: 150,
            render: (_, record) => (
                <Space size={4}>
                    <Button size="small" type="text" icon={<PlayCircle className="size-4" />} disabled={!record.model.trim()} onClick={() => openModelTest(record)}>
                        测试
                    </Button>
                    <Button danger type="text" icon={<Trash2 className="size-4" />} onClick={() => removeRow(record.key)} />
                </Space>
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
                description="只有配置了积分价格的模型，才会开放给用户使用。视频模型建议按上游文档显式选择接口路由；未指定时默认走 /v1/videos，不再仅凭模型名称猜测分支。"
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
                    <Table rowKey="key" columns={columns} dataSource={rows} pagination={false} scroll={{ x: 1500 }} />
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
            <Modal
                title={`高级配置${advancedRow?.model ? `：${advancedRow.model}` : ""}`}
                open={Boolean(advancedRow)}
                width={760}
                onCancel={() => setAdvancedRowKey(null)}
                footer={
                    <Button type="primary" onClick={() => setAdvancedRowKey(null)}>
                        完成
                    </Button>
                }
            >
                {advancedRow ? (
                    <div className="space-y-5">
                        <Alert type="info" showIcon message="这里的修改会先暂存在表格中，点击页面底部“保存全部配置”后才会写入后台。" />
                        <section className="rounded-xl border border-stone-200 p-4 dark:border-stone-700">
                            <div className="mb-3 text-sm font-semibold text-stone-900 dark:text-stone-100">接口路由</div>
                            <div className="grid gap-4 md:grid-cols-2">
                                <label className="space-y-1.5">
                                    <span className="text-xs text-stone-500 dark:text-stone-400">文生图接口</span>
                                    <Select
                                        value={advancedRow.image_generate_route || "auto"}
                                        options={imageRouteOptions}
                                        className="w-full"
                                        disabled={!advancedRow.enabled || !advancedRow.capabilities.includes("image")}
                                        onChange={(value) => updateRow(advancedRow.key, { image_generate_route: value })}
                                    />
                                </label>
                                <label className="space-y-1.5">
                                    <span className="text-xs text-stone-500 dark:text-stone-400">图生图接口</span>
                                    <Select
                                        value={advancedRow.image_edit_route || "auto"}
                                        options={imageRouteOptions}
                                        className="w-full"
                                        disabled={!advancedRow.enabled || !advancedRow.capabilities.includes("image")}
                                        onChange={(value) => updateRow(advancedRow.key, { image_edit_route: value })}
                                    />
                                </label>
                                <label className="space-y-1.5">
                                    <span className="text-xs text-stone-500 dark:text-stone-400">视频接口</span>
                                    <Select
                                        value={advancedRow.video_route || "auto"}
                                        options={videoRouteOptions}
                                        className="w-full"
                                        disabled={!advancedRow.enabled || !advancedRow.capabilities.includes("video")}
                                        onChange={(value) => updateRow(advancedRow.key, { video_route: value })}
                                    />
                                </label>
                            </div>
                        </section>
                        <section className="rounded-xl border border-stone-200 p-4 dark:border-stone-700">
                            <div className="mb-3 text-sm font-semibold text-stone-900 dark:text-stone-100">视频时长</div>
                            <div className="grid gap-4 md:grid-cols-2">
                                <label className="space-y-1.5">
                                    <span className="text-xs text-stone-500 dark:text-stone-400">可选时长</span>
                                    <Input
                                        value={advancedRow.video_durations}
                                        placeholder="如 5,10"
                                        disabled={!advancedRow.enabled || !advancedRow.capabilities.includes("video")}
                                        onChange={(event) => updateRow(advancedRow.key, { video_durations: event.target.value })}
                                    />
                                </label>
                                <label className="space-y-1.5">
                                    <span className="text-xs text-stone-500 dark:text-stone-400">允许用户自定义时长</span>
                                    <Select
                                        value={advancedRow.video_customizable ? "true" : "false"}
                                        options={[
                                            { label: "关闭", value: "false" },
                                            { label: "开启", value: "true" },
                                        ]}
                                        className="w-full"
                                        disabled={!advancedRow.enabled || !advancedRow.capabilities.includes("video")}
                                        onChange={(value) => updateRow(advancedRow.key, { video_customizable: value === "true" })}
                                    />
                                </label>
                            </div>
                        </section>
                        <section className="rounded-xl border border-stone-200 p-4 dark:border-stone-700">
                            <div className="mb-3 flex items-center justify-between gap-3">
                                <div className="text-sm font-semibold text-stone-900 dark:text-stone-100">动态视频计费</div>
                                {advancedRow.pricing_mode === "video_dynamic" ? <Tag color="blue">已启用</Tag> : <Tag>需先选择“视频动态”</Tag>}
                            </div>
                            <div className="space-y-4">
                                <label className="block max-w-xs space-y-1.5">
                                    <span className="text-xs text-stone-500 dark:text-stone-400">基础积分</span>
                                    <InputNumber
                                        min={0}
                                        className="w-full"
                                        value={advancedRow.video_base_credits}
                                        disabled={!advancedRow.enabled || advancedRow.pricing_mode !== "video_dynamic"}
                                        placeholder="0"
                                        onChange={(value) => updateRow(advancedRow.key, { video_base_credits: Number(value || 0) })}
                                    />
                                </label>
                                <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
                                    {videoRateFields.map((field) => (
                                        <label key={field.key} className="rounded-lg border border-stone-200 bg-stone-50 px-3 py-2 dark:border-stone-700 dark:bg-stone-900/60">
                                            <span className="mb-2 block text-xs font-medium text-stone-500 dark:text-stone-400">{field.label} 秒单价</span>
                                            <InputNumber
                                                min={0}
                                                className="w-full"
                                                controls={false}
                                                value={advancedRow[field.key]}
                                                disabled={!advancedRow.enabled || advancedRow.pricing_mode !== "video_dynamic"}
                                                placeholder="0"
                                                onChange={(value) => updateRow(advancedRow.key, { [field.key]: Number(value || 0) } as Partial<ModelRow>)}
                                            />
                                        </label>
                                    ))}
                                </div>
                            </div>
                        </section>
                    </div>
                ) : null}
            </Modal>
            <Modal
                title={`模型测试${modelTest?.row.model ? `：${modelTest.row.model}` : ""}`}
                open={Boolean(modelTest)}
                width={780}
                onCancel={() => setModelTest(null)}
                footer={[
                    <Button key="cancel" onClick={() => setModelTest(null)}>
                        关闭
                    </Button>,
                    <Button key="run" type="primary" loading={testingModel} icon={<PlayCircle className="size-4" />} onClick={runModelTest}>
                        开始测试
                    </Button>,
                ]}
            >
                {modelTest ? (
                    <div className="space-y-4">
                        <Alert
                            type="warning"
                            showIcon
                            message="测试会真实调用上游 API"
                            description="本次测试不扣平台用户积分，但可能消耗上游 API 额度；成功和失败都会写入模型日志，并影响渠道状态统计。"
                        />
                        <div className="grid gap-4 md:grid-cols-2">
                            <label className="space-y-1.5">
                                <span className="text-xs text-stone-500 dark:text-stone-400">测试能力</span>
                                <Select
                                    value={modelTest.generation}
                                    options={(modelTest.row.capabilities.length ? modelTest.row.capabilities : (["image"] as ModelCapability[])).map((value) => ({
                                        label: capabilityOptions.find((item) => item.value === value)?.label || value,
                                        value,
                                    }))}
                                    className="w-full"
                                    onChange={(value) =>
                                        setModelTest((current) =>
                                            current ? { ...current, generation: value as ModelCapability, prompt: defaultTestPrompts[value as ModelCapability] } : current,
                                        )
                                    }
                                />
                            </label>
                            <div className="space-y-1.5">
                                <span className="text-xs text-stone-500 dark:text-stone-400">将测试的接口路由</span>
                                <div className="rounded-lg border border-stone-200 px-3 py-2 font-mono text-xs dark:border-stone-700">
                                    {modelTestRouteLabel(modelTest.row, modelTest.generation)}
                                </div>
                            </div>
                        </div>
                        <label className="block space-y-1.5">
                            <span className="text-xs text-stone-500 dark:text-stone-400">测试提示词</span>
                            <Input.TextArea
                                rows={3}
                                value={modelTest.prompt}
                                onChange={(event) => setModelTest((current) => (current ? { ...current, prompt: event.target.value } : current))}
                            />
                        </label>
                        {modelTestResult ? (
                            <Card
                                size="small"
                                title={
                                    <Space>
                                        <Tag color={modelTestResult.success ? "green" : "red"}>{modelTestResult.success ? "成功" : "失败"}</Tag>
                                        <span>{modelTestResult.method} {modelTestResult.path}</span>
                                    </Space>
                                }
                            >
                                <div className="mb-3 grid gap-3 text-sm md:grid-cols-3">
                                    <div>
                                        <div className="text-xs text-stone-500">HTTP 状态</div>
                                        <div className="font-medium">{modelTestResult.status_code || "本地错误"}</div>
                                    </div>
                                    <div>
                                        <div className="text-xs text-stone-500">耗时</div>
                                        <div className="font-medium">{modelTestResult.response_time_ms} ms</div>
                                    </div>
                                    <div>
                                        <div className="text-xs text-stone-500">路由</div>
                                        <div className="font-medium">{modelTestResult.route || "auto"}</div>
                                    </div>
                                </div>
                                {modelTestResult.error_message ? (
                                    <Alert className="mb-3" type="error" showIcon message={modelTestResult.error_message} />
                                ) : null}
                                <Typography.Paragraph className="!mb-0 max-h-72 overflow-auto rounded-lg bg-stone-950 p-3 !font-mono !text-xs !text-stone-100">
                                    {modelTestResult.response_body || "无响应内容"}
                                </Typography.Paragraph>
                            </Card>
                        ) : null}
                    </div>
                ) : null}
            </Modal>
        </div>
    );
}

function buildRows(config: ApiConfigInfo, pricing: PricingItem[]): ModelRow[] {
    const orderedModels = Array.from(new Set([...(config.models || []), ...pricing.map((item) => item.model)]));
    const pricingMap = new Map(pricing.map((item) => [item.model, item]));
    return orderedModels.map((model, index) => {
        const pricingItem = pricingMap.get(model);
        const pricingRule = parseVideoPricingRule(pricingItem?.pricing_rule);
        return {
            key: `${model}-${index}`,
            model,
            enabled: pricingItem ? pricingItemHasValidPricing(pricingItem) : false,
            capabilities: collectCapabilities(model, config),
            credits_per_unit: pricingItem?.credits_per_unit,
            unit_type: pricingItem?.unit_type || inferUnitType(model, config),
            pricing_mode: pricingItem?.pricing_mode || "per_unit",
            ...pricingRule,
            image_generate_route: config.model_routes?.[`image_generate:${model}`] || "auto",
            image_edit_route: config.model_routes?.[`image_edit:${model}`] || config.model_routes?.[`image:${model}`] || "auto",
            video_route: config.model_routes?.[`video:${model}`] || config.model_routes?.[model] || "auto",
            video_durations: formatDurationInput(config.model_video_durations?.[model]),
            video_customizable: Boolean(config.model_video_customizable?.[model]),
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
            pricing_mode: item.pricing_mode === "video_dynamic" ? "video_dynamic" : "per_unit",
            unit_type: item.pricing_mode === "video_dynamic" ? "per_video_second" : item.unit_type,
            image_generate_route: item.capabilities.includes("image") ? item.image_generate_route || "auto" : "auto",
            image_edit_route: item.capabilities.includes("image") ? item.image_edit_route || "auto" : "auto",
            video_route: item.capabilities.includes("video") ? item.video_route || "auto" : "auto",
            video_durations: item.capabilities.includes("video") ? formatDurationInput(parseDurationInput(item.video_durations)) : "",
            video_customizable: item.capabilities.includes("video") ? item.video_customizable === true : false,
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

function parseDurationInput(value: string) {
    return Array.from(
        new Set(
            String(value || "")
                .split(",")
                .map((item) => Math.floor(Number(item.trim()) || 0))
                .filter((item) => item > 0),
        ),
    ).sort((left, right) => left - right);
}

function formatDurationInput(values?: number[]) {
    return (values || []).join(",");
}

function parseVideoPricingRule(value?: string): Partial<ModelRow> {
    if (!value) return {};
    try {
        const parsed = JSON.parse(value) as {
            base_credits?: number;
            resolution_second_rates?: Record<string, number>;
        };
        const rates = parsed.resolution_second_rates || {};
        return {
            video_base_credits: Number(parsed.base_credits || 0),
            video_rate_480p: Number(rates["480p"] || 0),
            video_rate_720p: Number(rates["720p"] || 0),
            video_rate_1080p: Number(rates["1080p"] || 0),
            video_rate_2k: Number(rates["2K"] || rates["2k"] || 0),
            video_rate_4k: Number(rates["4K"] || rates["4k"] || 0),
        };
    } catch {
        return {};
    }
}

function buildVideoPricingRule(row: ModelRow) {
    const rates: Record<string, number> = {};
    [
        ["480p", row.video_rate_480p],
        ["720p", row.video_rate_720p],
        ["1080p", row.video_rate_1080p],
        ["2K", row.video_rate_2k],
        ["4K", row.video_rate_4k],
    ].forEach(([label, value]) => {
        const rate = Number(value || 0);
        if (rate > 0) rates[String(label)] = rate;
    });
    return JSON.stringify({
        base_credits: Number(row.video_base_credits || 0),
        resolution_second_rates: rates,
    });
}

function hasVideoRate(row: ModelRow) {
    return [row.video_rate_480p, row.video_rate_720p, row.video_rate_1080p, row.video_rate_2k, row.video_rate_4k].some((value) => Number(value || 0) > 0);
}

function rowHasValidPricing(row: ModelRow) {
    if (!row.enabled) return false;
    if (row.pricing_mode === "video_dynamic") return hasVideoRate(row);
    return Number(row.credits_per_unit || 0) > 0;
}

function pricingItemHasValidPricing(item: PricingItem) {
    if (item.pricing_mode === "video_dynamic" || item.unit_type === "per_video_second") {
        return hasVideoRate(parseVideoPricingRule(item.pricing_rule) as ModelRow);
    }
    return Number(item.credits_per_unit || 0) > 0;
}

function modelTestRoute(row: ModelRow, generation: ModelCapability) {
    if (generation === "image") return row.image_generate_route || "auto";
    if (generation === "video") return row.video_route || "auto";
    return "";
}

function modelTestRouteLabel(row: ModelRow, generation: ModelCapability) {
    if (generation === "image") {
        const route = row.image_generate_route || "auto";
        return imageRouteOptions.find((item) => item.value === route)?.label || route;
    }
    if (generation === "video") {
        const route = row.video_route || "auto";
        return videoRouteOptions.find((item) => item.value === route)?.label || route;
    }
    if (generation === "audio") return "/v1/audio/speech";
    return "/v1/chat/completions";
}
