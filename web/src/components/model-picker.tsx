"use client";

import { useEffect, useId, useMemo, useState } from "react";
import { SwapOutlined } from "@ant-design/icons";
import { Cpu } from "lucide-react";

import { Select, SelectContent, SelectItem, SelectTrigger } from "@/components/ui/select";
import { cn } from "@/lib/utils";
import { channelModelOptionsByCapability, hasUsableAutoChannel, isMergeModelValue, modelOptionLabel, modelOptionName, selectableModelsByCapability, selectedChannelId, useConfigStore, type AiConfig, type ModelCapability } from "@/stores/use-config-store";

type ModelPickerProps = {
    config: AiConfig;
    value?: string;
    onChange: (model: string) => void;
    capability?: ModelCapability;
    className?: string;
    fullWidth?: boolean;
    placeholder?: string;
    onMissingConfig?: () => void;
};

export function ModelPicker({ config, value, onChange, capability, className, fullWidth = false, placeholder = "选择模型", onMissingConfig }: ModelPickerProps) {
    const pickerId = useId();
    const [open, setOpen] = useState(false);
    const options = useMemo(() => Array.from(new Set([...(capability ? [] : [value]), ...selectableModelsByCapability(config, capability)].filter((model): model is string => Boolean(model)))), [capability, config, value]);
    const current = value || "";
    const channels = useConfigStore((state) => state.serverChannels);
    const channelModels = useConfigStore((state) => state.serverChannelModels);
    const autoChannelModels = useConfigStore((state) => state.autoChannelModels);
    const serverPricing = useConfigStore((state) => state.serverPricing);
    const serverMetrics = useConfigStore((state) => state.serverMetrics);
    const serverCatalogLoading = useConfigStore((state) => state.serverCatalogLoading);
    const serverCatalogError = useConfigStore((state) => state.serverCatalogError);
    const selectCapabilityChannel = useConfigStore((state) => state.selectCapabilityChannel);

    useEffect(() => {
        const closeOtherPicker = (event: Event) => {
            if ((event as CustomEvent<string>).detail !== pickerId) setOpen(false);
        };
        window.addEventListener("model-picker-open", closeOtherPicker);
        return () => window.removeEventListener("model-picker-open", closeOtherPicker);
    }, [pickerId]);

    const picker = (
        <Select
            open={open}
            value={current}
            onOpenChange={(nextOpen) => {
                if (nextOpen && !options.length) onMissingConfig?.();
                if (nextOpen) window.dispatchEvent(new CustomEvent("model-picker-open", { detail: pickerId }));
                setOpen(nextOpen);
            }}
            onValueChange={onChange}
        >
            <SelectTrigger
                className={cn(
                    "canvas-composer-model-picker h-8 w-fit max-w-full gap-2 rounded-full border border-input bg-transparent px-3 text-sm font-normal shadow-sm transition-colors",
                    fullWidth ? "w-full min-w-[8rem] justify-start" : "min-w-[13rem] justify-start",
                    "data-[state=open]:border-ring data-[state=open]:ring-2 data-[state=open]:ring-ring/20",
                    className,
                )}
                onMouseDown={(event) => event.stopPropagation()}
                onPointerDown={(event) => event.stopPropagation()}
                title={current ? modelOptionLabel(config, current) : placeholder}
            >
                <ModelIcon model={current} />
                <span className="canvas-model-picker-text min-w-0 flex-1 truncate text-left">{current ? modelOptionLabel(config, current) : placeholder}</span>
            </SelectTrigger>
            <SelectContent
                data-canvas-no-zoom
                className="z-[1200] w-96 max-w-[calc(100vw-24px)] rounded-xl border border-border/70 bg-popover p-1 shadow-xl"
                position="popper"
                align="start"
                side="bottom"
                sideOffset={6}
                onPointerDown={(event) => event.stopPropagation()}
                onMouseDown={(event) => event.stopPropagation()}
            >
                {options.length ? (
                    options.map((model) => (
                        <SelectItem key={model} value={model} textValue={modelOptionLabel(config, model)}>
                            <ModelLabel config={config} model={model} capability={capability} />
                        </SelectItem>
                    ))
                ) : (
                    <SelectItem value="__empty__" disabled>
                        {serverCatalogLoading ? "正在加载模型…" : serverCatalogError ? "模型列表加载失败，请稍后重试" : emptyModelLabel(config, capability)}
                    </SelectItem>
                )}
            </SelectContent>
        </Select>
    );

    if (!capability) return picker;
    const channelId = selectedChannelId(config, capability);
    const hasAutoOptions = hasUsableAutoChannel(capability, { serverChannels: channels, serverChannelModels: channelModels, autoChannelModels, serverPricing, serverMetrics });
    return (
        <div className={cn("flex min-w-0 gap-2", fullWidth && "w-full")}>
            <Select
                value={channelId != null ? String(channelId) : ""}
                onValueChange={(value) => {
                    const parsed = Number(value);
                    selectCapabilityChannel(capability, value.trim() && Number.isInteger(parsed) && parsed >= 0 ? parsed : null);
                }}
            >
                <SelectTrigger
                    className="h-8 min-w-[7rem] max-w-[10rem] justify-start rounded-full border border-input bg-transparent px-3 text-sm shadow-sm"
                    title={channelId === 0 ? "Auto（自动路由）" : channels.find((channel) => channel.id === channelId)?.name || "选择渠道"}
                >
                    <span className="truncate">{channelId === 0 ? "Auto（自动路由）" : channels.find((channel) => channel.id === channelId)?.name || "选择渠道"}</span>
                </SelectTrigger>
                <SelectContent className="z-[1200] max-w-[calc(100vw-24px)]" position="popper" align="start" side="bottom" sideOffset={6}>
                    {channels.length || hasAutoOptions ? (
                        <>
                            {hasAutoOptions ? (
                                <SelectItem key="auto" value="0">
                                    <span className="flex items-center gap-2">
                                        <SwapOutlined style={{ color: "#1677ff" }} />
                                        <span>Auto（自动路由）</span>
                                    </span>
                                </SelectItem>
                            ) : null}
                            {channels.map((channel) => (
                                <SelectItem key={channel.id} value={String(channel.id)}>
                                    {channel.name}
                                </SelectItem>
                            ))}
                        </>
                    ) : (
                        <SelectItem value="__empty_channel__" disabled>
                            暂无可用渠道
                        </SelectItem>
                    )}
                </SelectContent>
            </Select>
            <div className={cn("min-w-0", fullWidth && "flex-1")}>{picker}</div>
        </div>
    );
}

function emptyModelLabel(config: AiConfig, capability?: ModelCapability) {
    const label = capability === "image" ? "生图" : capability === "video" ? "视频" : capability === "text" ? "文本" : capability === "audio" ? "音频" : "";
    if (capability && config.models.length) return "请先在上方配置可选模型";
    return config.models.length ? `暂无匹配的${label}模型` : "暂无可用模型，请联系管理员配置";
}

function ModelLabel({ config, model, capability }: { config: AiConfig; model: string; capability?: ModelCapability }) {
    const isMerge = isMergeModelValue(model);
    const option = capability && !isMerge ? channelModelOptionsByCapability(capability).find((item) => item.value === model) : null;
    return (
        <span className="flex min-w-0 items-center gap-2" data-channel-model-id={option?.channelModelId}>
            <ModelIcon model={model} />
            <span className="min-w-0 flex-1 truncate">{modelOptionLabel(config, model)}</span>
            {isMerge ? (
                <span className="shrink-0 rounded border border-border/50 px-1.5 py-0.5 text-[10px] font-medium leading-none text-muted-foreground">合并</span>
            ) : option && option.successRate !== null ? (
                <span className="shrink-0 text-xs font-medium" style={{ color: `hsl(${option.successRate * 1.2}, 80%, 45%)` }}>
                    {option.successRate}%
                </span>
            ) : option ? (
                <span className="shrink-0 text-xs opacity-55">{rateUnavailableLabel(option.metricsStatus)}</span>
            ) : null}
        </span>
    );
}

function rateUnavailableLabel(status: string) {
    if (status === "stale") return "已过期";
    if (status === "error") return "获取失败";
    if (status === "unmapped") return "未映射";
    return "暂无数据";
}

function ModelIcon({ model }: { model: string }) {
    const icon = resolveModelIcon(modelOptionName(model));
    return icon ? <img src={icon} alt="" className="size-4 shrink-0 dark:invert" /> : <Cpu className="size-4 shrink-0 opacity-70" />;
}

function resolveModelIcon(model: string) {
    const name = model.toLowerCase();
    if (name.includes("claude") || name.includes("anthropic")) return "/icons/claude.svg";
    if (name.includes("gemini") || name.includes("google")) return "/icons/gemini.svg";
    if (name.includes("gpt") || name.includes("openai")) return "/icons/openai.svg";
    if (name.includes("grok") || name.includes("grok")) return "/icons/grok.svg";
    if (name.includes("deepseek") || name.includes("deepseek")) return "/icons/deepseek.svg";
    if (name.includes("glm") || name.includes("glm")) return "/icons/glm.svg";
    return "";
}
