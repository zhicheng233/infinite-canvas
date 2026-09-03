"use client";

import { useEffect, useState } from "react";
import { Button, Modal } from "antd";
import { BookOpen } from "lucide-react";

import type { FeatureGuide } from "@/services/api/feature-guide";
import { MarkdownContent } from "./markdown-content";

export type FeatureGuideDialogProps = {
    open: boolean;
    guide: FeatureGuide;
    mode?: "required" | "preview";
    onComplete?: () => void | Promise<void>;
    onClose?: () => void;
};

export function FeatureGuideDialog({ open, guide, mode = "required", onComplete, onClose }: FeatureGuideDialogProps) {
    const [pageIndex, setPageIndex] = useState(0);
    const [completing, setCompleting] = useState(false);
    const pages = guide.pages.length ? guide.pages : [""];
    const currentPage = Math.min(pageIndex, pages.length - 1);
    const lastPage = currentPage === pages.length - 1;
    const preview = mode === "preview";
    const pageContent = pages[currentPage] ?? "";

    useEffect(() => {
        setPageIndex(0);
        setCompleting(false);
    }, [open, guide.surface, guide.version]);

    const finish = async () => {
        if (preview && !onComplete) {
            onClose?.();
            return;
        }
        setCompleting(true);
        try {
            await onComplete?.();
        } finally {
            setCompleting(false);
        }
    };

    return (
        <Modal
            open={open}
            centered
            width={760}
            title={
                <div className="flex items-center gap-2.5">
                    <span className="grid size-8 place-items-center rounded-md bg-muted text-foreground">
                        <BookOpen className="size-4" />
                    </span>
                    <span>{guide.title || "功能引导"}</span>
                </div>
            }
            closable={preview}
            maskClosable={preview}
            keyboard={preview}
            onCancel={preview ? onClose : undefined}
            footer={
                <div className="grid grid-cols-[1fr_auto_1fr] items-center gap-3">
                    <div className="justify-self-start">
                        {currentPage > 0 ? <Button onClick={() => setPageIndex(currentPage - 1)}>上一页</Button> : null}
                    </div>
                    <span className="text-xs tabular-nums text-muted-foreground">
                        {currentPage + 1} / {pages.length}
                    </span>
                    <Button className="justify-self-end" type="primary" loading={completing} onClick={lastPage ? () => void finish() : () => setPageIndex(currentPage + 1)}>
                        {lastPage ? (preview && !onComplete ? "关闭" : "完成") : "下一页"}
                    </Button>
                </div>
            }
        >
            <div className="thin-scrollbar max-h-[min(62vh,640px)] min-h-56 overflow-y-auto pr-1">
                {pageContent.trim() ? <MarkdownContent content={pageContent} /> : <div className="grid min-h-56 place-items-center text-sm text-muted-foreground">暂无引导内容</div>}
            </div>
        </Modal>
    );
}
