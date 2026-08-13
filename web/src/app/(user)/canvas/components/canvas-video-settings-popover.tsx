"use client";

import { useEffect, useRef, useState, type RefObject } from "react";
import { createPortal } from "react-dom";
import { Settings2 } from "lucide-react";
import { Button } from "antd";

import { VideoSettingsPanel, videoResolutionLabel, videoSecondsLabel, videoSizeLabel } from "@/components/video-settings-panel";
import { canvasThemes } from "@/lib/canvas-theme";
import { customVideoConfigForModel, normalizeVideoDurationForModel, type AiConfig } from "@/stores/use-config-store";
import type { CustomVideoMediaFeature } from "@/lib/custom-video-config";
import type { CustomVideoRuntimeSnapshot } from "@/lib/custom-video-runtime";
import { useThemeStore } from "@/stores/use-theme-store";
import { CanvasCustomVideoReferenceInputs } from "./canvas-custom-video-reference-inputs";
import { canvasCustomVideoRuntimeForModel } from "./canvas-custom-video-runtime";

type CanvasVideoSettingsPopoverProps = {
    config: AiConfig;
    model?: string;
    onConfigChange: (key: keyof AiConfig, value: string) => void;
    customVideoRuntime?: CustomVideoRuntimeSnapshot;
    onCustomVideoRuntimeChange?: (runtime: CustomVideoRuntimeSnapshot) => void;
    buttonClassName?: string;
    placement?: "topLeft" | "top" | "topRight" | "bottomLeft" | "bottom" | "bottomRight";
};

export function CanvasVideoSettingsPopover({ config, model, onConfigChange, customVideoRuntime, onCustomVideoRuntimeChange, buttonClassName, placement = "topLeft" }: CanvasVideoSettingsPopoverProps) {
    const theme = canvasThemes[useThemeStore((state) => state.theme)];
    const displaySeconds = normalizeVideoDurationForModel(config, model || config.model || config.videoModel, config.videoSeconds);
    const buttonRef = useRef<HTMLSpanElement>(null);
    const panelRef = useRef<HTMLDivElement>(null);
    const [open, setOpen] = useState(false);
    const [buttonRect, setButtonRect] = useState<DOMRect | null>(null);

    useEffect(() => {
        if (!open) return;
        const syncPosition = () => setButtonRect(buttonRef.current?.getBoundingClientRect() || null);
        const closeOnOutsidePointer = (event: PointerEvent) => {
            const target = event.target;
            if (!(target instanceof Node)) return;
            if (buttonRef.current?.contains(target) || panelRef.current?.contains(target)) return;
            setOpen(false);
        };

        syncPosition();
        window.addEventListener("resize", syncPosition);
        window.addEventListener("scroll", syncPosition, true);
        window.addEventListener("pointerdown", closeOnOutsidePointer, true);
        return () => {
            window.removeEventListener("resize", syncPosition);
            window.removeEventListener("scroll", syncPosition, true);
            window.removeEventListener("pointerdown", closeOnOutsidePointer, true);
        };
    }, [open]);

    const panel =
        open && buttonRect ? (
            <VideoSettingsPortal
                buttonRect={buttonRect}
                panelRef={panelRef}
                placement={placement}
                theme={theme}
                config={config}
                model={model}
                onConfigChange={onConfigChange}
                customVideoRuntime={customVideoRuntime}
                onCustomVideoRuntimeChange={onCustomVideoRuntimeChange}
            />
        ) : null;

    return (
        <>
            <span ref={buttonRef} className="inline-flex min-w-0">
                <Button
                    size="small"
                    type="text"
                    className={buttonClassName || "!h-8 !max-w-[170px] !justify-start !rounded-full !px-2.5"}
                    style={{ background: theme.node.fill, color: theme.node.text }}
                    icon={<Settings2 className="size-3.5" />}
                    onClick={() => setOpen((current) => !current)}
                >
                    <span className="truncate">
                        {videoResolutionLabel(config.vquality)} · {videoSizeLabel(config.size)} · {videoSecondsLabel(displaySeconds)}
                    </span>
                </Button>
            </span>
            {panel}
        </>
    );
}

function VideoSettingsPortal({
    buttonRect,
    panelRef,
    placement,
    theme,
    config,
    model,
    onConfigChange,
    customVideoRuntime,
    onCustomVideoRuntimeChange,
}: {
    buttonRect: DOMRect;
    panelRef: RefObject<HTMLDivElement | null>;
    placement: CanvasVideoSettingsPopoverProps["placement"];
    theme: (typeof canvasThemes)[keyof typeof canvasThemes];
    config: AiConfig;
    model?: string;
    onConfigChange: (key: keyof AiConfig, value: string) => void;
    customVideoRuntime?: CustomVideoRuntimeSnapshot;
    onCustomVideoRuntimeChange?: (runtime: CustomVideoRuntimeSnapshot) => void;
}) {
    const selectedModel = model || config.videoModel || config.model;
    const customConfig = customVideoConfigForModel(config, selectedModel);
    const normalizedRuntime = canvasCustomVideoRuntimeForModel(config, selectedModel, customVideoRuntime);
    const [focusRole, setFocusRole] = useState<CustomVideoMediaFeature>();
    const width = 356;
    const gap = 8;
    const margin = 12;
    const alignRight = placement?.endsWith("Right");
    const alignCenter = placement === "top" || placement === "bottom";
    const left = alignCenter ? buttonRect.left + buttonRect.width / 2 - width / 2 : alignRight ? buttonRect.right - width : buttonRect.left;
    const topPlacement = placement?.startsWith("top");
    const style = {
        position: "fixed",
        zIndex: 1200,
        width,
        left: Math.max(margin, Math.min(window.innerWidth - width - margin, left)),
        ...(topPlacement ? { bottom: window.innerHeight - buttonRect.top + gap, maxHeight: Math.max(260, buttonRect.top - margin * 2) } : { top: buttonRect.bottom + gap, maxHeight: Math.max(260, window.innerHeight - buttonRect.bottom - margin * 2) }),
        background: theme.toolbar.panel,
        borderRadius: 18,
        boxShadow: "0 18px 54px rgba(28, 25, 23, 0.16)",
        padding: 18,
        overflowY: "auto",
        color: theme.node.text,
    } as const;

    return createPortal(
        <div ref={panelRef} className="canvas-image-settings-popover" style={style} onPointerDown={(event) => event.stopPropagation()} onMouseDown={(event) => event.stopPropagation()} onClick={(event) => event.stopPropagation()}>
            <VideoSettingsPanel
                config={config}
                model={model}
                onConfigChange={(key, value) => onConfigChange(key, value)}
                theme={theme}
                className="space-y-4"
                customVideoRuntime={normalizedRuntime}
                onCustomVideoRuntimeChange={onCustomVideoRuntimeChange}
                onCustomVideoMediaRoleOpen={setFocusRole}
            />
            {customConfig && normalizedRuntime && onCustomVideoRuntimeChange ? <CanvasCustomVideoReferenceInputs config={customConfig} runtime={normalizedRuntime} theme={theme} focusRole={focusRole} onChange={onCustomVideoRuntimeChange} /> : null}
        </div>,
        document.body,
    );
}
