"use client";

import { useEffect, useState } from "react";
import { App, Button, Form, Input, Modal, Popover, Select, Tooltip } from "antd";
import { Save, Trash2 } from "lucide-react";
import type { FormInstance } from "antd";

import { customVideoFeatureNames, customVideoMediaFeatureNames, normalizeAndValidateCustomVideoConfig, summarizeCustomVideoConfig, type CustomVideoConfig, type CustomVideoFeature } from "@/lib/custom-video-config";
import { createVideoConfigPreset, deleteVideoConfigPreset, listVideoConfigPresets, type VideoConfigPreset } from "@/services/api/video-config-presets";

type CustomVideoConfigPresetsProps = {
    readonly form: FormInstance;
    readonly disabled: boolean;
};

type PresetNameValues = {
    readonly name: string;
};

const featureLabels: Record<CustomVideoFeature, string> = {
    seconds: "时长",
    dimensions: "尺寸/宽高比",
    images: "普通参考图",
    input_reference: "首帧参考图",
    style_references: "风格参考图",
    element_references: "元素参考图",
    reference_images: "兼容参考图",
    reference_mode: "兼容参考图模式",
    input_video: "源视频",
    audio: "音频",
    n: "生成数量",
};

export function CustomVideoConfigPresets({ form, disabled }: CustomVideoConfigPresetsProps) {
    const { message } = App.useApp();
    const config = Form.useWatch("video_custom_config", form);
    const [presets, setPresets] = useState<VideoConfigPreset[]>([]);
    const [loading, setLoading] = useState(true);
    const [selectedPresetId, setSelectedPresetId] = useState<number>();
    const [saveModalOpen, setSaveModalOpen] = useState(false);
    const [saving, setSaving] = useState(false);
    const [nameForm] = Form.useForm<PresetNameValues>();

    useEffect(() => {
        void loadPresets().catch(() => undefined);
    }, []);

    useEffect(() => {
        const selected = presets.find((preset) => preset.id === selectedPresetId);
        if (selected && !sameConfig(config, selected.config)) setSelectedPresetId(undefined);
    }, [config, presets, selectedPresetId]);

    const loadPresets = async (showError = true) => {
        setLoading(true);
        try {
            setPresets(await listVideoConfigPresets());
        } catch (error) {
            if (showError) message.error(errorMessage(error, "读取预设列表失败"));
            throw error;
        } finally {
            setLoading(false);
        }
    };

    const applyPreset = (preset: VideoConfigPreset) => {
        form.setFieldValue("video_custom_config", cloneConfig(preset.config));
        setSelectedPresetId(preset.id);
    };

    const handlePresetChange = (presetId: number | undefined) => {
        if (presetId === undefined) {
            setSelectedPresetId(undefined);
            return;
        }
        const preset = presets.find((item) => item.id === presetId);
        if (!preset) return;
        if (shouldConfirmOverwrite(form.getFieldValue("video_custom_config"), preset.config)) {
            Modal.confirm({
                title: "覆盖当前设置？",
                content: "应用预设将覆盖当前未保存设置",
                okText: "应用预设",
                cancelText: "取消",
                onOk: () => applyPreset(preset),
            });
            return;
        }
        applyPreset(preset);
    };

    const openSaveModal = () => {
        const result = normalizeAndValidateCustomVideoConfig(config);
        if (!result.ok) {
            message.error("请先修正自定义视频配置后再保存预设");
            return;
        }
        nameForm.resetFields();
        setSaveModalOpen(true);
    };

    const savePreset = async ({ name }: PresetNameValues) => {
        const result = normalizeAndValidateCustomVideoConfig(form.getFieldValue("video_custom_config"));
        if (!result.ok) {
            message.error("请先修正自定义视频配置后再保存预设");
            return;
        }
        setSaving(true);
        try {
            const preset = await createVideoConfigPreset({ name: name.trim(), config: cloneConfig(result.config) });
            setPresets((current) => [...current, preset].toSorted((left, right) => left.name.localeCompare(right.name, "zh-CN") || left.id - right.id));
            setSelectedPresetId(preset.id);
            setSaveModalOpen(false);
            message.success("预设已保存");
        } catch (error) {
            message.error(errorMessage(error, "保存预设失败"));
        } finally {
            setSaving(false);
        }
    };

    const confirmDelete = (preset: VideoConfigPreset) => {
        Modal.confirm({
            title: "删除预设？",
            content: `确定删除预设“${preset.name}”？此操作不会影响当前模型配置。`,
            okText: "删除",
            cancelText: "取消",
            okButtonProps: { danger: true },
            onOk: async () => {
                try {
                    await deleteVideoConfigPreset(preset.id);
                    setPresets((current) => current.filter((item) => item.id !== preset.id));
                    if (selectedPresetId === preset.id) setSelectedPresetId(undefined);
                    message.success("预设已删除");
                } catch (error) {
                    message.error(errorMessage(error, "删除预设失败"));
                    throw error;
                }
            },
        });
    };

    return (
        <>
            <div className="flex flex-wrap items-end gap-2 border-b border-stone-200 pb-4 dark:border-stone-800">
                <div className="min-w-52 flex-1">
                    <div className="mb-1 text-sm font-medium">预设</div>
                    <Select allowClear className="w-full" disabled={disabled} loading={loading} placeholder="选择租户预设" value={selectedPresetId} onChange={handlePresetChange}>
                        {presets.map((preset) => (
                            <Select.Option key={preset.id} value={preset.id}>
                                <div className="flex min-w-0 items-center gap-1">
                                    <Popover content={<PresetSummary config={preset.config} />} placement="rightTop" trigger="hover">
                                        <span className="min-w-0 flex-1 truncate">{preset.name}</span>
                                    </Popover>
                                    <Tooltip title="删除预设">
                                        <Button
                                            aria-label={`删除预设 ${preset.name}`}
                                            danger
                                            disabled={disabled}
                                            icon={<Trash2 className="size-3.5" />}
                                            size="small"
                                            type="text"
                                            onClick={(event) => {
                                                event.stopPropagation();
                                                confirmDelete(preset);
                                            }}
                                            onMouseDown={(event) => {
                                                event.preventDefault();
                                                event.stopPropagation();
                                            }}
                                        />
                                    </Tooltip>
                                </div>
                            </Select.Option>
                        ))}
                    </Select>
                </div>
                <Button disabled={disabled} icon={<Save className="size-4" />} onClick={openSaveModal}>
                    保存为预设
                </Button>
            </div>

            <Modal confirmLoading={saving} destroyOnHidden okText="保存" open={saveModalOpen} title="保存为预设" onCancel={() => setSaveModalOpen(false)} onOk={() => nameForm.submit()}>
                <Form form={nameForm} layout="vertical" preserve={false} onFinish={savePreset}>
                    <Form.Item name="name" label="预设名称" rules={[{ required: true, whitespace: true, message: "请输入预设名称" }]}>
                        <Input autoFocus maxLength={200} placeholder="例如：Omni 原生" />
                    </Form.Item>
                </Form>
            </Modal>
        </>
    );
}

function PresetSummary({ config }: { readonly config: CustomVideoConfig }) {
    const summary = summarizeCustomVideoConfig(config);
    const aliases = summary.enabled.map((feature) => `${featureLabels[feature]}: ${summary.aliases[feature]}`).join("；");
    const media = summarizeMediaRequirements(summary);

    return (
        <div className="max-w-sm space-y-1 text-xs leading-5">
            {summary.seconds ? (
                <SummaryLine
                    label="时长"
                    value={summary.seconds.mode === "range" ? `${summary.seconds.min}-${summary.seconds.max} 秒，步长 ${summary.seconds.step}，默认 ${summary.seconds.default}` : `${summary.seconds.options.join("、")} 秒，默认 ${summary.seconds.default}`}
                />
            ) : null}
            {summary.dimensions ? <SummaryLine label={summary.dimensions.mode === "size" ? "尺寸" : "宽高比"} value={`${summary.dimensions.options.join("、")}，默认 ${summary.dimensions.default}`} /> : null}
            <SummaryLine label="已启用参数" value={summary.enabled.map((feature) => featureLabels[feature]).join("、") || "无"} />
            {aliases ? <SummaryLine label="请求别名" value={aliases} /> : null}
            {media ? <SummaryLine label="素材输入" value={media} /> : null}
            {summary.reference_mode ? <SummaryLine label="兼容参考图模式" value={`${summary.reference_mode.options.join("、")}，默认 ${summary.reference_mode.default}`} /> : null}
            {summary.audio ? <SummaryLine label="音频" value={`${summary.audio.mode === "user" ? "用户可选" : "固定值"}：${summary.audio.value ? "开启" : "关闭"}`} /> : null}
            {summary.n ? <SummaryLine label="生成数量" value={`固定 ${summary.n}`} /> : null}
        </div>
    );
}

export function summarizeMediaRequirements(summary: ReturnType<typeof summarizeCustomVideoConfig>): string {
    return customVideoMediaFeatureNames
        .flatMap((feature) => {
            const limit = summary.media_limits[feature];
            if (limit === undefined) return [];
            return [`${featureLabels[feature]}上限 ${limit}，${summary.media_required[feature] ? "必填" : "可选"}`];
        })
        .join("；");
}

function SummaryLine({ label, value }: { readonly label: string; readonly value: string }) {
    return (
        <div className="flex gap-1">
            <span className="shrink-0 text-stone-500 dark:text-stone-400">{label}：</span>
            <span className="break-all">{value}</span>
        </div>
    );
}

function shouldConfirmOverwrite(value: unknown, presetConfig: CustomVideoConfig): boolean {
    return hasEnabledFeature(value) && !sameConfig(value, presetConfig);
}

function hasEnabledFeature(value: unknown): boolean {
    if (!isRecord(value)) return false;
    return customVideoFeatureNames.some((feature) => isRecord(value[feature]) && value[feature].enabled === true);
}

function sameConfig(value: unknown, presetConfig: CustomVideoConfig): boolean {
    const result = normalizeAndValidateCustomVideoConfig(value);
    return result.ok ? JSON.stringify(result.config) === JSON.stringify(presetConfig) : JSON.stringify(value) === JSON.stringify(presetConfig);
}

function cloneConfig(config: CustomVideoConfig): CustomVideoConfig {
    return structuredClone(config);
}

function errorMessage(error: unknown, fallback: string): string {
    if (!(error instanceof Error)) return fallback;
    return error.message === "预设名称已存在" ? "预设名称已存在，请更换名称" : error.message || fallback;
}

function isRecord(value: unknown): value is Record<string, unknown> {
    return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}
