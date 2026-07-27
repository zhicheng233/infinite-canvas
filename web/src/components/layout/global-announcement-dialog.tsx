"use client";

import { useEffect, useState } from "react";
import { Button, Checkbox, Modal, Typography } from "antd";
import { Megaphone } from "lucide-react";

import { getAnnouncement, type SiteAnnouncement } from "@/services/api/announcement";

const STORAGE_KEY = "infinite-canvas:announcement:dismissed_version";

export function GlobalAnnouncementDialog() {
    const [announcement, setAnnouncement] = useState<SiteAnnouncement | null>(null);
    const [open, setOpen] = useState(false);
    const [dismissUntilUpdate, setDismissUntilUpdate] = useState(true);

    useEffect(() => {
        let cancelled = false;
        const load = async () => {
            try {
                const item = await getAnnouncement();
                if (cancelled || !item?.enabled || !item.content?.trim()) return;
                const version = String(item.version || 1);
                if (localStorage.getItem(STORAGE_KEY) === version) return;
                setAnnouncement(item);
                setOpen(true);
            } catch {
                // Announcement failures should never block app entry.
            }
        };
        void load();
        return () => {
            cancelled = true;
        };
    }, []);

    const close = () => {
        if (announcement && dismissUntilUpdate) {
            localStorage.setItem(STORAGE_KEY, String(announcement.version || 1));
        }
        setOpen(false);
    };

    if (!announcement) return null;

    return (
        <Modal
            open={open}
            centered
            width={520}
            title={
                <div className="flex items-center gap-2">
                    <span className="grid size-8 place-items-center rounded-lg bg-blue-50 text-blue-600 dark:bg-blue-950/50 dark:text-blue-300">
                        <Megaphone className="size-4" />
                    </span>
                    <span>{announcement.title || "公告"}</span>
                </div>
            }
            onCancel={close}
            footer={
                <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                    <Checkbox checked={dismissUntilUpdate} onChange={(event) => setDismissUntilUpdate(event.target.checked)}>
                        本次更新前不再弹出
                    </Checkbox>
                    <Button type="primary" onClick={close}>
                        我知道了
                    </Button>
                </div>
            }
        >
            <Typography.Paragraph className="whitespace-pre-wrap !text-sm !leading-6">{announcement.content}</Typography.Paragraph>
        </Modal>
    );
}
