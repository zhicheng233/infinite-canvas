"use client";

import { useCallback, useEffect, useState } from "react";
import { App, Button, Popconfirm, Table, Tag, Typography } from "antd";
import { RefreshCw, Trash2 } from "lucide-react";

import { summarizeCustomVideoConfig } from "@/lib/custom-video-config";
import { deleteVideoConfigPreset, listVideoConfigPresets, type VideoConfigPreset } from "@/services/api/video-config-presets";

export function ModelServicePresets({ refreshToken, onChanged }: { readonly refreshToken: number; readonly onChanged: () => void }) {
    const { message } = App.useApp();
    const [items, setItems] = useState<VideoConfigPreset[]>([]);
    const [loading, setLoading] = useState(true);

    const load = useCallback(async () => {
        setLoading(true);
        try {
            setItems(await listVideoConfigPresets());
        } catch (error) {
            message.error(error instanceof Error ? error.message : "读取视频预设失败");
        } finally {
            setLoading(false);
        }
    }, [message]);

    useEffect(() => {
        void load();
    }, [load, refreshToken]);

    const remove = async (item: VideoConfigPreset) => {
        try {
            await deleteVideoConfigPreset(item.id);
            message.success("视频配置预设已删除");
            await load();
            onChanged();
        } catch (error) {
            message.error(error instanceof Error ? error.message : "删除视频预设失败");
        }
    };

    return (
        <div className="space-y-4">
            <div className="flex flex-wrap items-center justify-between gap-3">
                <div>
                    <Typography.Title level={5} className="!mb-0">视频配置预设</Typography.Title>
                    <Typography.Text type="secondary">预设供模型详情中的自定义视频协议复用，保存模型时才会写入该模型合同。</Typography.Text>
                </div>
                <Button icon={<RefreshCw className="size-4" />} loading={loading} onClick={() => void load()}>刷新</Button>
            </div>
            <Table
                rowKey="id"
                loading={loading}
                dataSource={items}
                pagination={{ pageSize: 20, hideOnSinglePage: true }}
                columns={[
                    { title: "预设名称", dataIndex: "name", render: (value) => <span className="font-medium">{value}</span> },
                    { title: "尺寸", width: 200, render: (_, item) => dimensionText(item) },
                    { title: "时长", width: 180, render: (_, item) => secondsText(item) },
                    { title: "素材输入", render: (_, item) => mediaTags(item) },
                    { title: "操作", width: 80, render: (_, item) => <Popconfirm title={`删除预设“${item.name}”？`} description="已保存的模型配置不会受影响。" onConfirm={() => void remove(item)}><Button type="text" danger title="删除预设" icon={<Trash2 className="size-4" />} /></Popconfirm> },
                ]}
            />
        </div>
    );
}

function dimensionText(item: VideoConfigPreset) {
    const value = summarizeCustomVideoConfig(item.config).dimensions;
    if (!value) return "未启用";
    return `${value.mode === "size" ? "尺寸" : "宽高比"}：${value.options.join("、")}`;
}

function secondsText(item: VideoConfigPreset) {
    const value = summarizeCustomVideoConfig(item.config).seconds;
    if (!value) return "未启用";
    return value.mode === "range" ? `${value.min}-${value.max} 秒` : `${value.options.join("、")} 秒`;
}

function mediaTags(item: VideoConfigPreset) {
    const summary = summarizeCustomVideoConfig(item.config);
    const labels: Record<string, string> = { images: "图片参考", input_reference: "首帧", style_references: "风格图", element_references: "元素图", reference_images: "兼容参考", input_video: "源视频" };
    const enabled = Object.keys(summary.media_limits).filter((key) => summary.media_limits[key as keyof typeof summary.media_limits] !== undefined);
    return enabled.length ? enabled.map((key) => <Tag key={key}>{labels[key] ?? key}</Tag>) : <Typography.Text type="secondary">无</Typography.Text>;
}
