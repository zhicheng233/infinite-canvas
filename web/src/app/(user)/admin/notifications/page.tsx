"use client";

import { useCallback, useEffect, useState } from "react";
import { App, Button, Input, Modal, Space, Switch, Table, Tag, Typography } from "antd";
import { Play, RefreshCw, Save } from "lucide-react";

import { listWebhookConfigs, listWebhookLogs, saveWebhookConfig, testWebhookSend, type TestSendResult, type WebhookConfig, type WebhookLogItem } from "@/services/api/webhook";

const platforms = ["feishu", "dtalk", "wecom", "telegram"] as const;
const platformLabels: Record<string, string> = { feishu: "飞书", dtalk: "钉钉", wecom: "企业微信", telegram: "Telegram" };

export default function AdminNotificationsPage() {
    const { message } = App.useApp();
    const [configs, setConfigs] = useState<Record<string, WebhookConfig>>({});
    const [logs, setLogs] = useState<WebhookLogItem[]>([]);
    const [loading, setLoading] = useState(true);
    const [busy, setBusy] = useState("");
    const [testPlatform, setTestPlatform] = useState("");
    const [testMessage, setTestMessage] = useState("");
    const [testResult, setTestResult] = useState<TestSendResult>();

    const load = useCallback(async () => {
        setLoading(true);
        try {
            const [nextConfigs, nextLogs] = await Promise.all([listWebhookConfigs(), listWebhookLogs(50)]);
            const byPlatform: Record<string, WebhookConfig> = {};
            for (const platform of platforms) byPlatform[platform] = nextConfigs.find((item) => item.platform === platform) ?? { platform, webhook_url: "", enabled: false, cooldown_minutes: 30 };
            setConfigs(byPlatform);
            setLogs(nextLogs);
        } catch (error) {
            message.error(errorMessage(error, "读取消息通知配置失败"));
        } finally {
            setLoading(false);
        }
    }, [message]);

    useEffect(() => {
        void load();
    }, [load]);

    const update = (platform: string, patch: Partial<WebhookConfig>) => setConfigs((current) => ({ ...current, [platform]: { ...current[platform], ...patch } }));

    const save = async (platform: string) => {
        const config = configs[platform];
        if (!config?.webhook_url.trim()) {
            message.warning("请输入 Webhook URL");
            return;
        }
        setBusy(`save:${platform}`);
        try {
            const saved = await saveWebhookConfig({ platform, webhook_url: config.webhook_url.trim(), enabled: config.enabled, cooldown_minutes: config.cooldown_minutes });
            update(platform, saved);
            message.success(`${platformLabels[platform]}通知配置已保存`);
        } catch (error) {
            message.error(errorMessage(error, "保存通知配置失败"));
        } finally {
            setBusy("");
        }
    };

    const runTest = async () => {
        if (!testMessage.trim()) {
            message.warning("请输入测试消息");
            return;
        }
        setBusy("test");
        setTestResult(undefined);
        try {
            const result = await testWebhookSend({ platform: testPlatform, message: testMessage.trim() });
            setTestResult(result);
            if (result.success) {
                message.success("测试消息已发送");
                setLogs(await listWebhookLogs(50));
            }
        } catch (error) {
            message.error(errorMessage(error, "发送测试消息失败"));
        } finally {
            setBusy("");
        }
    };

    return (
        <div className="mx-auto max-w-[1400px] space-y-6">
            <div className="flex flex-wrap items-end justify-between gap-3">
                <div>
                    <Typography.Title level={4} className="!mb-1">消息通知</Typography.Title>
                    <Typography.Text type="secondary">配置模型调用异常通知及发送冷却时间。</Typography.Text>
                </div>
                <Button icon={<RefreshCw className="size-4" />} loading={loading} onClick={() => void load()}>刷新</Button>
            </div>

            <section>
                <Typography.Title level={5}>通知渠道</Typography.Title>
                <Table
                    rowKey="platform"
                    loading={loading}
                    pagination={false}
                    dataSource={platforms.map((platform) => configs[platform] ?? { platform, webhook_url: "", enabled: false, cooldown_minutes: 30 })}
                    columns={[
                        { title: "平台", width: 110, dataIndex: "platform", render: (value) => <span className="font-medium">{platformLabels[value] ?? value}</span> },
                        { title: "Webhook URL", render: (_, item) => <Input.Password visibilityToggle={false} value={item.webhook_url} placeholder="https://..." onChange={(event) => update(item.platform, { webhook_url: event.target.value })} /> },
                        { title: "冷却（分钟）", width: 140, render: (_, item) => <Input type="number" min={0} value={item.cooldown_minutes ?? 30} onChange={(event) => update(item.platform, { cooldown_minutes: Math.max(0, Number(event.target.value) || 0) })} /> },
                        { title: "启用", width: 80, render: (_, item) => <Switch checked={item.enabled} onChange={(enabled) => update(item.platform, { enabled })} /> },
                        {
                            title: "操作",
                            width: 150,
                            render: (_, item) => <Space size={2}><Button type="text" icon={<Save className="size-4" />} loading={busy === `save:${item.platform}`} title="保存" onClick={() => void save(item.platform)} /><Button type="text" icon={<Play className="size-4" />} title="发送测试" onClick={() => { setTestPlatform(item.platform); setTestMessage(""); setTestResult(undefined); }} /></Space>,
                        },
                    ]}
                />
            </section>

            <section>
                <Typography.Title level={5}>发送记录</Typography.Title>
                <Table
                    rowKey="id"
                    loading={loading}
                    dataSource={logs}
                    pagination={{ pageSize: 15, hideOnSinglePage: true }}
                    columns={[
                        { title: "时间", dataIndex: "created_at", width: 190, render: (value) => new Date(value).toLocaleString("zh-CN") },
                        { title: "平台", dataIndex: "platform", width: 100, render: (value) => platformLabels[value] ?? value },
                        { title: "模型", dataIndex: "model_name", width: 180, ellipsis: true },
                        { title: "状态", width: 100, render: (_, item) => item.cooldown_skipped ? <Tag>冷却跳过</Tag> : item.success ? <Tag color="green">成功</Tag> : <Tag color="red">失败</Tag> },
                        { title: "消息", dataIndex: "message", ellipsis: true },
                    ]}
                />
            </section>

            <Modal open={Boolean(testPlatform)} title={`测试推送：${platformLabels[testPlatform] ?? testPlatform}`} okText="发送测试" confirmLoading={busy === "test"} onCancel={() => setTestPlatform("")} onOk={() => void runTest()}>
                <Input.TextArea rows={4} value={testMessage} placeholder="输入测试消息" onChange={(event) => setTestMessage(event.target.value)} />
                {testResult ? <div className={`mt-3 border-l-2 pl-3 text-sm ${testResult.success ? "border-emerald-500" : "border-red-500"}`}>{testResult.success ? "发送成功" : testResult.error || "发送失败"}</div> : null}
            </Modal>
        </div>
    );
}

function errorMessage(error: unknown, fallback: string) {
    return error instanceof Error && error.message ? error.message : fallback;
}
