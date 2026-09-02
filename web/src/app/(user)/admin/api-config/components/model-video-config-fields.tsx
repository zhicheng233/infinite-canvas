"use client";

import { Alert, Form, Input, Select, Switch } from "antd";
import type { FormInstance } from "antd";

import { createDefaultCustomVideoConfig, normalizeAndValidateCustomVideoConfig, normalizeCustomVideoConfig, type CustomVideoConfig } from "@/lib/custom-video-config";
import { CustomVideoConfigEditor, formatCustomVideoConfigError } from "./custom-video-config-editor";

const videoRouteOptions = [
    { label: "兼容默认 /v1/videos", value: "auto" },
    { label: "/v1/videos", value: "openai" },
    { label: "/v1/videos（JSON / veo）", value: "veo_json" },
    { label: "/v1/videos（JSON / yijia）", value: "yijia" },
    { label: "/v1/videos JSON / Waninter", value: "waninter" },
    { label: "/v1/videos/generations", value: "xai" },
    { label: "/v1/video/generations", value: "newapi" },
    { label: "Seedance /contents/generations/tasks", value: "seedance" },
    { label: "自定义固定参数目录", value: "custom" },
];

type ModelVideoConfigFieldsProps = {
    form: FormInstance;
    disabled: boolean;
    binghuo: boolean;
};

type ModelVideoFormValues = {
    readonly video_route?: string;
    readonly video_durations?: string;
    readonly video_customizable?: boolean;
    readonly video_custom_config?: CustomVideoConfig | null;
};

export function ModelVideoConfigFields({ form, disabled, binghuo }: ModelVideoConfigFieldsProps) {
    const videoRoute = Form.useWatch("video_route", form);
    const clearForRoute = (route: string) => {
        if (route === "custom") {
            if (videoRoute !== "custom") {
                form.setFieldValue("video_durations", undefined);
                form.setFieldValue("video_customizable", undefined);
                form.setFieldValue("video_custom_config", createDefaultCustomVideoConfig());
            }
            return;
        }
        if (videoRoute === "custom") {
            form.setFieldValue("video_custom_config", undefined);
            form.setFieldValue("video_durations", "");
            form.setFieldValue("video_customizable", false);
        }
    };

    return (
        <>
            <Form.Item name="video_route" label="视频生成接口路由">
                <Select options={videoRouteOptions} disabled={disabled || binghuo} onChange={clearForRoute} />
            </Form.Item>
            {binghuo ? <Alert className="mb-4" type="info" showIcon message="该渠道使用炳火 API 标准，模型视频路由暂不生效。" /> : null}
            <Form.Item noStyle shouldUpdate={(previous, current) => previous.video_route !== current.video_route}>
                {() =>
                    form.getFieldValue("video_route") === "custom" && !binghuo ? (
                        <CustomVideoConfigEditor form={form} disabled={disabled} />
                    ) : (
                        <>
                            <Form.Item name="video_durations" label="可选视频时长 (逗号分隔)" help="多个时长用半角逗号分隔，例如: 5,10">
                                <Input placeholder="如: 5,10" disabled={disabled} />
                            </Form.Item>
                            <Form.Item name="video_customizable" valuePropName="checked" label="允许用户自定义视频时长">
                                <Switch disabled={disabled} />
                            </Form.Item>
                        </>
                    )
                }
            </Form.Item>
        </>
    );
}

export function initialVideoModelFormValues(videoRoute: string, videoDurations: readonly number[], videoCustomizable: boolean, videoCustomConfig: CustomVideoConfig | null | undefined): ModelVideoFormValues {
    const customConfig = normalizeCustomVideoConfig(videoCustomConfig) ?? createDefaultCustomVideoConfig();
    return {
        video_route: videoRoute || "auto",
        video_durations: videoDurations.join(","),
        video_customizable: videoCustomizable,
        video_custom_config: videoRoute === "custom" ? customConfig : undefined,
    };
}

export function normalizeVideoModelFormValues(values: ModelVideoFormValues) {
    const videoRoute = values.video_route?.trim() || "auto";
    if (videoRoute === "custom") {
        const result = normalizeAndValidateCustomVideoConfig(values.video_custom_config);
        if (!result.ok) throw new Error(result.errors.map(formatCustomVideoConfigError).join("；"));
        return { video_route: videoRoute, video_custom_config: result.config };
    }
    return {
        video_route: videoRoute,
        video_durations: parseDurationInput(values.video_durations || ""),
        video_customizable: Boolean(values.video_customizable),
        video_custom_config: null,
    };
}

function parseDurationInput(value: string) {
    return Array.from(
        new Set(
            value
                .split(",")
                .map((item) => Math.floor(Number(item.trim()) || 0))
                .filter((item) => item > 0),
        ),
    ).sort((left, right) => left - right);
}
