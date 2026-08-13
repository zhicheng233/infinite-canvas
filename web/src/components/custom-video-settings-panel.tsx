"use client";

import { useState, type ReactNode } from "react";
import { Slider, Switch } from "antd";

import { ImageSettingsTheme } from "@/components/image-settings-panel";
import { type CanvasTheme } from "@/lib/canvas-theme";
import { customVideoMediaFeatureNames, type CustomVideoConfig, type CustomVideoMediaFeature } from "@/lib/custom-video-config";
import { normalizeCustomVideoRuntimeState, type CustomVideoRuntimeSnapshot } from "@/lib/custom-video-runtime";

export type CustomVideoSettingsState = { readonly kind: "invalid" } | { readonly kind: "ready"; readonly config: CustomVideoConfig; readonly runtime: CustomVideoRuntimeSnapshot };

type CustomVideoSettingsPanelProps = {
    config: CustomVideoConfig | null;
    runtime?: CustomVideoRuntimeSnapshot;
    onRuntimeChange?: (runtime: CustomVideoRuntimeSnapshot) => void;
    onMediaRoleOpen?: (role: CustomVideoMediaFeature) => void;
    theme: CanvasTheme;
    showTitle?: boolean;
    className?: string;
};

export function resolveCustomVideoSettingsState(config: CustomVideoConfig | null, runtime?: CustomVideoRuntimeSnapshot): CustomVideoSettingsState {
    if (!config) return { kind: "invalid" };
    return { kind: "ready", config, runtime: normalizeCustomVideoRuntimeState(config, runtime?.values, runtime?.media) };
}

export function CustomVideoSettingsPanel({ config, runtime: providedRuntime, onRuntimeChange, onMediaRoleOpen, theme, showTitle = true, className = "w-[320px] space-y-4 rounded-2xl px-1 py-0.5" }: CustomVideoSettingsPanelProps) {
    const [localRuntime, setLocalRuntime] = useState<CustomVideoRuntimeSnapshot>();
    const state = resolveCustomVideoSettingsState(config, providedRuntime || localRuntime);

    if (state.kind === "invalid") {
        return (
            <ImageSettingsTheme theme={theme}>
                <div className={className} style={{ color: theme.node.text }} onMouseDown={(event) => event.stopPropagation()} data-generation-blocked="true">
                    {showTitle ? <div className="text-lg font-semibold">视频设置</div> : null}
                    <div role="alert" className="rounded-xl border px-3 py-2.5 text-sm leading-5" style={{ borderColor: theme.node.activeStroke, background: theme.node.fill }}>
                        该模型的自定义视频配置无效，请联系管理员。生成已禁用。
                    </div>
                </div>
            </ImageSettingsTheme>
        );
    }

    const updateRuntime = (values: CustomVideoRuntimeSnapshot["values"]) => {
        const next = normalizeCustomVideoRuntimeState(state.config, values, state.runtime.media);
        setLocalRuntime(next);
        onRuntimeChange?.(next);
    };
    const { seconds, dimensions, audio } = state.config;

    return (
        <ImageSettingsTheme theme={theme}>
            <div className={className} style={{ color: theme.node.text }} onMouseDown={(event) => event.stopPropagation()}>
                {showTitle ? <div className="text-lg font-semibold">视频设置</div> : null}
                {seconds.enabled ? (
                    <SettingGroup title="秒数" theme={theme}>
                        {seconds.mode === "range" ? (
                            <div className="space-y-2">
                                <div className="flex items-center justify-between text-sm">
                                    <span style={{ color: theme.node.muted }}>
                                        {seconds.min}s - {seconds.max}s
                                    </span>
                                    <span className="font-medium">{state.runtime.values.seconds}s</span>
                                </div>
                                <Slider
                                    min={seconds.min}
                                    max={seconds.max}
                                    step={seconds.step}
                                    marks={{ [seconds.min]: `${seconds.min}s`, [seconds.max]: `${seconds.max}s` }}
                                    value={state.runtime.values.seconds}
                                    tooltip={{ formatter: (value) => `${value}s` }}
                                    styles={{ handle: { borderColor: theme.node.activeStroke }, rail: { background: theme.node.stroke }, track: { background: theme.node.activeStroke } }}
                                    onChange={(value) => {
                                        if (typeof value === "number") updateRuntime({ ...state.runtime.values, seconds: value });
                                    }}
                                />
                            </div>
                        ) : (
                            <div className="grid grid-cols-3 gap-2.5">
                                {seconds.options.map((value) => (
                                    <OptionPill key={value} selected={state.runtime.values.seconds === value} theme={theme} onClick={() => updateRuntime({ ...state.runtime.values, seconds: value })}>
                                        {value}s
                                    </OptionPill>
                                ))}
                            </div>
                        )}
                    </SettingGroup>
                ) : null}
                {dimensions.enabled ? (
                    <SettingGroup title={dimensions.mode === "size" ? "尺寸" : "宽高比"} theme={theme}>
                        <div className="grid grid-cols-2 gap-2.5">
                            {dimensions.options.map((option) => (
                                <DimensionOption key={option} option={option} selected={state.runtime.values.dimension === option} theme={theme} onClick={() => updateRuntime({ ...state.runtime.values, dimension: option })} />
                            ))}
                        </div>
                    </SettingGroup>
                ) : null}
                {state.config.reference_images.enabled && state.config.reference_mode.enabled ? (
                    <SettingGroup title="参考图模式" theme={theme}>
                        <div className="grid grid-cols-3 gap-2.5">
                            {state.config.reference_mode.options.map((value) => (
                                <OptionPill key={value} selected={state.runtime.values.reference_mode === value} theme={theme} onClick={() => updateRuntime({ ...state.runtime.values, reference_mode: value })}>
                                    {referenceModeLabels[value]}
                                </OptionPill>
                            ))}
                        </div>
                    </SettingGroup>
                ) : null}
                {audio.enabled && audio.mode === "user" ? (
                    <SettingGroup title="输出" theme={theme}>
                        <div className="rounded-xl border px-2.5" style={{ borderColor: theme.node.stroke }}>
                            <SwitchRow label="生成声音" checked={state.runtime.values.audio ?? audio.value} theme={theme} onChange={(checked) => updateRuntime({ ...state.runtime.values, audio: checked })} />
                        </div>
                    </SettingGroup>
                ) : null}
                <MediaSummary config={state.config} runtime={state.runtime} theme={theme} onMediaRoleOpen={onMediaRoleOpen} />
            </div>
        </ImageSettingsTheme>
    );
}

function SettingGroup({ title, theme, children }: { title: string; theme: CanvasTheme; children: ReactNode }) {
    return (
        <div className="space-y-2.5">
            <div className="text-xs font-medium" style={{ color: theme.node.muted }}>
                {title}
            </div>
            {children}
        </div>
    );
}

function OptionPill({ selected, theme, onClick, children }: { selected: boolean; theme: CanvasTheme; onClick: () => void; children: ReactNode }) {
    return (
        <button
            type="button"
            className="h-9 cursor-pointer rounded-full border px-2 text-sm transition hover:opacity-80"
            style={{ background: "transparent", borderColor: selected ? theme.node.text : theme.node.stroke, color: theme.node.text }}
            onMouseDown={(event) => event.stopPropagation()}
            onClick={onClick}
        >
            {children}
        </button>
    );
}

function DimensionOption({ option, selected, theme, onClick }: { option: string; selected: boolean; theme: CanvasTheme; onClick: () => void }) {
    const preview = option.includes("x") ? dimensionsFor(option) : ratioFor(option);
    return (
        <button
            type="button"
            className="flex h-16 cursor-pointer flex-col items-center justify-center gap-1 rounded-xl border bg-transparent px-2 text-sm transition hover:opacity-80"
            style={{ borderColor: selected ? theme.node.text : theme.node.stroke, color: theme.node.text }}
            onMouseDown={(event) => event.stopPropagation()}
            onClick={onClick}
        >
            <SizePreview width={preview.width} height={preview.height} color={theme.node.text} />
            <span className="max-w-full truncate">{option}</span>
        </button>
    );
}

function MediaSummary({ config, runtime, theme, onMediaRoleOpen }: { config: CustomVideoConfig; runtime: CustomVideoRuntimeSnapshot; theme: CanvasTheme; onMediaRoleOpen?: (role: CustomVideoMediaFeature) => void }) {
    const media = customVideoMediaFeatureNames.filter((role) => config[role].enabled);
    if (!media.length) return null;
    return (
        <SettingGroup title="素材" theme={theme}>
            <div className="grid gap-2">
                {media.map((role) => (
                    <div key={role} className="flex items-center justify-between gap-3 rounded-xl border px-2.5 py-2 text-sm" style={{ borderColor: theme.node.stroke }}>
                        <span className="min-w-0">
                            <span className="block truncate">{mediaLabels[role]}</span>
                            <span className="block text-xs" style={{ color: theme.node.muted }}>
                                已选 {runtime.media[role].length} / {config[role].max_count}
                            </span>
                        </span>
                        <button type="button" disabled={!onMediaRoleOpen} className="shrink-0 text-xs disabled:opacity-55" style={{ color: theme.node.muted }} onMouseDown={(event) => event.stopPropagation()} onClick={() => onMediaRoleOpen?.(role)}>
                            打开输入区
                        </button>
                    </div>
                ))}
            </div>
        </SettingGroup>
    );
}

function SwitchRow({ label, checked, theme, onChange }: { label: string; checked: boolean; theme: CanvasTheme; onChange: (checked: boolean) => void }) {
    return (
        <div className="flex h-8 items-center justify-between gap-3">
            <span className="text-sm" style={{ color: theme.node.text }}>
                {label}
            </span>
            <span onMouseDown={(event) => event.stopPropagation()}>
                <Switch size="small" checked={checked} onChange={onChange} />
            </span>
        </div>
    );
}

function SizePreview({ width, height, color }: { width: number; height: number; color: string }) {
    if (!width || !height) return null;
    const longSide = Math.max(width, height);
    return <span className="rounded-[3px] border-2" style={{ width: Math.max(10, Math.round((width / longSide) * 26)), height: Math.max(10, Math.round((height / longSide) * 26)), borderColor: color }} />;
}

function dimensionsFor(value: string) {
    const match = value.match(/^(\d+)x(\d+)$/);
    return { width: Number(match?.[1]) || 16, height: Number(match?.[2]) || 9 };
}

function ratioFor(value: string) {
    const match = value.match(/^(\d+):(\d+)$/);
    return { width: Number(match?.[1]) || 16, height: Number(match?.[2]) || 9 };
}

const mediaLabels: Record<CustomVideoMediaFeature, string> = { images: "普通参考图", input_reference: "首帧参考图", style_references: "风格参考图", element_references: "元素参考图", reference_images: "兼容参考图", input_video: "源视频" };

const referenceModeLabels = { frame: "首帧", style: "风格", element: "元素" } as const;
