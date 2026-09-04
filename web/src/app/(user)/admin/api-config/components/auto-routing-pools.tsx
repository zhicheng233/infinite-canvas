"use client";

import { useCallback, useEffect, useState } from "react";
import { Alert, App, Button, Empty, InputNumber, Popconfirm, Switch, Table, Tag } from "antd";
import { RefreshCw, Route, Trash2 } from "lucide-react";

import { createAutoRoutingPool, deleteAutoRoutingPool, listAutoRoutingPools, listAutoRoutingSuggestions, updateAutoRoutingMember, updateAutoRoutingPool, type AutoRoutingPool, type AutoRoutingSuggestion } from "@/services/api/auto-routing-admin";
import { ChannelNameWithRemark } from "@/components/channel-name-with-remark";

const capabilityLabel: Record<string, string> = { image: "图片", video: "视频", text: "文本", audio: "音频" };

export function AutoRoutingPools() {
    const { message } = App.useApp();
    const [pools, setPools] = useState<AutoRoutingPool[]>([]);
    const [suggestions, setSuggestions] = useState<AutoRoutingSuggestion[]>([]);
    const [loading, setLoading] = useState(false);
    const [saving, setSaving] = useState<string>("");

    const load = useCallback(async () => {
        setLoading(true);
        try {
            const [nextPools, nextSuggestions] = await Promise.all([listAutoRoutingPools(), listAutoRoutingSuggestions()]);
            setPools(nextPools);
            setSuggestions(nextSuggestions.filter((item) => {
                const pool = nextPools.find((candidate) => candidate.model === item.model && candidate.capability === item.capability);
                if (!pool) return true;
                const current = new Set(pool.members.map((member) => member.channel_model_id));
                return pool.contract_key !== item.contract_key || item.members.length !== current.size || item.members.some((member) => !current.has(member.channel_model_id));
            }));
        } catch (error) {
            message.error(error instanceof Error ? error.message : "读取智能路由配置失败");
        } finally {
            setLoading(false);
        }
    }, [message]);

    useEffect(() => { void load(); }, [load]);

    const run = async (key: string, action: () => Promise<unknown>) => {
        setSaving(key);
        try { await action(); await load(); }
        catch (error) { message.error(error instanceof Error ? error.message : "保存智能路由配置失败"); }
        finally { setSaving(""); }
    };

    return (
        <div className="space-y-4">
            <Alert type="info" showIcon message="智能路由只在管理员确认后生效" description="系统仅建议模型名、能力和协议合同完全一致的候选。新建路由池默认关闭，确认候选状态后再启用。" />
            <div className="flex items-center justify-between gap-3">
                <div className="text-sm text-muted-foreground">每次生成最多尝试两个候选；参数或内容错误不会切换渠道。</div>
                <Button icon={<RefreshCw className="size-4" />} loading={loading} onClick={() => void load()}>刷新</Button>
            </div>
            {suggestions.length ? (
                <Table
                    rowKey={(item) => `${item.model}:${item.capability}:${item.contract_key}`}
                    size="small"
                    pagination={false}
                    dataSource={suggestions}
                    columns={[
                        { title: "系统建议", dataIndex: "model" },
                        { title: "能力", dataIndex: "capability", width: 90, render: (value: string) => capabilityLabel[value] || value },
                        { title: "候选", width: 260, render: (_, item) => item.members.map((member) => <Tag key={member.channel_model_id}><ChannelNameWithRemark name={member.channel_name} remark={member.channel_remark} /></Tag>) },
                        { title: "操作", width: 120, render: (_, item) => {
                            const pool = pools.find((candidate) => candidate.model === item.model && candidate.capability === item.capability);
                            const key = `members:${item.model}:${item.capability}`;
                            const memberIds = item.members.map((member) => member.channel_model_id);
                            const reconfirm = pool && pool.contract_key !== item.contract_key;
                            return <Button type="primary" size="small" icon={<Route className="size-4" />} loading={saving === key} onClick={() => void run(key, () => pool ? updateAutoRoutingPool(pool.id, { contract_key: item.contract_key, channel_model_ids: memberIds }) : createAutoRoutingPool({ model: item.model, capability: item.capability, contract_key: item.contract_key, channel_model_ids: memberIds }))}>{reconfirm ? "重新确认" : pool ? "同步候选" : "确认创建"}</Button>;
                        } },
                    ]}
                />
            ) : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无新的兼容候选建议" />}
            <Table
                rowKey="id"
                loading={loading}
                pagination={false}
                dataSource={pools}
                expandable={{ expandedRowRender: (pool) => (
                    <Table rowKey="id" size="small" pagination={false} dataSource={pool.members} columns={[
                        { title: "渠道", dataIndex: "channel_name", render: (_, member) => <ChannelNameWithRemark name={member.channel_name} remark={member.channel_remark} /> },
                        { title: "合同", width: 100, render: (_, member) => member.contract_valid ? <Tag color="green">有效</Tag> : <Tag color="red">需确认</Tag> },
                        { title: "可靠性", width: 110, render: (_, member) => `${Math.round(member.success_rate)}% / ${member.sample_count} 次` },
                        { title: "P95", width: 90, render: (_, member) => member.p95_latency_ms ? `${member.p95_latency_ms}ms` : "暂无" },
                        { title: "优先级", width: 110, render: (_, member) => <InputNumber size="small" value={member.priority} disabled={!member.contract_valid} onChange={(value) => void run(`member:${member.id}`, () => updateAutoRoutingMember(pool.id, member.id, { priority: Number(value || 0) }))} /> },
                        { title: "启用", width: 80, render: (_, member) => <Switch size="small" checked={member.enabled} disabled={!member.contract_valid} loading={saving === `member:${member.id}`} onChange={(enabled) => void run(`member:${member.id}`, () => updateAutoRoutingMember(pool.id, member.id, { enabled }))} /> },
                    ]} />
                ) }}
                columns={[
                    { title: "模型", dataIndex: "model" },
                    { title: "能力", dataIndex: "capability", width: 90, render: (value: string) => capabilityLabel[value] || value },
                    { title: "候选", width: 100, render: (_, pool) => `${pool.members.filter((member) => member.enabled && member.contract_valid).length}/${pool.members.length}` },
                    { title: "最大尝试", dataIndex: "max_attempts", width: 100 },
                    { title: "启用", width: 90, render: (_, pool) => <Switch checked={pool.enabled} loading={saving === `pool:${pool.id}`} onChange={(enabled) => void run(`pool:${pool.id}`, () => updateAutoRoutingPool(pool.id, { enabled }))} /> },
                    { title: "操作", width: 90, render: (_, pool) => <Popconfirm title="删除这个智能路由池？" onConfirm={() => void run(`delete:${pool.id}`, () => deleteAutoRoutingPool(pool.id))}><Button danger type="text" icon={<Trash2 className="size-4" />} /></Popconfirm> },
                ]}
            />
        </div>
    );
}
