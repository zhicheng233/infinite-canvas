"use client";

import { lazy, Suspense, useEffect, useRef, useState } from "react";
import { usePathname } from "next/navigation";
import { Button, Checkbox, Modal, Typography } from "antd";
import { Megaphone } from "lucide-react";

import { getAnnouncement, type SiteAnnouncement } from "@/services/api/announcement";
import { getFeatureGuide, type FeatureGuide } from "@/services/api/feature-guide";
import { useUserStore } from "@/stores/use-user-store";
import { getFeatureGuideCompletionKey, getFeatureGuideSurface, shouldPresentFeatureGuide } from "./feature-guide-state";

const STORAGE_KEY = "infinite-canvas:announcement:dismissed_version";
const FeatureGuideDialog = lazy(() => import("@/components/feature-guide/feature-guide-dialog").then((module) => ({ default: module.FeatureGuideDialog })));

type AnnouncementPhase = "loading" | "open" | "closing" | "done";
type PendingGuide = { guide: FeatureGuide; userId: string };

function readStorage(key: string) {
    try {
        return localStorage.getItem(key);
    } catch {
        return null;
    }
}

export function GlobalAnnouncementDialog() {
    const pathname = usePathname();
    const surface = getFeatureGuideSurface(pathname);
    const userId = useUserStore((state) => state.user?.id ?? null);
    const [announcement, setAnnouncement] = useState<SiteAnnouncement | null>(null);
    const [announcementPhase, setAnnouncementPhase] = useState<AnnouncementPhase>("loading");
    const [dismissUntilUpdate, setDismissUntilUpdate] = useState(true);
    const [pendingGuide, setPendingGuide] = useState<PendingGuide | null>(null);
    const guideRequestId = useRef(0);
    const completedGuideVersions = useRef(new Map<string, string>());
    const guide = pendingGuide?.userId === userId && pendingGuide.guide.surface === surface ? pendingGuide.guide : null;

    useEffect(() => {
        let cancelled = false;
        const load = async () => {
            try {
                const item = await getAnnouncement();
                if (cancelled) return;
                if (!item?.enabled || !item.content?.trim()) {
                    setAnnouncementPhase("done");
                    return;
                }
                const version = String(item.version || 1);
                if (readStorage(STORAGE_KEY) === version) {
                    setAnnouncementPhase("done");
                    return;
                }
                setAnnouncement(item);
                setAnnouncementPhase("open");
            } catch {
                // Announcement failures should never block app entry.
                if (!cancelled) setAnnouncementPhase("done");
            }
        };
        void load();
        return () => {
            cancelled = true;
        };
    }, []);

    useEffect(() => {
        const requestId = ++guideRequestId.current;
        setPendingGuide(null);
        if (!userId || !surface) return;

        const load = async () => {
            try {
                const item = await getFeatureGuide(surface);
                if (guideRequestId.current !== requestId) return;
                if (!item || item.surface !== surface) return;
                const completionKey = getFeatureGuideCompletionKey(userId, surface);
                const version = String(item.version);
                const completedVersion = completedGuideVersions.current.get(completionKey) === version ? version : readStorage(completionKey);
                if (shouldPresentFeatureGuide(item, completedVersion)) setPendingGuide({ guide: item, userId });
            } catch {
                // Guide failures should never block app entry.
            }
        };
        void load();
        return () => {
            if (guideRequestId.current === requestId) guideRequestId.current += 1;
        };
    }, [surface, userId]);

    const closeAnnouncement = () => {
        try {
            if (announcement && dismissUntilUpdate) {
                localStorage.setItem(STORAGE_KEY, String(announcement.version || 1));
            }
        } catch {
            // Storage can be unavailable in restricted browser contexts.
        } finally {
            setAnnouncementPhase("closing");
        }
    };

    const completeGuide = () => {
        if (!guide || !userId) return;
        const completionKey = getFeatureGuideCompletionKey(userId, guide.surface);
        const version = String(guide.version);
        completedGuideVersions.current.set(completionKey, version);
        try {
            localStorage.setItem(completionKey, version);
        } catch {
            // Completion still applies to the current session.
        } finally {
            setPendingGuide(null);
        }
    };

    return (
        <>
            {announcement ? (
                <Modal
                    open={announcementPhase === "open"}
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
                    onCancel={closeAnnouncement}
                    afterOpenChange={(open) => {
                        if (!open) setAnnouncementPhase((phase) => (phase === "closing" ? "done" : phase));
                    }}
                    footer={
                        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                            <Checkbox checked={dismissUntilUpdate} onChange={(event) => setDismissUntilUpdate(event.target.checked)}>
                                本次更新前不再弹出
                            </Checkbox>
                            <Button type="primary" onClick={closeAnnouncement}>
                                我知道了
                            </Button>
                        </div>
                    }
                >
                    <Typography.Paragraph className="whitespace-pre-wrap !text-sm !leading-6">{announcement.content}</Typography.Paragraph>
                </Modal>
            ) : null}
            {announcementPhase === "done" && guide ? (
                <Suspense fallback={null}>
                    <FeatureGuideDialog open guide={guide} onComplete={completeGuide} />
                </Suspense>
            ) : null}
        </>
    );
}
