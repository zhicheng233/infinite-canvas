"use client";

import { Alert, Form, Input, InputNumber, Segmented, Select, Switch, Typography } from "antd";
import type { FormInstance } from "antd";

import {
    createDefaultCustomVideoConfig,
    customVideoDimensionDefaultKeys,
    customVideoMediaHardLimits,
    customVideoReferenceModes,
    normalizeAndValidateCustomVideoConfig,
    type CustomVideoConfig,
    type CustomVideoMediaFeature,
} from "@/lib/custom-video-config";
import { CustomVideoConfigPresets } from "./custom-video-config-presets";

type CustomVideoConfigEditorProps = {
    form: FormInstance;
    disabled: boolean;
};

const { Text } = Typography;

const fieldLabels = {
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
    key: "请求字段名",
    mode: "模式",
    min: "最小值",
    max: "最大值",
    step: "步长",
    default: "默认值",
    options: "可选项",
    max_count: "最大数量",
    value: "固定值",
} as const;

export function CustomVideoConfigEditor({ form, disabled }: CustomVideoConfigEditorProps) {
    const config = Form.useWatch("video_custom_config", form);
    const result = normalizeAndValidateCustomVideoConfig(config);

    return (
        <div className="space-y-4 rounded-lg border border-stone-200 p-4 dark:border-stone-800">
            <div>
                <div className="font-medium">自定义视频请求参数</div>
                <Text type="secondary" className="text-xs">
                    仅启用需要发送的固定目录参数；模型和提示词始终由系统注入。
                </Text>
            </div>

            <CustomVideoConfigPresets form={form} disabled={disabled} />

            {!result.ok ? <Alert type="error" showIcon message="自定义视频配置有误" description={result.errors.map(formatCustomVideoConfigError).join("；")} /> : null}

            <ConfigSection title="时长">
                <SwitchField form={form} path={["seconds", "enabled"]} label="启用时长" disabled={disabled} reset={() => form.setFieldValue(["video_custom_config", "seconds"], createDefaultCustomVideoConfig().seconds)} />
                <Form.Item noStyle shouldUpdate={(previous, current) => previous.video_custom_config?.seconds?.enabled !== current.video_custom_config?.seconds?.enabled}>
                    {() =>
                        form.getFieldValue(["video_custom_config", "seconds", "enabled"]) ? (
                            <div className="grid gap-3 md:grid-cols-2">
                                <KeyField form={form} feature="seconds" disabled={disabled} />
                                <Form.Item name={["video_custom_config", "seconds", "mode"]} label="时长模式" rules={[{ required: true, message: "请选择时长模式" }]}>
                                    <Segmented
                                        block
                                        options={[
                                            { label: "连续范围", value: "range" },
                                            { label: "离散选项", value: "options" },
                                        ]}
                                        disabled={disabled}
                                        onChange={(mode) => {
                                            const defaults = createDefaultCustomVideoConfig().seconds;
                                            form.setFieldValue(["video_custom_config", "seconds"], mode === "range" ? { ...defaults, enabled: true } : { enabled: true, key: defaults.key, mode: "options", options: [], default: 0 });
                                        }}
                                    />
                                </Form.Item>
                                <Form.Item noStyle shouldUpdate={(previous, current) => previous.video_custom_config?.seconds?.mode !== current.video_custom_config?.seconds?.mode}>
                                    {() =>
                                        form.getFieldValue(["video_custom_config", "seconds", "mode"]) === "options" ? (
                                            <>
                                                <Form.Item name={["video_custom_config", "seconds", "options"]} label="可选秒数" rules={[{ required: true, message: "请输入至少一个可选秒数" }]}>
                                                    <Select
                                                        mode="tags"
                                                        tokenSeparators={[","]}
                                                        placeholder="例如：3,5,10"
                                                        disabled={disabled}
                                                        onChange={(options) => form.setFieldValue(["video_custom_config", "seconds", "options"], options.map(Number))}
                                                    />
                                                </Form.Item>
                                                <Form.Item name={["video_custom_config", "seconds", "default"]} label="默认秒数" rules={[{ required: true, message: "请输入默认秒数" }]}>
                                                    <InputNumber min={1} max={3600} className="w-full" disabled={disabled} />
                                                </Form.Item>
                                            </>
                                        ) : (
                                            <>
                                                <NumberField name="min" label="最小秒数" disabled={disabled} />
                                                <NumberField name="max" label="最大秒数" disabled={disabled} />
                                                <NumberField name="step" label="步长" disabled={disabled} />
                                                <NumberField name="default" label="默认秒数" disabled={disabled} />
                                            </>
                                        )
                                    }
                                </Form.Item>
                            </div>
                        ) : null
                    }
                </Form.Item>
            </ConfigSection>

            <ConfigSection title="尺寸或宽高比">
                <SwitchField form={form} path={["dimensions", "enabled"]} label="启用尺寸" disabled={disabled} reset={() => form.setFieldValue(["video_custom_config", "dimensions"], createDefaultCustomVideoConfig().dimensions)} />
                <Form.Item noStyle shouldUpdate={(previous, current) => previous.video_custom_config?.dimensions?.enabled !== current.video_custom_config?.dimensions?.enabled}>
                    {() =>
                        form.getFieldValue(["video_custom_config", "dimensions", "enabled"]) ? (
                            <div className="grid gap-3 md:grid-cols-2">
                                <KeyField form={form} feature="dimensions" disabled={disabled} />
                                <Form.Item name={["video_custom_config", "dimensions", "mode"]} label="参数类型" rules={[{ required: true, message: "请选择参数类型" }]}>
                                    <Segmented
                                        block
                                        options={[
                                            { label: "尺寸", value: "size" },
                                            { label: "宽高比", value: "aspect_ratio" },
                                        ]}
                                        disabled={disabled}
                                        onChange={(mode) => {
                                            const dimensionMode = mode === "aspect_ratio" ? "aspect_ratio" : "size";
                                            form.setFieldValue(["video_custom_config", "dimensions"], {
                                                ...createDefaultCustomVideoConfig().dimensions,
                                                enabled: true,
                                                mode: dimensionMode,
                                                key: customVideoDimensionDefaultKeys[dimensionMode],
                                            });
                                        }}
                                    />
                                </Form.Item>
                                <Form.Item name={["video_custom_config", "dimensions", "options"]} label="可选值" rules={[{ required: true, message: "请输入至少一个可选值" }]}>
                                    <Select mode="tags" tokenSeparators={[","]} placeholder="例如：1280x720,720x1280" disabled={disabled} />
                                </Form.Item>
                                <Form.Item name={["video_custom_config", "dimensions", "default"]} label="默认值" rules={[{ required: true, message: "请输入默认值" }]}>
                                    <Input placeholder="必须位于可选值中" disabled={disabled} />
                                </Form.Item>
                            </div>
                        ) : null
                    }
                </Form.Item>
            </ConfigSection>

            <ConfigSection title="素材输入">
                <MediaConfigSection form={form} name="images" label="普通参考图" disabled={disabled} />
                <MediaConfigSection form={form} name="input_reference" label="首帧参考图" disabled={disabled} />
                <MediaConfigSection form={form} name="style_references" label="风格参考图" disabled={disabled} />
                <MediaConfigSection form={form} name="element_references" label="元素参考图" disabled={disabled} />
                <MediaConfigSection form={form} name="reference_images" label="兼容参考图" disabled={disabled} />
                <MediaConfigSection form={form} name="input_video" label="源视频" disabled={disabled} />
            </ConfigSection>

            <ConfigSection title="兼容参考图模式">
                <SwitchField form={form} path={["reference_mode", "enabled"]} label="启用兼容参考图模式" disabled={disabled} reset={() => form.setFieldValue(["video_custom_config", "reference_mode"], createDefaultCustomVideoConfig().reference_mode)} />
                <Form.Item noStyle shouldUpdate={(previous, current) => previous.video_custom_config?.reference_mode?.enabled !== current.video_custom_config?.reference_mode?.enabled}>
                    {() =>
                        form.getFieldValue(["video_custom_config", "reference_mode", "enabled"]) ? (
                            <div className="grid gap-3 md:grid-cols-2">
                                <KeyField form={form} feature="reference_mode" disabled={disabled} />
                                <Form.Item name={["video_custom_config", "reference_mode", "options"]} label="可选模式" rules={[{ required: true, message: "请选择至少一种参考图模式" }]}>
                                    <Select mode="multiple" options={customVideoReferenceModes.map((value) => ({ label: referenceModeLabel(value), value }))} disabled={disabled} />
                                </Form.Item>
                                <Form.Item name={["video_custom_config", "reference_mode", "default"]} label="默认模式" rules={[{ required: true, message: "请选择默认参考图模式" }]}>
                                    <Select options={customVideoReferenceModes.map((value) => ({ label: referenceModeLabel(value), value }))} disabled={disabled} />
                                </Form.Item>
                            </div>
                        ) : null
                    }
                </Form.Item>
            </ConfigSection>

            <ConfigSection title="音频">
                <SwitchField form={form} path={["audio", "enabled"]} label="启用音频" disabled={disabled} reset={() => form.setFieldValue(["video_custom_config", "audio"], createDefaultCustomVideoConfig().audio)} />
                <Form.Item noStyle shouldUpdate={(previous, current) => previous.video_custom_config?.audio?.enabled !== current.video_custom_config?.audio?.enabled}>
                    {() =>
                        form.getFieldValue(["video_custom_config", "audio", "enabled"]) ? (
                            <div className="grid gap-3 md:grid-cols-2">
                                <KeyField form={form} feature="audio" disabled={disabled} />
                                <Form.Item name={["video_custom_config", "audio", "mode"]} label="音频模式" rules={[{ required: true, message: "请选择音频模式" }]}>
                                    <Segmented
                                        block
                                        options={[
                                            { label: "固定值", value: "fixed" },
                                            { label: "用户可选", value: "user" },
                                        ]}
                                        disabled={disabled}
                                    />
                                </Form.Item>
                                <Form.Item name={["video_custom_config", "audio", "value"]} valuePropName="checked" label="默认生成声音">
                                    <Switch disabled={disabled} />
                                </Form.Item>
                            </div>
                        ) : null
                    }
                </Form.Item>
            </ConfigSection>

            <ConfigSection title="生成数量">
                <SwitchField form={form} path={["n", "enabled"]} label="固定发送生成数量" disabled={disabled} reset={() => form.setFieldValue(["video_custom_config", "n"], createDefaultCustomVideoConfig().n)} />
                <Form.Item noStyle shouldUpdate={(previous, current) => previous.video_custom_config?.n?.enabled !== current.video_custom_config?.n?.enabled}>
                    {() =>
                        form.getFieldValue(["video_custom_config", "n", "enabled"]) ? (
                            <div className="grid gap-3 md:grid-cols-2">
                                <KeyField form={form} feature="n" disabled={disabled} />
                                <Form.Item name={["video_custom_config", "n", "value"]} label="固定数量" rules={[{ required: true, message: "请输入固定生成数量" }]}>
                                    <InputNumber min={1} max={16} className="w-full" disabled={disabled} />
                                </Form.Item>
                            </div>
                        ) : null
                    }
                </Form.Item>
            </ConfigSection>
        </div>
    );
}

function ConfigSection({ title, children }: { title: string; children: React.ReactNode }) {
    return (
        <section className="space-y-3 border-t border-stone-200 pt-4 first:border-t-0 first:pt-0 dark:border-stone-800">
            <div className="text-sm font-medium">{title}</div>
            {children}
        </section>
    );
}

function SwitchField({ form, path, label, disabled, reset }: { form: FormInstance; path: string[]; label: string; disabled: boolean; reset: () => void }) {
    return (
        <Form.Item name={["video_custom_config", ...path]} valuePropName="checked" label={label} className="!mb-0">
            <Switch disabled={disabled} onChange={(enabled) => !enabled && reset()} />
        </Form.Item>
    );
}

function KeyField({ form, feature, disabled }: { form: FormInstance; feature: keyof CustomVideoConfig; disabled: boolean }) {
    return (
        <Form.Item dependencies={[["video_custom_config"]]} name={["video_custom_config", feature, "key"]} label="请求字段名" rules={[{ required: true, whitespace: true, message: "请输入请求字段名" }, keyRule(form, feature)]}>
            <Input disabled={disabled} />
        </Form.Item>
    );
}

function NumberField({ name, label, disabled }: { name: "min" | "max" | "step" | "default"; label: string; disabled: boolean }) {
    return (
        <Form.Item name={["video_custom_config", "seconds", name]} label={label} rules={[{ required: true, message: `请输入${label}` }]}>
            <InputNumber min={1} max={3600} className="w-full" disabled={disabled} />
        </Form.Item>
    );
}

function MediaConfigSection({ form, name, label, disabled }: { form: FormInstance; name: CustomVideoMediaFeature; label: string; disabled: boolean }) {
    return (
        <div className="rounded-md bg-stone-50 p-3 dark:bg-stone-900/40">
            <SwitchField form={form} path={[name, "enabled"]} label={`启用${label}`} disabled={disabled} reset={() => form.setFieldValue(["video_custom_config", name], createDefaultCustomVideoConfig()[name])} />
            <Form.Item noStyle shouldUpdate={(previous, current) => previous.video_custom_config?.[name]?.enabled !== current.video_custom_config?.[name]?.enabled}>
                {() =>
                    form.getFieldValue(["video_custom_config", name, "enabled"]) ? (
                        <div className="mt-3 grid gap-3 md:grid-cols-2">
                            <KeyField form={form} feature={name} disabled={disabled} />
                            <Form.Item name={["video_custom_config", name, "max_count"]} label="最大数量" rules={[{ required: true, message: "请输入最大数量" }]}>
                                <InputNumber min={1} max={customVideoMediaHardLimits[name]} className="w-full" disabled={disabled} />
                            </Form.Item>
                        </div>
                    ) : null
                }
            </Form.Item>
        </div>
    );
}

function keyRule(form: FormInstance, feature: keyof CustomVideoConfig) {
    return {
        validator: () => {
            const result = normalizeAndValidateCustomVideoConfig(form.getFieldValue("video_custom_config"));
            if (result.ok) return Promise.resolve();
            const error = result.errors.find((item) => item.startsWith(`${feature}.key`));
            return error ? Promise.reject(new Error(formatCustomVideoConfigError(error))) : Promise.resolve();
        },
    };
}

function referenceModeLabel(value: string) {
    if (value === "frame") return "首帧";
    if (value === "style") return "风格";
    return "元素";
}

export function formatCustomVideoConfigError(error: string) {
    const names = Object.keys(fieldLabels)
        .sort((left, right) => right.length - left.length)
        .join("|");
    return error.replace(new RegExp(`(^|[.\\s])(${names})(?=[.\\s]|$)`, "g"), (_match, prefix: string, name: keyof typeof fieldLabels) => `${prefix}${fieldLabels[name]}`);
}
