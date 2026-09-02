"use client";

import dynamic from "next/dynamic";
import { useCallback, useDeferredValue, useEffect, useMemo, useState } from "react";
import { App, Button, Checkbox, Drawer, Form, Input, InputNumber, Popconfirm, Segmented, Select, Space, Switch, Table, Tag, Tooltip, Typography } from "antd";
import { Archive, CircleAlert, Pencil, Play, RefreshCw, RotateCcw, Save } from "lucide-react";

import { normalizeAndValidateCustomVideoConfig, type CustomVideoConfig } from "@/lib/custom-video-config";
import {
    listModelConfigs,
    setModelConfigArchived,
    testModelConfig,
    updateModelConfig,
    type ModelCapability,
    type ModelConfig,
    type ModelOperation,
    type ModelPricingRule,
    type ModelStatus,
    type SaveModelOperationInput,
    type SaveModelPricingInput,
    type UpdateModelConfigInput,
} from "@/services/api/model-service";
import type { ApiModelTestResult } from "@/services/api/api-config";

const DynamicVideoFields = dynamic(() => import("./model-video-config-fields").then((module) => module.ModelVideoConfigFields), { ssr: false });

const operationDefinitions = [
    { key: "image:generate", capability: "image", operation: "generate", label: "图片生成", adapters: ["generations", "chat", "banana"] },
    { key: "image:edit", capability: "image", operation: "edit", label: "图片编辑", adapters: ["generations", "edits", "chat", "banana"] },
    { key: "video:generate", capability: "video", operation: "generate", label: "视频生成", adapters: [] },
    { key: "text:generate", capability: "text", operation: "generate", label: "文本生成", adapters: ["openai"] },
    { key: "audio:generate", capability: "audio", operation: "generate", label: "音频生成", adapters: ["openai"] },
] as const;

const capabilityLabels: Record<ModelCapability, string> = { image: "图片", video: "视频", text: "文本", audio: "音频" };
const statusLabels: Record<ModelStatus, string> = { draft: "待配置", active: "已启用", disabled: "已停用" };

type ViewMode = "channel" | "model";
type Props = { readonly view: ViewMode; readonly onViewChange: (view: ViewMode) => void; readonly refreshToken: number; readonly onChanged: () => void };
type ModelFormValues = { public_key: string; display_name: string; upstream_model_id: string; status: ModelStatus; sort_order: number; video_route?: string; video_durations?: string; video_customizable?: boolean; video_custom_config?: CustomVideoConfig };
type PricingDraft = SaveModelPricingInput & { override: boolean; effective_source?: string };

export function ModelServiceModels({ view, onViewChange, refreshToken, onChanged }: Props) {
    const { message } = App.useApp();
    const [models, setModels] = useState<ModelConfig[]>([]);
    const [loading, setLoading] = useState(true);
    const [search, setSearch] = useState("");
    const deferredSearch = useDeferredValue(search);
    const [capability, setCapability] = useState("");
    const [status, setStatus] = useState("");
    const [readiness, setReadiness] = useState("");
    const [includeArchived, setIncludeArchived] = useState(false);
    const [editing, setEditing] = useState<ModelConfig>();

    const load = useCallback(async () => {
        setLoading(true);
        try {
            setModels(await listModelConfigs({ search: deferredSearch, capability, status, includeArchived }));
        } catch (error) {
            message.error(errorMessage(error, "读取模型目录失败"));
        } finally {
            setLoading(false);
        }
    }, [capability, deferredSearch, includeArchived, message, status]);

    useEffect(() => {
        void load();
    }, [load, refreshToken]);

    const visibleModels = useMemo(() => models.filter((item) => {
        if (readiness === "ready") return item.ready;
        if (readiness === "issues") return !item.ready;
        if (readiness === "legacy") return item.legacy_unreviewed;
        if (readiness === "missing") return item.discovery_status === "missing";
        return true;
    }), [models, readiness]);

    const channelGroups = useMemo(() => groupModelsByChannel(visibleModels), [visibleModels]);
    const catalogGroups = useMemo(() => groupModelsByCatalog(visibleModels), [visibleModels]);

    const archive = async (item: ModelConfig, archived: boolean) => {
        try {
            await setModelConfigArchived(item.id, archived);
            message.success(archived ? "模型已归档" : "模型已恢复为停用状态");
            await load();
            onChanged();
        } catch (error) {
            message.error(errorMessage(error, archived ? "归档模型失败" : "恢复模型失败"));
        }
    };

    const modelTable = (items: ModelConfig[]) => (
        <Table
            rowKey="id"
            size="small"
            pagination={false}
            dataSource={items}
            onRow={(item) => ({ onDoubleClick: () => setEditing(item) })}
            columns={[
                { title: "显示名称", dataIndex: "display_name", render: (value, item) => <div><div className="font-medium">{value}</div><Typography.Text type="secondary" className="text-xs">{item.public_key}</Typography.Text></div> },
                { title: "上游模型 ID", dataIndex: "upstream_model_id", ellipsis: true },
                { title: "渠道", dataIndex: "channel_name", width: 150 },
                { title: "能力", width: 210, render: (_, item) => operationCapabilities(item).map((value) => <Tag key={value}>{capabilityLabels[value]}</Tag>) },
                { title: "状态", width: 100, render: (_, item) => <StatusTag item={item} /> },
                { title: "就绪", width: 100, render: (_, item) => item.ready ? <Tag color="green">可用</Tag> : <Tooltip title={item.readiness_issues.map((issue) => issue.message).join("；")}><Tag icon={<CircleAlert className="size-3" />} color="warning">{item.readiness_issues.length} 项</Tag></Tooltip> },
                {
                    title: "操作",
                    width: 100,
                    render: (_, item) => (
                        <Space size={2}>
                            <Tooltip title="编辑模型"><Button type="text" icon={<Pencil className="size-4" />} disabled={item.archived} onClick={() => setEditing(item)} /></Tooltip>
                            {item.archived ? <Tooltip title="恢复模型"><Button type="text" icon={<RotateCcw className="size-4" />} onClick={() => void archive(item, false)} /></Tooltip> : <Popconfirm title="归档该模型？" onConfirm={() => void archive(item, true)}><Tooltip title="归档模型"><Button type="text" danger icon={<Archive className="size-4" />} /></Tooltip></Popconfirm>}
                        </Space>
                    ),
                },
            ]}
        />
    );

    return (
        <div className="space-y-4">
            <div className="flex flex-wrap items-center justify-between gap-3">
                <div>
                    <Typography.Title level={5} className="!mb-0">模型目录</Typography.Title>
                    <Typography.Text type="secondary">公开调用键面向业务保持稳定，上游模型 ID 和显示名称可独立维护。</Typography.Text>
                </div>
                <Space>
                    <Segmented value={view} options={[{ label: "渠道视角", value: "channel" }, { label: "模型视角", value: "model" }]} onChange={(value) => onViewChange(value as ViewMode)} />
                    <Button icon={<RefreshCw className="size-4" />} loading={loading} onClick={() => void load()}>刷新</Button>
                </Space>
            </div>
            <div className="flex flex-wrap items-center gap-2">
                <Input.Search allowClear className="w-full sm:w-72" placeholder="搜索名称、调用键或上游 ID" value={search} onChange={(event) => setSearch(event.target.value)} />
                <Select allowClear className="w-32" placeholder="全部能力" value={capability || undefined} onChange={(value) => setCapability(value ?? "")} options={Object.entries(capabilityLabels).map(([value, label]) => ({ value, label }))} />
                <Select allowClear className="w-32" placeholder="全部状态" value={status || undefined} onChange={(value) => setStatus(value ?? "")} options={Object.entries(statusLabels).map(([value, label]) => ({ value, label }))} />
                <Select allowClear className="w-36" placeholder="全部就绪情况" value={readiness || undefined} onChange={(value) => setReadiness(value ?? "")} options={[{ value: "ready", label: "已就绪" }, { value: "issues", label: "有配置问题" }, { value: "legacy", label: "旧配置待复核" }, { value: "missing", label: "上游已缺失" }]} />
                <Checkbox checked={includeArchived} onChange={(event) => setIncludeArchived(event.target.checked)}>显示已归档</Checkbox>
            </div>

            {view === "channel" ? (
                <Table
                    rowKey="id"
                    loading={loading}
                    dataSource={channelGroups}
                    pagination={false}
                    expandable={{ defaultExpandAllRows: true, expandedRowRender: (group) => modelTable(group.models) }}
                    columns={[
                        { title: "渠道", dataIndex: "name", render: (value) => <span className="font-medium">{value}</span> },
                        { title: "模型", width: 100, render: (_, group) => group.models.length },
                        { title: "已就绪", width: 100, render: (_, group) => group.models.filter((item) => item.ready).length },
                        { title: "待复核", width: 100, render: (_, group) => group.models.filter((item) => item.legacy_unreviewed).length },
                    ]}
                />
            ) : (
                <Table
                    rowKey="id"
                    loading={loading}
                    dataSource={catalogGroups}
                    pagination={{ pageSize: 20, hideOnSinglePage: true }}
                    expandable={{ expandedRowRender: (group) => modelTable(group.models) }}
                    columns={[
                        { title: "公开模型", render: (_, group) => <div><div className="font-medium">{group.displayName}</div><Typography.Text type="secondary" className="text-xs">{group.publicKey}</Typography.Text></div> },
                        { title: "渠道实现", width: 120, render: (_, group) => group.models.length },
                        { title: "可用实现", width: 120, render: (_, group) => group.models.filter((item) => item.status === "active" && item.ready).length },
                        { title: "覆盖渠道", width: 280, render: (_, group) => group.models.map((item) => <Tag key={item.id}>{item.channel_name}</Tag>) },
                    ]}
                />
            )}

            <ModelConfigDrawer item={editing} onClose={() => setEditing(undefined)} onSaved={async () => { setEditing(undefined); await load(); onChanged(); }} />
        </div>
    );
}

function ModelConfigDrawer({ item, onClose, onSaved }: { readonly item?: ModelConfig; readonly onClose: () => void; readonly onSaved: () => Promise<void> }) {
    const { message, modal } = App.useApp();
    const [form] = Form.useForm<ModelFormValues>();
    const [operations, setOperations] = useState<SaveModelOperationInput[]>([]);
    const [pricing, setPricing] = useState<Record<ModelCapability, PricingDraft>>(emptyPricing);
    const [dirty, setDirty] = useState(false);
    const [saving, setSaving] = useState(false);
    const [testing, setTesting] = useState(false);
    const [testOperation, setTestOperation] = useState("text:generate");
    const [testPrompt, setTestPrompt] = useState("");
    const [testResult, setTestResult] = useState<ApiModelTestResult>();
    const videoOperation = operations.find((value) => value.capability === "video" && value.operation === "generate");

    useEffect(() => {
        if (!item) return;
        const normalizedOperations = completeOperations(item.operations);
        setOperations(normalizedOperations);
        setPricing(pricingDrafts(item.pricing));
        const video = normalizedOperations.find((value) => value.capability === "video");
        const videoConfig = (video?.mode === "override" ? video.config : item.operations.find((value) => value.capability === "video")?.effective.config) ?? {};
        form.setFieldsValue({
            public_key: item.public_key,
            display_name: item.display_name,
            upstream_model_id: item.upstream_model_id,
            status: item.status,
            sort_order: item.sort_order,
            video_route: video?.adapter || item.operations.find((value) => value.capability === "video")?.effective.adapter || "auto",
            video_durations: arrayValue(videoConfig.durations).join(","),
            video_customizable: Boolean(videoConfig.customizable),
            video_custom_config: videoConfig.custom_config as CustomVideoConfig | undefined,
        });
        const firstEnabled = normalizedOperations.find((value) => value.enabled);
        setTestOperation(firstEnabled ? `${firstEnabled.capability}:${firstEnabled.operation}` : "text:generate");
        setTestPrompt("");
        setTestResult(undefined);
        setDirty(false);
    }, [form, item]);

    const close = () => {
        if (!dirty) {
            onClose();
            return;
        }
        modal.confirm({ title: "放弃未保存的修改？", content: "关闭后，本次模型配置和测试草稿不会保留。", okText: "放弃修改", cancelText: "继续编辑", onOk: onClose });
    };

    const buildDraft = async (): Promise<UpdateModelConfigInput> => {
        if (!item) throw new Error("模型不存在");
        const values = await form.validateFields();
        const normalizedOperations = operations.map((operation) => {
            if (operation.capability !== "video" || operation.operation !== "generate" || operation.mode !== "override") return operation;
            const route = values.video_route?.trim() || "auto";
            const config: Record<string, unknown> = { ...operation.config };
            if (route === "custom") {
                const result = normalizeAndValidateCustomVideoConfig(values.video_custom_config);
                if (!result.ok) throw new Error("自定义视频配置无效");
                config.custom_config = result.config;
                config.customizable = false;
                config.durations = [];
            } else {
                config.durations = parseDurations(values.video_durations);
                config.customizable = Boolean(values.video_customizable);
                delete config.custom_config;
            }
            return { ...operation, adapter: route, config };
        });
        return {
            expected_revision: item.config_revision,
            public_key: values.public_key.trim(),
            display_name: values.display_name.trim(),
            upstream_model_id: values.upstream_model_id.trim(),
            status: values.status,
            sort_order: Number(values.sort_order || 0),
            operations: normalizedOperations,
            pricing_overrides: Object.values(pricing).filter((value) => value.override).map(({ override: _override, effective_source: _source, ...value }) => value),
        };
    };

    const save = async () => {
        if (!item) return;
        setSaving(true);
        try {
            await updateModelConfig(item.id, await buildDraft());
            setDirty(false);
            message.success("模型身份、协议和定价已原子保存");
            await onSaved();
        } catch (error) {
            message.error(errorMessage(error, "保存模型配置失败"));
        } finally {
            setSaving(false);
        }
    };

    const runTest = async () => {
        if (!item) return;
        const [capability, operation] = testOperation.split(":") as [ModelCapability, string];
        setTesting(true);
        try {
            const result = await testModelConfig(item.id, { capability, operation, prompt: testPrompt, reference_count: operation === "edit" ? 1 : 0, draft: await buildDraft() });
            setTestResult(result);
            if (result.success) message.success("草稿协议测试成功");
        } catch (error) {
            message.error(errorMessage(error, "模型测试失败"));
        } finally {
            setTesting(false);
        }
    };

    return (
        <Drawer
            width={760}
            open={Boolean(item)}
            title={<div><div>{item?.display_name}</div><Typography.Text type="secondary" className="text-xs">{item?.channel_name}</Typography.Text></div>}
            onClose={close}
            extra={<Space><Button icon={<Play className="size-4" />} loading={testing} onClick={() => void runTest()}>测试草稿</Button><Button type="primary" icon={<Save className="size-4" />} loading={saving} onClick={() => void save()}>保存</Button></Space>}
        >
            {item ? (
                <Form form={form} layout="vertical" onValuesChange={() => setDirty(true)}>
                <div className="space-y-6">
                    {item.readiness_issues.length ? <div className="border-l-2 border-amber-500 pl-3 text-sm"><div className="font-medium">启用前需处理</div>{item.readiness_issues.map((issue) => <div key={`${issue.code}:${issue.capability}:${issue.operation}`} className="text-stone-500">{issue.message}</div>)}</div> : null}

                    <Section title="模型身份">
                        <div className="grid gap-3 md:grid-cols-2">
                            <Form.Item name="public_key" label="公开调用键" rules={[{ required: true, whitespace: true }]}><Input disabled={item.status !== "draft"} /></Form.Item>
                            <Form.Item name="display_name" label="显示名称" rules={[{ required: true, whitespace: true }]}><Input /></Form.Item>
                            <Form.Item name="upstream_model_id" label="上游模型 ID" rules={[{ required: true, whitespace: true }]}><Input /></Form.Item>
                            <Form.Item name="sort_order" label="排序"><InputNumber className="w-full" min={0} /></Form.Item>
                            <Form.Item name="status" label="状态" className="md:col-span-2"><Segmented block options={Object.entries(statusLabels).map(([value, label]) => ({ value, label }))} /></Form.Item>
                        </div>
                    </Section>

                    <Section title="能力与协议">
                        <Table
                            rowKey={(value) => `${value.capability}:${value.operation}`}
                            size="small"
                            pagination={false}
                            dataSource={operations}
                            columns={[
                                { title: "操作", width: 120, render: (_, value) => operationDefinitions.find((item) => item.key === `${value.capability}:${value.operation}`)?.label },
                                { title: "启用", width: 70, render: (_, value, index) => <Switch size="small" checked={value.enabled} onChange={(enabled) => updateOperation(index, { enabled }, setOperations, setDirty)} /> },
                                { title: "协议来源", width: 130, render: (_, value, index) => <Select className="w-full" value={value.mode} options={[{ value: "inherit", label: "继承渠道" }, { value: "override", label: "模型覆盖" }]} onChange={(mode) => updateOperation(index, { mode, adapter: mode === "inherit" ? "" : value.adapter || item.operations.find((current) => current.capability === value.capability && current.operation === value.operation)?.effective.adapter || defaultAdapter(value.capability, value.operation) }, setOperations, setDirty)} /> },
                                {
                                    title: "适配器",
                                    render: (_, value, index) => {
                                        if (value.mode === "inherit") return <Typography.Text type="secondary">{item.operations.find((current) => current.capability === value.capability && current.operation === value.operation)?.effective.adapter || "未配置"}</Typography.Text>;
                                        if (value.capability === "video") return <Typography.Text type="secondary">在下方配置视频协议</Typography.Text>;
                                        const definition = operationDefinitions.find((current) => current.key === `${value.capability}:${value.operation}`);
                                        return <Select className="w-full" value={value.adapter} options={(definition?.adapters ?? []).map((adapter) => ({ value: adapter, label: adapterLabel(adapter) }))} onChange={(adapter) => updateOperation(index, { adapter }, setOperations, setDirty)} />;
                                    },
                                },
                            ]}
                        />
                        {videoOperation?.enabled && videoOperation.mode === "override" ? <div className="mt-4"><DynamicVideoFields form={form} disabled={false} binghuo={false} /></div> : null}
                    </Section>

                    <Section title="渠道定价覆盖">
                        <PricingEditor operations={operations} pricing={pricing} onChange={(next) => { setPricing(next); setDirty(true); }} />
                    </Section>

                    <Section title="测试未保存草稿">
                        <div className="grid gap-3 md:grid-cols-[220px_1fr]">
                            <Select value={testOperation} onChange={setTestOperation} options={operations.filter((value) => value.enabled).map((value) => ({ value: `${value.capability}:${value.operation}`, label: operationDefinitions.find((item) => item.key === `${value.capability}:${value.operation}`)?.label }))} />
                            <Input value={testPrompt} onChange={(event) => setTestPrompt(event.target.value)} placeholder="测试提示词，可留空使用默认值" />
                        </div>
                        {testResult ? <div className={`mt-3 border-l-2 pl-3 text-sm ${testResult.success ? "border-emerald-500" : "border-red-500"}`}><div className="flex gap-3"><strong>{testResult.success ? "成功" : "失败"}</strong><span>{testResult.method} {testResult.path}</span><span>{testResult.response_time_ms}ms</span></div>{testResult.error_message ? <pre className="mt-2 max-h-40 overflow-auto whitespace-pre-wrap text-xs">{testResult.error_message}</pre> : null}</div> : null}
                    </Section>
                </div>
                </Form>
            ) : null}
        </Drawer>
    );
}

function PricingEditor({ operations, pricing, onChange }: { operations: SaveModelOperationInput[]; pricing: Record<ModelCapability, PricingDraft>; onChange: (value: Record<ModelCapability, PricingDraft>) => void }) {
    const capabilities = [...new Set(operations.filter((value) => value.enabled).map((value) => value.capability))];
    return (
        <Table
            rowKey="capability"
            size="small"
            pagination={false}
            dataSource={capabilities.map((capability) => pricing[capability])}
            locale={{ emptyText: "先启用至少一个模型能力" }}
            columns={[
                { title: "能力", width: 80, dataIndex: "capability", render: (value: ModelCapability) => capabilityLabels[value] },
                { title: "覆盖", width: 75, render: (_, value) => <Switch size="small" checked={value.override} onChange={(override) => onChange({ ...pricing, [value.capability]: { ...value, override } })} /> },
                { title: "积分", width: 110, render: (_, value) => <InputNumber min={0} className="w-full" disabled={!value.override} value={value.credits_per_unit} onChange={(credits) => onChange({ ...pricing, [value.capability]: { ...value, credits_per_unit: Number(credits || 0) } })} /> },
                { title: "单位", width: 150, render: (_, value) => <Select className="w-full" disabled={!value.override} value={value.unit_type} options={unitOptions(value.capability)} onChange={(unit_type) => onChange({ ...pricing, [value.capability]: { ...value, unit_type } })} /> },
                { title: "来源", width: 100, render: (_, value) => value.override ? <Tag color="blue">渠道覆盖</Tag> : value.effective_source === "default" ? <Tag>全局默认</Tag> : <Tag color="warning">未定价</Tag> },
            ]}
        />
    );
}

export function groupModelsByChannel(items: ModelConfig[]) {
    const grouped = new Map<number, { id: number; name: string; models: ModelConfig[] }>();
    for (const item of items) {
        const group = grouped.get(item.channel_id) ?? { id: item.channel_id, name: item.channel_name, models: [] };
        group.models.push(item);
        grouped.set(item.channel_id, group);
    }
    return [...grouped.values()];
}

export function groupModelsByCatalog(items: ModelConfig[]) {
    const grouped = new Map<number, { id: number; publicKey: string; displayName: string; models: ModelConfig[] }>();
    for (const item of items) {
        const group = grouped.get(item.catalog_model_id) ?? { id: item.catalog_model_id, publicKey: item.public_key, displayName: item.display_name, models: [] };
        group.models.push(item);
        grouped.set(item.catalog_model_id, group);
    }
    return [...grouped.values()];
}

export function completeOperations(items: ModelOperation[]): SaveModelOperationInput[] {
    const byKey = new Map(items.map((item) => [`${item.capability}:${item.operation}`, item]));
    return operationDefinitions.map((definition) => {
        const item = byKey.get(definition.key);
        return item ? { capability: item.capability, operation: item.operation, enabled: item.enabled, mode: item.mode, adapter: item.adapter, config: structuredClone(item.config) } : { capability: definition.capability, operation: definition.operation, enabled: false, mode: "inherit", adapter: "", config: {} };
    });
}

const emptyPricing: Record<ModelCapability, PricingDraft> = {
    image: { capability: "image", override: false, credits_per_unit: 0, unit_type: "per_image", pricing_mode: "per_unit", pricing_rule: "" },
    video: { capability: "video", override: false, credits_per_unit: 0, unit_type: "per_video", pricing_mode: "per_unit", pricing_rule: "" },
    text: { capability: "text", override: false, credits_per_unit: 0, unit_type: "per_token", pricing_mode: "per_unit", pricing_rule: "" },
    audio: { capability: "audio", override: false, credits_per_unit: 0, unit_type: "per_token", pricing_mode: "per_unit", pricing_rule: "" },
};

export function pricingDrafts(items: ModelPricingRule[]) {
    const result = structuredClone(emptyPricing);
    for (const item of items) result[item.capability] = { capability: item.capability, override: item.effective_source === "implementation", credits_per_unit: item.credits_per_unit, unit_type: item.unit_type, pricing_mode: item.pricing_mode, pricing_rule: item.pricing_rule, effective_source: item.effective_source };
    return result;
}

function updateOperation(index: number, patch: Partial<SaveModelOperationInput>, setOperations: React.Dispatch<React.SetStateAction<SaveModelOperationInput[]>>, setDirty: (value: boolean) => void) {
    setOperations((current) => current.map((value, itemIndex) => itemIndex === index ? { ...value, ...patch } : value));
    setDirty(true);
}

function operationCapabilities(item: ModelConfig) {
    return [...new Set(item.operations.filter((value) => value.enabled).map((value) => value.capability))];
}

function StatusTag({ item }: { item: ModelConfig }) {
    if (item.archived) return <Tag>已归档</Tag>;
    if (item.discovery_status === "missing") return <Tag color="red">上游缺失</Tag>;
    if (item.status === "active") return <Tag color="green">已启用</Tag>;
    if (item.status === "draft") return <Tag color="gold">待配置</Tag>;
    return <Tag>已停用</Tag>;
}

function Section({ title, children }: { readonly title: string; readonly children: React.ReactNode }) {
    return <section><Typography.Title level={5} className="!mb-3 !text-sm">{title}</Typography.Title>{children}</section>;
}

function defaultAdapter(capability: ModelCapability, operation: string) {
    if (capability === "image") return operation === "edit" ? "edits" : "generations";
    if (capability === "video") return "openai";
    return "openai";
}

function adapterLabel(adapter: string) {
    const labels: Record<string, string> = { generations: "Images Generations", edits: "Images Edits", chat: "Chat 多模态", banana: "Banana", openai: "OpenAI" };
    return labels[adapter] ?? adapter;
}

function unitOptions(capability: ModelCapability) {
    if (capability === "image") return [{ value: "per_image", label: "每张图片" }];
    if (capability === "video") return [{ value: "per_video", label: "每个视频" }, { value: "per_video_second", label: "每视频秒" }];
    return [{ value: "per_token", label: "每 Token" }];
}

function parseDurations(value?: string) {
    return [...new Set((value ?? "").split(",").map((item) => Math.floor(Number(item.trim()))).filter((item) => item > 0))].sort((left, right) => left - right);
}

function arrayValue(value: unknown): unknown[] {
    return Array.isArray(value) ? value : [];
}

function errorMessage(error: unknown, fallback: string) {
    return error instanceof Error && error.message ? error.message : fallback;
}
