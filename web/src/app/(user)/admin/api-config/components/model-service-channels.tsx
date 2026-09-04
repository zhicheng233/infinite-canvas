"use client";

import { useCallback, useEffect, useState } from "react";
import { App, Button, Form, Input, InputNumber, Modal, Popconfirm, Segmented, Select, Space, Switch, Table, Tag, Tooltip, Typography } from "antd";
import { Archive, KeyRound, Pencil, Plus, RefreshCw, RotateCcw, Settings2 } from "lucide-react";

import {
    createModelServiceChannel,
    listModelServiceChannels,
    previewChannelProtocolDefaults,
    saveChannelProtocolDefaults,
    setModelServiceChannelArchived,
    syncModelServiceChannel,
    updateModelServiceChannel,
    type ModelServiceChannel,
    type ProtocolDefault,
    type SaveModelServiceChannelInput,
} from "@/services/api/model-service";
import { ChannelNameWithRemark } from "@/components/channel-name-with-remark";
import { ApiConfigTransfer } from "./api-config-transfer";

const protocolRows = [
    { key: "image:generate", capability: "image", operation: "generate", label: "图片生成", adapters: ["auto", "generations", "chat", "banana"] },
    { key: "image:edit", capability: "image", operation: "edit", label: "图片编辑", adapters: ["auto", "generations", "edits", "chat", "banana"] },
    { key: "video:generate", capability: "video", operation: "generate", label: "视频生成", adapters: ["auto", "openai", "veo_json", "waninter", "yijia", "xai", "newapi", "seedance", "binghuo"] },
    { key: "text:generate", capability: "text", operation: "generate", label: "文本生成", adapters: ["openai"] },
    { key: "audio:generate", capability: "audio", operation: "generate", label: "音频生成", adapters: ["openai"] },
] as const;

type Props = { readonly refreshToken: number; readonly onChanged: () => void };

export function ModelServiceChannels({ refreshToken, onChanged }: Props) {
    const { message, modal } = App.useApp();
    const [channels, setChannels] = useState<ModelServiceChannel[]>([]);
    const [loading, setLoading] = useState(true);
    const [busy, setBusy] = useState("");
    const [editing, setEditing] = useState<ModelServiceChannel>();
    const [channelOpen, setChannelOpen] = useState(false);
    const [defaultsChannel, setDefaultsChannel] = useState<ModelServiceChannel>();
    const [defaults, setDefaults] = useState<ProtocolDefault[]>([]);
    const [channelForm] = Form.useForm<SaveModelServiceChannelInput>();

    const load = useCallback(async () => {
        setLoading(true);
        try {
            setChannels(await listModelServiceChannels());
        } catch (error) {
            message.error(errorMessage(error, "读取渠道失败"));
        } finally {
            setLoading(false);
        }
    }, [message]);

    useEffect(() => {
        void load();
    }, [load, refreshToken]);

    const openChannel = (item?: ModelServiceChannel) => {
        setEditing(item);
        channelForm.setFieldsValue(
            item
                ? {
                      expected_revision: item.config_revision,
                      name: item.name,
                      base_url: item.base_url,
                      api_key: "",
                      enabled: item.enabled,
                      video_api_standard: item.video_api_standard,
                      new_api_channel_id: item.new_api_channel_id,
                      metrics_base_url: item.metrics_base_url,
                      remark: item.remark,
                  }
                : { name: "", base_url: "", api_key: "", enabled: true, video_api_standard: "default", remark: "" },
        );
        setChannelOpen(true);
    };

    const saveChannel = async (values: SaveModelServiceChannelInput) => {
        setBusy("channel");
        try {
            const payload = { ...values, expected_revision: editing?.config_revision, api_key: values.api_key?.trim() || undefined };
            if (editing) await updateModelServiceChannel(editing.id, payload);
            else await createModelServiceChannel(payload);
            message.success(editing ? "渠道已保存" : "渠道已创建");
            setChannelOpen(false);
            await load();
            onChanged();
        } catch (error) {
            message.error(errorMessage(error, "保存渠道失败"));
        } finally {
            setBusy("");
        }
    };

    const runSync = async (item: ModelServiceChannel) => {
        setBusy(`sync:${item.id}`);
        try {
            const report = await syncModelServiceChannel(item.id);
            message.success(`同步完成：新增 ${report.created}，恢复 ${report.restored}，缺失 ${report.missing}`);
            await load();
            onChanged();
        } catch (error) {
            message.error(errorMessage(error, "同步渠道模型失败"));
            await load();
        } finally {
            setBusy("");
        }
    };

    const archiveChannel = async (item: ModelServiceChannel, archived: boolean) => {
        setBusy(`archive:${item.id}`);
        try {
            await setModelServiceChannelArchived(item.id, archived);
            message.success(archived ? "渠道已归档" : "渠道已恢复为停用状态");
            await load();
            onChanged();
        } catch (error) {
            message.error(errorMessage(error, archived ? "归档渠道失败" : "恢复渠道失败"));
        } finally {
            setBusy("");
        }
    };

    const openDefaults = (item: ModelServiceChannel) => {
        const byKey = new Map(item.protocol_defaults.map((value) => [`${value.capability}:${value.operation}`, value]));
        setDefaults(
            protocolRows.map((row) =>
                byKey.get(row.key) ?? {
                    capability: row.capability,
                    operation: row.operation,
                    adapter: row.adapters[0],
                    config: {},
                },
            ),
        );
        setDefaultsChannel(item);
    };

    const saveDefaults = async () => {
        if (!defaultsChannel) return;
        setBusy("defaults");
        try {
            const impact = await previewChannelProtocolDefaults(defaultsChannel.id, defaultsChannel.config_revision, defaults);
            if (impact.issues.length) {
                modal.error({ title: "默认协议尚不完整", content: impact.issues.map((item) => item.message).join("；") });
                return;
            }
            await saveChannelProtocolDefaults(defaultsChannel.id, defaultsChannel.config_revision, defaults);
            message.success(`渠道默认协议已保存，影响 ${impact.affected_model_ids.length} 个继承模型`);
            setDefaultsChannel(undefined);
            await load();
            onChanged();
        } catch (error) {
            message.error(errorMessage(error, "保存渠道默认协议失败"));
        } finally {
            setBusy("");
        }
    };

    return (
        <div className="space-y-4">
            <div className="flex flex-wrap items-center justify-between gap-3">
                <div>
                    <Typography.Title level={5} className="!mb-0">渠道</Typography.Title>
                    <Typography.Text type="secondary">维护接入凭据和渠道级默认协议；同步不会删除上游暂时缺失的模型。</Typography.Text>
                </div>
                <Space wrap>
                    <ApiConfigTransfer disabled={false} onImported={async () => { await load(); onChanged(); }} />
                    <Button icon={<RefreshCw className="size-4" />} loading={loading} onClick={() => void load()}>刷新</Button>
                    <Button type="primary" icon={<Plus className="size-4" />} onClick={() => openChannel()}>新建渠道</Button>
                </Space>
            </div>

            <Table
                rowKey="id"
                loading={loading}
                dataSource={channels}
                pagination={{ pageSize: 15, hideOnSinglePage: true }}
                rowClassName={(item) => (item.archived ? "opacity-60" : "")}
                columns={[
                    {
                        title: "渠道",
                        render: (_, item) => (
                            <div className="min-w-48">
                                <div className="flex items-center gap-2 font-medium"><ChannelNameWithRemark name={item.name} remark={item.remark} />{item.archived ? <Tag>已归档</Tag> : item.enabled ? <Tag color="green">已启用</Tag> : <Tag>已停用</Tag>}</div>
                                <Typography.Text type="secondary" className="block max-w-80 truncate text-xs">{item.base_url}</Typography.Text>
                                {item.remark ? <Tooltip title={item.remark}><Typography.Text type="secondary" className="block max-w-80 truncate text-xs">备注：{item.remark}</Typography.Text></Tooltip> : null}
                            </div>
                        ),
                    },
                    { title: "密钥", width: 80, render: (_, item) => item.has_key ? <Tooltip title="密钥已加密保存"><KeyRound className="size-4 text-emerald-600" /></Tooltip> : <Tag color="red">缺失</Tag> },
                    { title: "模型就绪", width: 120, render: (_, item) => `${item.ready_model_count}/${item.model_count}` },
                    { title: "默认协议", width: 110, render: (_, item) => `${item.protocol_defaults.length}/5` },
                    {
                        title: "同步状态",
                        width: 150,
                        render: (_, item) => item.sync_status === "failed" ? <Tooltip title={item.sync_error}><Tag color="red">同步失败</Tag></Tooltip> : item.sync_status === "syncing" ? <Tag color="processing">同步中</Tag> : item.synced_at ? <Tag color="green">已同步</Tag> : <Tag>未同步</Tag>,
                    },
                    {
                        title: "操作",
                        width: 190,
                        render: (_, item) => (
                            <Space size={2}>
                                <Tooltip title="编辑渠道"><Button type="text" icon={<Pencil className="size-4" />} disabled={item.archived} onClick={() => openChannel(item)} /></Tooltip>
                                <Tooltip title="默认协议"><Button type="text" icon={<Settings2 className="size-4" />} disabled={item.archived} onClick={() => openDefaults(item)} /></Tooltip>
                                <Tooltip title="同步模型"><Button type="text" icon={<RefreshCw className="size-4" />} loading={busy === `sync:${item.id}`} disabled={item.archived} onClick={() => void runSync(item)} /></Tooltip>
                                {item.archived ? (
                                    <Tooltip title="恢复渠道"><Button type="text" icon={<RotateCcw className="size-4" />} loading={busy === `archive:${item.id}`} onClick={() => void archiveChannel(item, false)} /></Tooltip>
                                ) : (
                                    <Popconfirm title="归档该渠道？" description="渠道及其模型将从业务列表隐藏，但不会删除。" onConfirm={() => void archiveChannel(item, true)}><Tooltip title="归档渠道"><Button type="text" danger icon={<Archive className="size-4" />} /></Tooltip></Popconfirm>
                                )}
                            </Space>
                        ),
                    },
                ]}
            />

            <Modal destroyOnHidden open={channelOpen} title={editing ? "编辑渠道" : "新建渠道"} okText="保存" confirmLoading={busy === "channel"} onCancel={() => setChannelOpen(false)} onOk={() => channelForm.submit()}>
                <Form form={channelForm} layout="vertical" preserve={false} onFinish={(values) => void saveChannel(values)}>
                    <Form.Item name="name" label="渠道名称" rules={[{ required: true, whitespace: true, message: "请输入渠道名称" }]}><Input maxLength={100} /></Form.Item>
                    <Form.Item name="base_url" label="Base URL" rules={[{ required: true, type: "url", message: "请输入有效的 HTTP(S) 地址" }]}><Input placeholder="https://api.example.com/v1" /></Form.Item>
                    <Form.Item name="api_key" label={editing ? "API Key（留空保持不变）" : "API Key"} rules={editing ? [] : [{ required: true, whitespace: true, message: "请输入 API Key" }]}><Input.Password autoComplete="new-password" /></Form.Item>
                    <Form.Item name="video_api_standard" label="视频 API 标准"><Segmented block options={[{ label: "默认标准", value: "default" }, { label: "炳火标准", value: "binghuo" }]} /></Form.Item>
                    <div className="grid grid-cols-2 gap-3">
                        <Form.Item name="new_api_channel_id" label="指标渠道 ID"><InputNumber className="w-full" min={1} /></Form.Item>
                        <Form.Item name="metrics_base_url" label="指标服务地址"><Input placeholder="可选" /></Form.Item>
                    </div>
                    <Form.Item name="remark" label="备注"><Input.TextArea maxLength={500} showCount rows={3} /></Form.Item>
                    <Form.Item name="enabled" label="启用渠道" valuePropName="checked"><Switch /></Form.Item>
                </Form>
            </Modal>

            <Modal width={760} open={Boolean(defaultsChannel)} title={<span>渠道默认协议：<ChannelNameWithRemark name={defaultsChannel?.name ?? ""} remark={defaultsChannel?.remark} /></span>} okText="预览并保存" confirmLoading={busy === "defaults"} onCancel={() => setDefaultsChannel(undefined)} onOk={() => void saveDefaults()}>
                <Table
                    rowKey={(item) => `${item.capability}:${item.operation}`}
                    pagination={false}
                    size="small"
                    dataSource={defaults}
                    columns={[
                        { title: "操作", width: 130, render: (_, item) => protocolRows.find((row) => row.key === `${item.capability}:${item.operation}`)?.label },
                        {
                            title: "默认适配器",
                            render: (_, item, index) => {
                                const options = protocolRows.find((row) => row.key === `${item.capability}:${item.operation}`)?.adapters ?? [];
                                return <Select className="w-full" value={item.adapter} options={options.map((value) => ({ value, label: adapterLabel(value) }))} onChange={(adapter) => setDefaults((current) => current.map((value, itemIndex) => itemIndex === index ? { ...value, adapter } : value))} />;
                            },
                        },
                    ]}
                />
            </Modal>
        </div>
    );
}

function adapterLabel(value: string) {
    const labels: Record<string, string> = { auto: "兼容默认", generations: "Images Generations", edits: "Images Edits", chat: "Chat 多模态", banana: "Banana", openai: "OpenAI", veo_json: "Veo JSON", waninter: "Waninter", yijia: "Yijia", xai: "xAI", newapi: "New API", seedance: "Seedance", binghuo: "炳火", custom: "自定义视频" };
    return labels[value] ?? value;
}

function errorMessage(error: unknown, fallback: string) {
    return error instanceof Error && error.message ? error.message : fallback;
}
