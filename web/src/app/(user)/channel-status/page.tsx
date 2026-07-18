'use client';

import type { ReactNode } from 'react';
import { useEffect, useMemo, useState } from 'react';
import { Badge, Card, Select, Spin, Tooltip } from 'antd';
import { CheckCircleOutlined, ClockCircleOutlined, CloseCircleOutlined, ThunderboltOutlined, WarningOutlined } from '@ant-design/icons';
import axios from 'axios';

interface TimelinePoint {
  timestamp: string;
  status: string;
  uptime: number;
}

interface RecentError {
  timestamp: string;
  message: string;
  count: number;
}

interface ModelStatus {
  model: string;
  generation: string;
  display_name: string;
  status: string;
  uptime_1d: number;
  uptime_7d: number;
  uptime_15d: number;
  uptime_30d: number;
  avg_response_ms: number;
  timeline: TimelinePoint[];
  recent_errors?: RecentError[];
}

interface ChannelStatusData {
  models: ModelStatus[];
  updated_at: string;
}

const RANGE_OPTIONS = [
  { label: '24 小时', value: 1 },
  { label: '7 天', value: 7 },
  { label: '15 天', value: 15 },
  { label: '30 天', value: 30 },
];

const GENERATION_LABELS: Record<string, string> = {
  image: '图像生成',
  video: '视频生成',
  text: '文本生成',
  audio: '音频生成',
};

export default function ChannelStatusPage() {
  const [data, setData] = useState<ChannelStatusData | null>(null);
  const [loading, setLoading] = useState(true);
  const [days, setDays] = useState(1);

  const fetchData = async () => {
    try {
      setLoading(true);
      const response = await axios.get(`/backend-api/channel-status?days=${days}`);
      setData(response.data);
    } catch (error) {
      console.error('获取渠道状态失败:', error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
    const interval = setInterval(fetchData, 60000);
    return () => clearInterval(interval);
  }, [days]);

  const groupedModels = useMemo(
    () =>
      data?.models.reduce((acc, model) => {
        if (!acc[model.generation]) {
          acc[model.generation] = [];
        }
        acc[model.generation].push(model);
        return acc;
      }, {} as Record<string, ModelStatus[]>) || {},
    [data],
  );

  if (loading && !data) {
    return (
      <div className="flex h-full items-center justify-center overflow-y-auto p-24">
        <Spin size="large" />
      </div>
    );
  }

  return (
    <div className="h-full overflow-y-auto">
      <div className="mx-auto max-w-7xl px-4 py-4 md:px-6">
        <div className="sticky top-0 z-10 -mx-4 mb-4 border-b border-stone-200 bg-background/95 px-4 py-3 backdrop-blur md:-mx-6 md:px-6 dark:border-stone-800">
          <div className="flex flex-wrap items-center justify-end gap-3">
            <div className="rounded-full border border-stone-200 px-3 py-1 text-xs text-stone-500 dark:border-stone-700 dark:text-stone-400">
              最后更新：{data ? new Date(data.updated_at).toLocaleString() : '-'}
            </div>
            <Select value={days} onChange={setDays} style={{ width: 128 }} options={RANGE_OPTIONS} />
          </div>
        </div>

        <div className="mb-4 flex flex-wrap gap-3 text-[11px] text-stone-500 dark:text-stone-400">
          <div className="flex items-center gap-1.5"><span className="inline-block size-2 rounded-full bg-[#22c55e]" />正常</div>
          <div className="flex items-center gap-1.5"><span className="inline-block size-2 rounded-full bg-[#f59e0b]" />降级</div>
          <div className="flex items-center gap-1.5"><span className="inline-block size-2 rounded-full bg-[#ef4444]" />不可用</div>
        </div>

        {Object.entries(groupedModels).map(([generation, models]) => (
          <div key={generation} className="mb-6">
            <div className="mb-3">
              <h2 className="m-0 text-base font-semibold text-stone-900 dark:text-stone-100">
                {GENERATION_LABELS[generation] || generation}
              </h2>
            </div>

            <div className="grid grid-cols-1 gap-3 md:grid-cols-2 2xl:grid-cols-3">
              {models.map((model) => (
                <Card key={`${model.generation}-${model.model}`} size="small" className="rounded-2xl">
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        {getStatusIcon(model.status)}
                        <span className="truncate text-sm font-semibold text-stone-900 dark:text-stone-100">
                          {model.model || '未识别模型'}
                        </span>
                      </div>
                      <div className="mt-0.5 text-[11px] text-stone-500 dark:text-stone-400">{model.display_name}</div>
                    </div>
                    <Badge
                      status={model.status === 'operational' ? 'success' : model.status === 'degraded' ? 'warning' : 'error'}
                      text={getStatusText(model.status)}
                    />
                  </div>

                  <div className="mt-3 grid grid-cols-2 gap-2">
                    <MetricCard
                      icon={<ThunderboltOutlined />}
                      label="可用率"
                      value={`${getUptimeForRange(model, days).toFixed(2)}%`}
                      color={getStatusColor(model.status)}
                    />
                    <MetricCard
                      icon={<ClockCircleOutlined />}
                      label="平均响应"
                      value={model.avg_response_ms > 0 ? `${(model.avg_response_ms / 1000).toFixed(1)}s` : '-'}
                      color="#44403c"
                    />
                  </div>

                  <div className="mt-3">
                    <div className="mb-1 text-[11px] text-stone-500 dark:text-stone-400">状态时间线</div>
                    <TimelineBars timeline={model.timeline} days={days} />
                  </div>

                </Card>
              ))}
            </div>
          </div>
        ))}

        {(!data || data.models.length === 0) && (
          <div className="py-12 text-center text-sm text-stone-500 dark:text-stone-400">暂无渠道状态数据</div>
        )}
      </div>
    </div>
  );
}

function TimelineBars({ timeline, days }: { timeline: TimelinePoint[]; days: number }) {
  if (!timeline || timeline.length === 0) {
    return <span className="text-xs text-stone-400">-</span>;
  }
  const limit = days <= 1 ? 24 : 36;
  const blocks = timeline.slice(-limit);

  return (
    <div className="mt-2.5 flex gap-1">
      {blocks.map((point, idx) => {
        const date = new Date(point.timestamp);
        const tooltipContent = `${date.toLocaleString()}\n可用率: ${point.uptime.toFixed(2)}%`;
        return (
          <Tooltip key={idx} title={tooltipContent}>
            <div
              className="min-w-0 flex-1 rounded-sm"
              style={{ height: 12, backgroundColor: getStatusColor(point.status), opacity: 0.9 }}
            />
          </Tooltip>
        );
      })}
    </div>
  );
}

function MetricCard({
  icon,
  label,
  value,
  color,
}: {
  icon: ReactNode;
  label: string;
  value: string;
  color: string;
}) {
  return (
    <div className="rounded-2xl bg-stone-50 px-3 py-2.5 dark:bg-stone-900">
      <div className="flex items-center gap-2 text-[11px] text-stone-500 dark:text-stone-400">
        <span>{icon}</span>
        <span>{label}</span>
      </div>
      <div className="mt-1.5 text-base font-semibold" style={{ color }}>{value}</div>
    </div>
  );
}

function getStatusIcon(status: string) {
  switch (status) {
    case 'operational':
      return <CheckCircleOutlined style={{ color: '#22c55e', fontSize: 18 }} />;
    case 'degraded':
      return <WarningOutlined style={{ color: '#f59e0b', fontSize: 18 }} />;
    case 'down':
      return <CloseCircleOutlined style={{ color: '#ef4444', fontSize: 18 }} />;
    default:
      return null;
  }
}

function getStatusText(status: string) {
  switch (status) {
    case 'operational':
      return '正常运行';
    case 'degraded':
      return '性能下降';
    case 'down':
      return '不可用';
    default:
      return status;
  }
}

function getStatusColor(status: string) {
  switch (status) {
    case 'operational':
      return '#22c55e';
    case 'degraded':
      return '#f59e0b';
    case 'down':
      return '#ef4444';
    default:
      return '#d6d3d1';
  }
}

function getUptimeForRange(model: ModelStatus, days: number) {
  if (days >= 30) return model.uptime_30d;
  if (days >= 15) return model.uptime_15d;
  if (days >= 7) return model.uptime_7d;
  return model.uptime_1d;
}
