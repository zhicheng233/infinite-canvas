"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";
import { Alert, Button, Card, Empty, InputNumber, Select, Spin, Tag } from "antd";
import { Activity, CheckCircle2, ChevronDown, ChevronRight, CircleAlert, CircleX, Clock3, RefreshCw } from "lucide-react";

import { getMetrics, type ChannelMetrics, type ModelMetrics, type MetricsResponse } from "@/services/api/metrics";

const CUSTOM_HOURS_VALUE = -1;
const HOURS_OPTIONS = [
    { label: "1 小时", value: 1 },
    { label: "6 小时", value: 6 },
    { label: "12 小时", value: 12 },
    { label: "24 小时", value: 24 },
    { label: "72 小时", value: 72 },
    { label: "7 天", value: 168 },
    { label: "14 天", value: 336 },
    { label: "30 天", value: 720 },
    { label: "自定义", value: CUSTOM_HOURS_VALUE },
];

const STATUS_LABELS: Record<string, string> = {
    ok: "正常",
    unavailable: "暂无数据",
    unmapped: "未映射",
    stale: "数据已过期",
    error: "指标错误",
};

const STATUS_COLORS: Record<string, string> = {
    ok: "success",
    unavailable: "default",
    unmapped: "warning",
    stale: "warning",
    error: "error",
};

export default function ChannelStatusPage() {
    const [hours, setHours] = useState(24);
    const [customHours, setCustomHours] = useState(24);
    const [data, setData] = useState<MetricsResponse | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    const effectiveHours = hours === CUSTOM_HOURS_VALUE ? customHours : hours;

    const loadMetrics = useCallback(async () => {
        setLoading(true);
        setError(null);
        try {
            setData(await getMetrics(effectiveHours));
        } catch (reason) {
            setError(reason instanceof Error ? reason.message : "获取渠道指标失败");
        } finally {
            setLoading(false);
        }
    }, [effectiveHours]);

    useEffect(() => {
        void loadMetrics();
    }, [loadMetrics]);

    const channels = useMemo(() => (data?.channels || []).map(sortChannelModels), [data]);

    const handleHoursChange = useCallback((value: number) => {
        setHours(value);
        if (value !== CUSTOM_HOURS_VALUE) {
            void getMetrics(value)
                .then(setData)
                .catch(() => {});
        }
    }, []);

    const handleCustomHoursApply = useCallback(() => {
        const clamped = Math.max(1, Math.min(720, Math.floor(customHours) || 24));
        setCustomHours(clamped);
        setHours(CUSTOM_HOURS_VALUE);
        void getMetrics(clamped)
            .then(setData)
            .catch(() => {});
    }, [customHours]);

    if (loading && !data) {
        return (
            <main className="flex h-full items-center justify-center">
                <Spin size="large" />
            </main>
        );
    }

    return (
        <main className="mx-auto h-full max-w-7xl overflow-y-auto px-4 py-6 md:px-6 md:py-8">
            <div className="mb-7 flex flex-wrap items-start justify-between gap-4">
                <div>
                    <div className="mb-2 flex items-center gap-2 text-xs font-medium uppercase tracking-[0.16em] text-stone-500 dark:text-stone-400">
                        <Activity className="size-4" />
                        渠道观测
                    </div>
                    <h1 className="m-0 text-2xl font-semibold text-stone-950 dark:text-stone-100">渠道指标</h1>
                    <p className="mt-2 text-sm text-stone-500 dark:text-stone-400">查看已启用渠道及其模型在所选时间窗口内的成功率。</p>
                </div>
                <div className="flex flex-wrap items-center gap-2">
                    <div className="flex items-center gap-2">
                        <Select data-testid="metrics-hours" value={hours} onChange={handleHoursChange} options={HOURS_OPTIONS} className="w-32" />
                        {hours === CUSTOM_HOURS_VALUE && (
                            <div className="flex items-center gap-1">
                                <InputNumber data-testid="custom-hours-input" min={1} max={720} value={customHours} onChange={(value) => setCustomHours(value ?? 24)} className="w-20" />
                                <span className="text-xs text-stone-500">小时</span>
                                <Button data-testid="apply-custom-hours" size="small" onClick={handleCustomHoursApply}>
                                    应用
                                </Button>
                            </div>
                        )}
                    </div>
                    <Button data-testid="refresh-metrics" icon={<RefreshCw className="size-4" />} loading={loading} onClick={() => void loadMetrics()}>
                        刷新指标
                    </Button>
                </div>
            </div>

            {error && <Alert className="mb-5" type="error" showIcon message="指标请求失败" description={error} />}
            {data?.error && <Alert className="mb-5" type="warning" showIcon message="指标服务状态异常" description={data.error} />}

            {data && (
                <Card size="small" className="mb-6 rounded-2xl">
                    <div className="grid gap-4 text-sm md:grid-cols-5">
                        <MetadataItem label="时间范围" value={`${data.hours} 小时（${data.window}）`} />
                        <MetadataItem label="服务状态" value={<StatusTag status={data.status} />} />
                        <MetadataItem label="更新时间" value={formatDate(data.updated_at)} />
                        <MetadataItem label="渠道数量" value={`${data.channels.length}`} />
                        <MetadataItem label="刷新状态" value={loading ? "正在刷新" : "已完成"} />
                    </div>
                </Card>
            )}

            {!loading && data && channels.length === 0 && <Empty className="py-16" description="暂无已启用渠道指标" />}
            {channels.length > 0 && (
                <div className="space-y-5">
                    {channels.map((channel) => (
                        <ChannelCard key={channel.channel_id} channel={channel} />
                    ))}
                </div>
            )}
        </main>
    );
}

function ChannelCard({ channel }: { channel: ChannelMetrics }) {
    const [expanded, setExpanded] = useState(false);
    const toggleExpanded = useCallback(() => setExpanded((prev) => !prev), []);

    return (
        <Card className="rounded-2xl" bodyStyle={{ padding: 0 }}>
            <button type="button" onClick={toggleExpanded} className="flex w-full cursor-pointer items-center justify-between border-0 bg-transparent px-4 py-4 text-left md:px-5 hover:bg-stone-50 dark:hover:bg-stone-900/50 transition-colors">
                <div className="flex min-w-0 flex-1 items-start gap-3">
                    <StatusIcon status={channel.status} />
                    <div className="min-w-0 flex-1">
                        <div className="flex flex-wrap items-center gap-2">
                            <h2 className="m-0 truncate text-base font-semibold text-stone-950 dark:text-stone-100">{channel.channel_name}</h2>
                            <StatusTag status={channel.status} />
                        </div>
                        <div className="mt-1 flex flex-wrap gap-x-4 gap-y-1 text-xs text-stone-500 dark:text-stone-400">
                            <span>应用渠道 ID：{channel.channel_id}</span>
                            <span>New-API 映射：{channel.new_api_channel_id ?? "未设置"}</span>
                        </div>
                    </div>
                </div>
                <div className="flex shrink-0 items-center gap-3">
                    <div className="text-right">
                        <RateValue rate={channel.success_rate} status={channel.status} prominent />
                        <CountSummary requestCount={channel.request_count} successCount={channel.success_count} />
                    </div>
                    {expanded ? <ChevronDown className="size-4 text-stone-400" /> : <ChevronRight className="size-4 text-stone-400" />}
                </div>
            </button>
            {expanded && (
                <div className="border-t border-stone-200 px-4 py-4 dark:border-stone-800 md:px-5">
                    <div className="mb-3 flex items-center justify-between gap-3">
                        <h3 className="m-0 text-sm font-semibold text-stone-800 dark:text-stone-200">支持的已启用模型</h3>
                        <span className="text-xs text-stone-500 dark:text-stone-400">{channel.models.length} 个模型</span>
                    </div>
                    {channel.models.length === 0 ? (
                        <div className="rounded-xl bg-stone-50 px-4 py-5 text-center text-sm text-stone-500 dark:bg-stone-900 dark:text-stone-400">暂无已启用模型</div>
                    ) : (
                        <div className="grid gap-2 md:grid-cols-2">
                            {channel.models.map((model) => (
                                <ModelRow key={model.channel_model_id} model={model} />
                            ))}
                        </div>
                    )}
                </div>
            )}
        </Card>
    );
}

function ModelRow({ model }: { model: ModelMetrics }) {
    return (
        <div className="flex items-center justify-between gap-3 rounded-xl border border-stone-200 px-3 py-3 dark:border-stone-800">
            <div className="min-w-0">
                <div className="truncate text-sm font-medium text-stone-900 dark:text-stone-100">{model.model_name}</div>
                <div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-stone-500 dark:text-stone-400">
                    <span>模型 ID：{model.channel_model_id}</span>
                    <span>
                        请求 {model.request_count} · 成功 {model.success_count}
                    </span>
                </div>
            </div>
            <RateValue rate={model.success_rate} status={model.status} />
        </div>
    );
}

function MetadataItem({ label, value }: { label: string; value: ReactNode }) {
    return (
        <div>
            <div className="mb-1 text-xs text-stone-500 dark:text-stone-400">{label}</div>
            <div className="font-medium text-stone-800 dark:text-stone-200">{value}</div>
        </div>
    );
}

function CountSummary({ requestCount, successCount }: { requestCount: number; successCount: number }) {
    return (
        <div className="mt-4 flex flex-wrap gap-x-5 gap-y-1 text-xs text-stone-500 dark:text-stone-400">
            <span className="inline-flex items-center gap-1.5">
                <Clock3 className="size-3.5" />
                请求 {requestCount}
            </span>
            <span className="inline-flex items-center gap-1.5">
                <CheckCircle2 className="size-3.5" />
                成功 {successCount}
            </span>
        </div>
    );
}

function RateValue({ rate, status, prominent = false }: { rate: number | null; status: string; prominent?: boolean }) {
    return (
        <span className={`${prominent ? "text-lg" : "text-sm"} whitespace-nowrap font-semibold ${rate === null ? "text-stone-400 dark:text-stone-500" : "text-stone-950 dark:text-stone-100"}`}>
            {rate === null ? unavailableRateText(status) : `${rate}%`}
        </span>
    );
}

function StatusTag({ status }: { status: string }) {
    return <Tag color={STATUS_COLORS[status] || "default"}>{STATUS_LABELS[status] || status}</Tag>;
}

function StatusIcon({ status }: { status: string }) {
    if (status === "ok") return <CheckCircle2 className="mt-0.5 size-5 text-emerald-500" />;
    if (status === "error") return <CircleX className="mt-0.5 size-5 text-rose-500" />;
    return <CircleAlert className="mt-0.5 size-5 text-amber-500" />;
}

function unavailableRateText(status: string) {
    if (status === "stale") return "已过期";
    if (status === "unmapped") return "未映射";
    if (status === "error") return "获取失败";
    return "暂无数据";
}

function sortChannelModels(channel: ChannelMetrics): ChannelMetrics {
    return {
        ...channel,
        models: sortByRate(
            channel.models,
            (model) => model.success_rate,
            (model) => model.model_name,
        ),
    };
}

function sortByRate<T>(items: T[], getRate: (item: T) => number | null, getName: (item: T) => string) {
    return items
        .map((item, index) => ({ item, index }))
        .sort((left, right) => {
            const leftRate = getRate(left.item);
            const rightRate = getRate(right.item);
            if (leftRate === null && rightRate !== null) return 1;
            if (leftRate !== null && rightRate === null) return -1;
            if (leftRate !== null && rightRate !== null && leftRate !== rightRate) return rightRate - leftRate;
            const nameOrder = getName(left.item).localeCompare(getName(right.item));
            return nameOrder || left.index - right.index;
        })
        .map(({ item }) => item);
}

function formatDate(value: string) {
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? value : date.toLocaleString("zh-CN");
}
