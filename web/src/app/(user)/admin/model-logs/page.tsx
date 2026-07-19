"use client";

import { useCallback, useEffect, useState } from "react";
import { App, Button, Card, Col, Empty, Form, Input, InputNumber, Modal, Row, Select, Space, Statistic, Table, Tag, Typography } from "antd";
import type { ColumnsType } from "antd/es/table";
import { Activity, AlertTriangle, Clock3, RotateCcw, Search, TrendingUp } from "lucide-react";

import { getModelHealth, listModelCallLogs, type ModelCallLogItem, type ModelHealthSummary } from "@/services/api/admin";

const { Title, Text } = Typography;

const generationLabels: Record<string, string> = {
    image: "图片",
    video: "视频",
    audio: "音频",
    text: "文本",
};

const generationOptions = [
    { label: "全部类型", value: "" },
    { label: "图片", value: "image" },
    { label: "视频", value: "video" },
    { label: "音频", value: "audio" },
    { label: "文本", value: "text" },
];

type FilterValues = {
    model?: string;
    generation?: string;
    keyword?: string;
    userId?: number;
};

export default function AdminModelLogsPage() {
    const { message } = App.useApp();
    const [form] = Form.useForm<FilterValues>();
    const [logs, setLogs] = useState<ModelCallLogItem[]>([]);
    const [health, setHealth] = useState<ModelHealthSummary | null>(null);
    const [loading, setLoading] = useState(false);
    const [healthLoading, setHealthLoading] = useState(false);
    const [selected, setSelected] = useState<ModelCallLogItem | null>(null);
    const [pagination, setPagination] = useState({ current: 1, pageSize: 20, total: 0 });

    const fetchLogs = useCallback(
        async (page = pagination.current, pageSize = pagination.pageSize) => {
            setLoading(true);
            try {
                const values = form.getFieldsValue();
                const data = await listModelCallLogs({
                    page,
                    pageSize,
                    model: values.model?.trim(),
                    generation: values.generation,
                    keyword: values.keyword?.trim(),
                    userId: values.userId,
                });
                setLogs(data.items || []);
                setPagination({ current: data.page, pageSize: data.page_size, total: data.total });
            } catch (err: any) {
                message.error(err?.message || "获取模型失败日志失败");
            } finally {
                setLoading(false);
            }
        },
        [form, message, pagination.current, pagination.pageSize],
    );

    const fetchHealth = useCallback(async () => {
        setHealthLoading(true);
        try {
            setHealth(await getModelHealth());
        } catch (err: any) {
            message.error(err?.message || "获取模型健康数据失败");
        } finally {
            setHealthLoading(false);
        }
    }, [message]);

    useEffect(() => {
        void fetchLogs(1, pagination.pageSize);
        void fetchHealth();
    }, []);

    const columns: ColumnsType<ModelCallLogItem> = [
        {
            title: "时间",
            dataIndex: "created_at",
            key: "created_at",
            width: 180,
            render: (value: string) => new Date(value).toLocaleString("zh-CN"),
        },
        {
            title: "用户",
            key: "user",
            width: 170,
            render: (_, record) => (
                <div className="min-w-0">
                    <div className="truncate font-medium">{record.display_name || record.username || `用户 #${record.user_id}`}</div>
                    <div className="text-xs text-stone-500">
                        ID: {record.user_id}
                        {record.username ? ` · ${record.username}` : ""}
                    </div>
                </div>
            ),
        },
        {
            title: "类型",
            dataIndex: "generation",
            key: "generation",
            width: 90,
            render: (value: string) => <Tag>{generationLabels[value] || value || "-"}</Tag>,
        },
        {
            title: "模型",
            dataIndex: "model",
            key: "model",
            width: 210,
            ellipsis: true,
            render: (value: string) => value || "-",
        },
        {
            title: "接口",
            key: "path",
            width: 260,
            ellipsis: true,
            render: (_, record) => (
                <span className="font-mono text-xs">
                    {record.method} {record.path}
                </span>
            ),
        },
        {
            title: "状态",
            dataIndex: "status_code",
            key: "status_code",
            width: 90,
            render: (value: number) => <Tag color={value >= 500 || value === 0 ? "red" : "orange"}>{value || "本地"}</Tag>,
        },
        {
            title: "错误",
            dataIndex: "error_message",
            key: "error_message",
            ellipsis: true,
            render: (value: string) => value || "-",
        },
        {
            title: "操作",
            key: "action",
            width: 90,
            render: (_, record) => (
                <Button size="small" onClick={() => setSelected(record)}>
                    详情
                </Button>
            ),
        },
    ];

    return (
        <div>
            <div className="mb-4 flex items-center justify-between gap-3">
                <div>
                    <Title level={4} className="!mb-1 flex items-center gap-2">
                        <AlertTriangle className="size-5 text-amber-500" />
                        模型失败日志
                    </Title>
                    <Text type="secondary">仅记录失败调用，用于排查上游模型、计费和用户请求问题。</Text>
                </div>
            </div>

            <Row gutter={[16, 16]} className="mb-5">
                <Col xs={24} md={8}>
                    <Card loading={healthLoading}>
                        <Statistic title="近 24 小时失败" value={health?.total_24h ?? 0} prefix={<Clock3 className="size-5 text-orange-500" />} />
                    </Card>
                </Col>
                <Col xs={24} md={8}>
                    <Card loading={healthLoading}>
                        <Statistic title="近 7 天失败" value={health?.total_7d ?? 0} prefix={<TrendingUp className="size-5 text-red-500" />} />
                    </Card>
                </Col>
                <Col xs={24} md={8}>
                    <Card loading={healthLoading}>
                        <Statistic title="异常模型数" value={health?.top_models?.length ?? 0} prefix={<Activity className="size-5 text-blue-500" />} />
                    </Card>
                </Col>
            </Row>

            <Card
                className="mb-5"
                title="模型健康概览"
                extra={
                    <Button size="small" icon={<RotateCcw className="size-4" />} onClick={() => void fetchHealth()}>
                        刷新
                    </Button>
                }
                loading={healthLoading}
            >
                {health?.top_models?.length ? (
                    <div className="grid gap-3 lg:grid-cols-2">
                        {health.top_models.map((item) => (
                            <div key={`${item.generation}-${item.model}`} className="rounded-xl border border-stone-200 p-3 dark:border-stone-800">
                                <div className="mb-2 flex items-center justify-between gap-3">
                                    <div className="min-w-0">
                                        <div className="truncate font-medium">{item.model || "未知模型"}</div>
                                        <div className="text-xs text-stone-500">{generationLabels[item.generation] || item.generation || "未知类型"}</div>
                                    </div>
                                    <Tag color="red">{item.failures} 次失败</Tag>
                                </div>
                                <div className="line-clamp-2 text-xs leading-5 text-stone-500 dark:text-stone-400">{item.last_error || "暂无错误摘要"}</div>
                            </div>
                        ))}
                    </div>
                ) : (
                    <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="近 7 天暂无模型失败记录" />
                )}
            </Card>

            <Form form={form} layout="inline" className="mb-4 gap-y-3" initialValues={{ generation: "" }} onFinish={() => fetchLogs(1, pagination.pageSize)}>
                <Form.Item name="model" label="模型">
                    <Input allowClear placeholder="模型名" className="!w-52" />
                </Form.Item>
                <Form.Item name="generation" label="类型">
                    <Select options={generationOptions} className="!w-32" />
                </Form.Item>
                <Form.Item name="keyword" label="关键词">
                    <Input allowClear placeholder="错误 / 接口 / 用户名" className="!w-56" />
                </Form.Item>
                <Form.Item name="userId" label="用户ID">
                    <InputNumber min={1} placeholder="数字 ID" className="!w-32" />
                </Form.Item>
                <Form.Item>
                    <Space>
                        <Button type="primary" htmlType="submit" icon={<Search className="size-4" />}>
                            查询
                        </Button>
                        <Button
                            icon={<RotateCcw className="size-4" />}
                            onClick={() => {
                                form.resetFields();
                                void fetchLogs(1, pagination.pageSize);
                            }}
                        >
                            重置
                        </Button>
                    </Space>
                </Form.Item>
            </Form>

            <Table
                rowKey="id"
                columns={columns}
                dataSource={logs}
                loading={loading}
                scroll={{ x: 1220 }}
                pagination={{
                    ...pagination,
                    showSizeChanger: true,
                    showTotal: (total) => `共 ${total} 条失败日志`,
                    onChange: (page, pageSize) => fetchLogs(page, pageSize),
                }}
            />

            <Modal title="失败详情" open={Boolean(selected)} footer={null} width={760} onCancel={() => setSelected(null)}>
                {selected ? (
                    <div className="space-y-3">
                        <div className="grid gap-2 text-sm md:grid-cols-2">
                            <div>
                                <Text type="secondary">用户：</Text>
                                {selected.display_name || selected.username || `#${selected.user_id}`}
                            </div>
                            <div>
                                <Text type="secondary">模型：</Text>
                                {selected.model || "-"}
                            </div>
                            <div>
                                <Text type="secondary">类型：</Text>
                                {generationLabels[selected.generation] || selected.generation || "-"}
                            </div>
                            <div>
                                <Text type="secondary">状态：</Text>
                                {selected.status_code || "本地错误"}
                            </div>
                            <div className="md:col-span-2">
                                <Text type="secondary">接口：</Text>
                                <span className="font-mono">
                                    {selected.method} {selected.path}
                                </span>
                            </div>
                            <div className="md:col-span-2">
                                <Text type="secondary">错误：</Text>
                                {selected.error_message || "-"}
                            </div>
                        </div>
                        <pre className="max-h-[360px] overflow-auto rounded-lg bg-stone-950 p-3 text-xs text-stone-100">{formatErrorBody(selected.error_body)}</pre>
                    </div>
                ) : null}
            </Modal>
        </div>
    );
}

function formatErrorBody(value: string) {
    if (!value) return "无错误响应体";
    try {
        return JSON.stringify(JSON.parse(value), null, 2);
    } catch {
        return value;
    }
}
