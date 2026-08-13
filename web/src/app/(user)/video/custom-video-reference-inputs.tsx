"use client";

import { Link, Plus, Trash2, Upload, VideoIcon } from "lucide-react";
import { useRef, useState } from "react";
import { Button, Input, Select } from "antd";

import { customVideoMediaFeatureNames, customVideoReferenceModes, type CustomVideoConfig, type CustomVideoMediaFeature, type CustomVideoReferenceMode } from "@/lib/custom-video-config";
import type { CustomVideoMediaState } from "@/lib/custom-video-runtime";

type CustomVideoReferenceInputsProps = {
    readonly config: CustomVideoConfig;
    readonly media: CustomVideoMediaState;
    readonly referenceMode?: CustomVideoReferenceMode;
    readonly onChange: (next: { readonly media: CustomVideoMediaState; readonly referenceMode?: CustomVideoReferenceMode }) => void;
    readonly onUpload: (role: CustomVideoMediaFeature, files: FileList | null) => Promise<readonly string[]>;
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

export function customVideoReferenceInputRoles(config: CustomVideoConfig): CustomVideoMediaFeature[] {
    return customVideoMediaFeatureNames.filter((role) => config[role].enabled);
}

export function appendCustomVideoMedia(media: CustomVideoMediaState, role: CustomVideoMediaFeature, sources: readonly string[], maxCount: number): CustomVideoMediaState {
    return { ...media, [role]: [...media[role], ...sources.map((source) => source.trim()).filter(Boolean)].slice(0, maxCount) };
}

export function removeCustomVideoMedia(media: CustomVideoMediaState, role: CustomVideoMediaFeature, index: number): CustomVideoMediaState {
    return { ...media, [role]: media[role].filter((_, itemIndex) => itemIndex !== index) };
}

export function CustomVideoReferenceInputs({ config, media, referenceMode, onChange, onUpload }: CustomVideoReferenceInputsProps) {
    const enabledRoles = customVideoReferenceInputRoles(config);
    if (!enabledRoles.length) return null;

    const updateMedia = (role: CustomVideoMediaFeature, next: CustomVideoMediaState) => onChange({ media: next, referenceMode });

    return (
        <section className="space-y-4" aria-label="自定义视频素材">
            <div className="flex items-center justify-between gap-3">
                <span className="text-base font-semibold">参考素材</span>
                <span className="text-xs text-stone-500 dark:text-stone-400">按模型配置分别添加</span>
            </div>
            {enabledRoles.map((role) => (
                <RoleMediaSection
                    key={role}
                    role={role}
                    label={roleLabels[role]}
                    sources={media[role]}
                    maxCount={config[role].max_count}
                    onAdd={(sources) => updateMedia(role, appendCustomVideoMedia(media, role, sources, config[role].max_count))}
                    onRemove={(index) => updateMedia(role, removeCustomVideoMedia(media, role, index))}
                    onUpload={(files) => onUpload(role, files)}
                    referenceMode={role === "reference_images" && config.reference_mode.enabled ? referenceMode : undefined}
                    referenceModeOptions={role === "reference_images" && config.reference_mode.enabled ? config.reference_mode.options : undefined}
                    onReferenceModeChange={(value) => onChange({ media, referenceMode: value })}
                />
            ))}
        </section>
    );
}

function RoleMediaSection({
    role,
    label,
    sources,
    maxCount,
    onAdd,
    onRemove,
    onUpload,
    referenceMode,
    referenceModeOptions,
    onReferenceModeChange,
}: {
    readonly role: CustomVideoMediaFeature;
    readonly label: string;
    readonly sources: readonly string[];
    readonly maxCount: number;
    readonly onAdd: (sources: readonly string[]) => void;
    readonly onRemove: (index: number) => void;
    readonly onUpload: (files: FileList | null) => Promise<readonly string[]>;
    readonly referenceMode?: CustomVideoReferenceMode;
    readonly referenceModeOptions?: readonly CustomVideoReferenceMode[];
    readonly onReferenceModeChange: (value: CustomVideoReferenceMode | undefined) => void;
}) {
    const inputRef = useRef<HTMLInputElement>(null);
    const [url, setUrl] = useState("");
    const atLimit = sources.length >= maxCount;
    const isVideo = role === "input_video";

    const addUrl = () => {
        const source = url.trim();
        if (!source || atLimit) return;
        onAdd([source]);
        setUrl("");
    };

    const uploadFiles = async (files: FileList | null) => {
        if (atLimit) return;
        const next = await onUpload(files);
        onAdd(next);
    };

    return (
        <div id={`custom-video-role-${role}`} className="rounded-lg border border-stone-200 p-3 dark:border-stone-800">
            <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
                <div className="flex items-center gap-2">
                    <span className="text-sm font-semibold">{label}</span>
                    {referenceModeOptions ? (
                        <Select
                            className="min-w-28"
                            size="small"
                            value={referenceMode}
                            options={referenceModeOptions.map((value) => ({ value, label: referenceModeLabels[value] }))}
                            aria-label="参考图模式"
                            onChange={(value: CustomVideoReferenceMode) => onReferenceModeChange(customVideoReferenceModes.includes(value) ? value : undefined)}
                        />
                    ) : null}
                </div>
                <span className="text-xs text-stone-500 dark:text-stone-400">
                    已选 {sources.length} / {maxCount}
                </span>
            </div>
            <div className="hover-scrollbar flex min-h-20 gap-2 overflow-x-auto pb-2">
                {sources.map((source, index) => (
                    <MediaPreview key={`${role}-${source}-${index}`} source={source} label={`${label} ${index + 1}`} video={isVideo} onRemove={() => onRemove(index)} />
                ))}
                {!sources.length ? <div className="flex min-w-32 items-center text-xs text-stone-500 dark:text-stone-400">暂无{label}</div> : null}
            </div>
            <div className="mt-2 grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto_auto]">
                <Input
                    value={url}
                    size="small"
                    prefix={<Link className="size-3.5" />}
                    placeholder={isVideo ? "粘贴源视频 URL 或 Data URI" : "粘贴图片 URL 或 Data URI"}
                    disabled={atLimit}
                    onChange={(event) => setUrl(event.target.value)}
                    onPressEnter={addUrl}
                />
                <Button size="small" icon={<Plus className="size-3.5" />} disabled={atLimit || !url.trim()} onClick={addUrl}>
                    添加链接
                </Button>
                <Button size="small" icon={<Upload className="size-3.5" />} disabled={atLimit} onClick={() => inputRef.current?.click()}>
                    上传
                </Button>
            </div>
            <input
                ref={inputRef}
                type="file"
                className="hidden"
                accept={isVideo ? "video/*" : "image/*"}
                multiple={!isVideo && maxCount > 1}
                onChange={(event) => {
                    void uploadFiles(event.target.files);
                    event.target.value = "";
                }}
            />
        </div>
    );
}

function MediaPreview({ source, label, video, onRemove }: { readonly source: string; readonly label: string; readonly video: boolean; readonly onRemove: () => void }) {
    return (
        <div className={`group relative shrink-0 overflow-hidden rounded-md border border-stone-200 dark:border-stone-800 ${video ? "h-20 w-32 bg-black" : "size-20"}`}>
            {video ? <video src={source} className="size-full object-cover" muted preload="metadata" /> : <img src={source} alt={label} className="size-full object-cover" />}
            {video ? <VideoIcon className="absolute bottom-1 left-1 size-3.5 text-white drop-shadow" /> : null}
            <button type="button" className="absolute right-1 top-1 hidden size-6 items-center justify-center rounded bg-black/60 text-white group-hover:flex" onClick={onRemove} aria-label={`移除${label}`}>
                <Trash2 className="size-3.5" />
            </button>
        </div>
    );
}
