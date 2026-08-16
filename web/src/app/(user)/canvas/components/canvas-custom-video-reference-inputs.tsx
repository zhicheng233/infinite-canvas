"use client";

import { useEffect, useRef, useState } from "react";
import { Button, Input, Select } from "antd";
import { ChevronDown, ChevronRight, Link, Plus, Trash2, Upload } from "lucide-react";

import { customVideoMediaFeatureNames, customVideoReferenceModes, type CustomVideoConfig, type CustomVideoMediaFeature, type CustomVideoReferenceMode } from "@/lib/custom-video-config";
import { normalizeCustomVideoRuntimeState, type CustomVideoRuntimeSnapshot } from "@/lib/custom-video-runtime";
import type { CanvasTheme } from "@/lib/canvas-theme";
import { uploadImage } from "@/services/image-storage";
import { uploadMediaFile } from "@/services/file-storage";

type CanvasCustomVideoReferenceInputsProps = {
    readonly config: CustomVideoConfig;
    readonly runtime: CustomVideoRuntimeSnapshot;
    readonly theme: CanvasTheme;
    readonly onChange: (runtime: CustomVideoRuntimeSnapshot) => void;
    readonly focusRole?: CustomVideoMediaFeature;
};

const roleLabels: Record<CustomVideoMediaFeature, string> = {
    images: "普通参考图",
    input_reference: "首帧参考图",
    style_references: "风格参考图",
    element_references: "元素参考图",
    reference_images: "兼容参考图",
    input_video: "源视频",
};

const referenceModeLabels: Record<CustomVideoReferenceMode, string> = { frame: "首帧", style: "风格", element: "元素" };

export function canvasCustomVideoReferenceRoles(config: CustomVideoConfig) {
    return customVideoMediaFeatureNames.filter((role) => config[role].enabled);
}

export function appendCanvasCustomVideoMedia(runtime: CustomVideoRuntimeSnapshot, role: CustomVideoMediaFeature, sources: readonly string[], config: CustomVideoConfig) {
    return normalizeCustomVideoRuntimeState(config, runtime.values, {
        ...runtime.media,
        [role]: [...runtime.media[role], ...sources.map((source) => source.trim()).filter(Boolean)].slice(0, config[role].max_count),
    });
}

export function removeCanvasCustomVideoMedia(runtime: CustomVideoRuntimeSnapshot, role: CustomVideoMediaFeature, index: number, config: CustomVideoConfig) {
    return normalizeCustomVideoRuntimeState(config, runtime.values, { ...runtime.media, [role]: runtime.media[role].filter((_, itemIndex) => itemIndex !== index) });
}

export function CanvasCustomVideoReferenceInputs({ config, runtime, theme, onChange, focusRole }: CanvasCustomVideoReferenceInputsProps) {
    const roles = canvasCustomVideoReferenceRoles(config);
    const roleKey = roles.join(",");
    const [expanded, setExpanded] = useState(false);

    useEffect(() => {
        if (focusRole && roles.includes(focusRole)) setExpanded(true);
    }, [focusRole, roleKey]);

    if (!roles.length) return null;
    const selected = roles.reduce((total, role) => total + runtime.media[role].length, 0);
    const requiredRoles = roles.filter((role) => config[role].required);

    return (
        <section className="border-t pt-3" style={{ borderColor: theme.node.stroke }} aria-label="自定义视频素材">
            <button type="button" className="flex w-full cursor-pointer items-center justify-between gap-3 text-left text-sm" style={{ color: theme.node.text }} onClick={() => setExpanded((value) => !value)}>
                <span className="inline-flex items-center gap-1.5 font-medium">
                    {expanded ? <ChevronDown className="size-3.5" /> : <ChevronRight className="size-3.5" />}
                    分角色素材
                </span>
                <span className="text-xs" style={{ color: theme.node.muted }}>
                    已选 {selected} 项
                </span>
            </button>
            {requiredRoles.length ? (
                <div className="mt-1.5 text-xs" style={{ color: theme.node.muted }}>
                    必填：{requiredRoles.map((role) => roleLabels[role]).join("、")}
                </div>
            ) : null}
            {expanded ? (
                <div className="mt-3 space-y-2.5">
                    {roles.map((role) => (
                        <CanvasCustomVideoRoleInput key={role} role={role} config={config} runtime={runtime} theme={theme} onChange={onChange} />
                    ))}
                </div>
            ) : null}
        </section>
    );
}

function CanvasCustomVideoRoleInput({
    role,
    config,
    runtime,
    theme,
    onChange,
}: {
    readonly role: CustomVideoMediaFeature;
    readonly config: CustomVideoConfig;
    readonly runtime: CustomVideoRuntimeSnapshot;
    readonly theme: CanvasTheme;
    readonly onChange: (runtime: CustomVideoRuntimeSnapshot) => void;
}) {
    const inputRef = useRef<HTMLInputElement>(null);
    const [url, setUrl] = useState("");
    const sources = runtime.media[role];
    const atLimit = sources.length >= config[role].max_count;
    const isVideo = role === "input_video";
    const showReferenceMode = role === "reference_images" && config.reference_mode.enabled;

    const addUrl = () => {
        if (!url.trim() || atLimit) return;
        onChange(appendCanvasCustomVideoMedia(runtime, role, [url], config));
        setUrl("");
    };

    const addFiles = async (files: FileList | null) => {
        if (!files || atLimit) return;
        const sources = await uploadCanvasCustomMediaFiles(files, isVideo, config[role].max_count - runtime.media[role].length);
        onChange(appendCanvasCustomVideoMedia(runtime, role, sources, config));
    };

    return (
        <div className="rounded-md border px-2.5 py-2" style={{ borderColor: theme.node.stroke, background: theme.node.fill }}>
            <div className="flex flex-wrap items-center justify-between gap-2">
                <div className="flex min-w-0 items-center gap-2">
                    <span className="truncate text-xs font-medium">
                        {roleLabels[role]}（{config[role].required ? "必填" : "可选"}）
                    </span>
                    {showReferenceMode ? (
                        <Select
                            className="canvas-control-select min-w-24"
                            size="small"
                            value={runtime.values.reference_mode}
                            options={config.reference_mode.options.map((value) => ({ value, label: referenceModeLabels[value] }))}
                            aria-label="参考图模式"
                            onChange={(value: CustomVideoReferenceMode) => {
                                if (!customVideoReferenceModes.includes(value)) return;
                                onChange(normalizeCustomVideoRuntimeState(config, { ...runtime.values, reference_mode: value }, runtime.media));
                            }}
                        />
                    ) : null}
                </div>
                <span className="shrink-0 text-xs" style={{ color: theme.node.muted }}>
                    {sources.length} / {config[role].max_count}
                </span>
            </div>
            {sources.length ? (
                <div className="mt-2 space-y-1">
                    {sources.map((source, index) => (
                        <div key={`${source}-${index}`} className="flex min-w-0 items-center gap-1.5 text-xs" style={{ color: theme.node.muted }}>
                            <span className="min-w-0 flex-1 truncate">{source}</span>
                            <Button
                                type="text"
                                size="small"
                                className="!size-6 !min-w-6 !p-0"
                                icon={<Trash2 className="size-3" />}
                                aria-label={`删除${roleLabels[role]} ${index + 1}`}
                                onClick={() => onChange(removeCanvasCustomVideoMedia(runtime, role, index, config))}
                            />
                        </div>
                    ))}
                </div>
            ) : null}
            <div className="mt-2 grid grid-cols-[minmax(0,1fr)_auto_auto] gap-1.5">
                <Input size="small" value={url} prefix={<Link className="size-3" />} placeholder={isVideo ? "视频 URL 或 Data URI" : "图片 URL 或 Data URI"} disabled={atLimit} onChange={(event) => setUrl(event.target.value)} onPressEnter={addUrl} />
                <Button size="small" icon={<Plus className="size-3" />} disabled={atLimit || !url.trim()} onClick={addUrl} aria-label={`添加${roleLabels[role]}链接`} />
                <Button size="small" icon={<Upload className="size-3" />} disabled={atLimit} onClick={() => inputRef.current?.click()} aria-label={`上传${roleLabels[role]}`} />
            </div>
            <input
                ref={inputRef}
                type="file"
                className="hidden"
                accept={isVideo ? "video/*" : "image/*"}
                multiple={!isVideo && config[role].max_count > 1}
                onChange={(event) => {
                    void addFiles(event.target.files);
                    event.target.value = "";
                }}
            />
        </div>
    );
}

async function uploadCanvasCustomMediaFiles(files: FileList, isVideo: boolean, limit: number) {
    const candidates = Array.from(files)
        .filter((file) => (isVideo ? file.type.startsWith("video/") : file.type.startsWith("image/")))
        .slice(0, limit);
    return Promise.all(candidates.map(async (file) => (isVideo ? (await uploadMediaFile(file, "video")).url : (await uploadImage(file)).url)));
}
