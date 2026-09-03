"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { App, Badge, Button, Empty, Input, Spin, Switch, Tabs, Tooltip, Typography, theme as antdTheme } from "antd";
import { ArrowDown, ArrowUp, Eye, FilePlus2, RefreshCw, Save, Trash2 } from "lucide-react";

import { FeatureGuideDialog } from "@/components/feature-guide/feature-guide-dialog";
import { MarkdownContent } from "@/components/feature-guide/markdown-content";
import { getAdminFeatureGuides, saveAdminFeatureGuide, type FeatureGuide, type FeatureGuideSurface } from "@/services/api/feature-guide";
import { useUserStore } from "@/stores/use-user-store";

const { TextArea } = Input;
const MAX_TITLE_LENGTH = 100;
const MAX_PAGES = 20;
const MAX_PAGE_LENGTH = 20_000;
const MAX_TOTAL_LENGTH = 100_000;

const surfaces: Array<{ surface: FeatureGuideSurface; label: string }> = [
    { surface: "canvas", label: "我的画布" },
    { surface: "image", label: "生图工作台" },
    { surface: "video", label: "视频创作台" },
];

type GuideRecord = Record<FeatureGuideSurface, FeatureGuide>;
type PageIndexRecord = Record<FeatureGuideSurface, number>;

function createGuide(surface: FeatureGuideSurface): FeatureGuide {
    return { surface, enabled: false, title: "", pages: [""], version: 1 };
}

function createGuideRecord(guides: FeatureGuide[] = []): GuideRecord {
    return surfaces.reduce((record, { surface }) => {
        const guide = guides.find((item) => item.surface === surface);
        record[surface] = guide ? { ...guide, surface, pages: [...guide.pages] } : createGuide(surface);
        return record;
    }, {} as GuideRecord);
}

function areGuidesEqual(left: FeatureGuide, right: FeatureGuide) {
    return left.enabled === right.enabled && left.title === right.title && left.version === right.version && left.pages.length === right.pages.length && left.pages.every((page, index) => page === right.pages[index]);
}

function codePointLength(value: string) {
    return Array.from(value).length;
}

function truncateCodePoints(value: string, limit: number) {
    return Array.from(value).slice(0, limit).join("");
}

function errorMessage(error: unknown, fallback: string) {
    return error instanceof Error && error.message ? error.message : fallback;
}

export default function AdminFeatureGuidesPage() {
    const { message } = App.useApp();
    const router = useRouter();
    const user = useUserStore((state) => state.user);
    const { token } = antdTheme.useToken();
    const isSuperAdmin = user?.role === "super_admin";
    const [drafts, setDrafts] = useState<GuideRecord>(() => createGuideRecord());
    const [baselines, setBaselines] = useState<GuideRecord>(() => createGuideRecord());
    const [surface, setSurface] = useState<FeatureGuideSurface>("canvas");
    const [pageIndexes, setPageIndexes] = useState<PageIndexRecord>({ canvas: 0, image: 0, video: 0 });
    const [loadState, setLoadState] = useState<"loading" | "error" | "ready">("loading");
    const [loadError, setLoadError] = useState("");
    const [savingSurface, setSavingSurface] = useState<FeatureGuideSurface | null>(null);
    const [previewOpen, setPreviewOpen] = useState(false);
    const loadRequestRef = useRef(0);

    const load = useCallback(async () => {
        const request = ++loadRequestRef.current;
        setLoadState("loading");
        setLoadError("");
        try {
            const next = createGuideRecord(await getAdminFeatureGuides());
            if (request !== loadRequestRef.current) return;
            setDrafts(next);
            setBaselines(createGuideRecord(Object.values(next)));
            setLoadState("ready");
        } catch (error) {
            if (request !== loadRequestRef.current) return;
            const nextError = errorMessage(error, "获取功能引导失败");
            setLoadError(nextError);
            setLoadState("error");
            message.error(nextError);
        }
    }, [message]);

    useEffect(() => {
        if (user && !isSuperAdmin) router.replace("/admin");
    }, [isSuperAdmin, router, user]);

    useEffect(() => {
        if (!isSuperAdmin) {
            loadRequestRef.current += 1;
            return;
        }
        void load();
        return () => {
            loadRequestRef.current += 1;
        };
    }, [isSuperAdmin, load]);

    const dirtySurfaces = useMemo(
        () => new Set(surfaces.filter(({ surface: item }) => !areGuidesEqual(drafts[item], baselines[item])).map(({ surface: item }) => item)),
        [baselines, drafts],
    );

    useEffect(() => {
        if (dirtySurfaces.size === 0) return;
        if (typeof window === "undefined") return;
        const beforeUnload = (event: BeforeUnloadEvent) => {
            event.preventDefault();
            event.returnValue = "";
        };
        window.addEventListener("beforeunload", beforeUnload);
        return () => window.removeEventListener("beforeunload", beforeUnload);
    }, [dirtySurfaces]);

    const guide = drafts[surface];
    const requestedPageIndex = pageIndexes[surface];
    const pageIndex = Math.min(requestedPageIndex, Math.max(0, guide.pages.length - 1));
    const page = guide.pages[pageIndex];
    const totalLength = guide.pages.reduce((total, item) => total + codePointLength(item), 0);
    const otherPagesLength = totalLength - codePointLength(page ?? "");
    const pageLengthLimit = Math.min(MAX_PAGE_LENGTH, Math.max(0, MAX_TOTAL_LENGTH - otherPagesLength));
    const disabled = savingSurface === surface;

    const updateGuide = (updater: (current: FeatureGuide) => FeatureGuide) => {
        setDrafts((current) => ({ ...current, [surface]: updater(current[surface]) }));
    };

    const updatePage = (value: string) => {
        const nextPage = truncateCodePoints(value, pageLengthLimit);
        updateGuide((current) => ({ ...current, pages: current.pages.map((item, index) => (index === pageIndex ? nextPage : item)) }));
    };

    const addPage = () => {
        if (guide.pages.length >= MAX_PAGES) return;
        updateGuide((current) => ({ ...current, pages: [...current.pages, ""] }));
        setPageIndexes((current) => ({ ...current, [surface]: guide.pages.length }));
    };

    const movePage = (direction: -1 | 1) => {
        const target = pageIndex + direction;
        if (target < 0 || target >= guide.pages.length) return;
        updateGuide((current) => {
            const pages = [...current.pages];
            [pages[pageIndex], pages[target]] = [pages[target], pages[pageIndex]];
            return { ...current, pages };
        });
        setPageIndexes((current) => ({ ...current, [surface]: target }));
    };

    const deletePage = () => {
        updateGuide((current) => ({ ...current, pages: current.pages.filter((_, index) => index !== pageIndex) }));
        setPageIndexes((current) => ({ ...current, [surface]: Math.max(0, Math.min(pageIndex, guide.pages.length - 2)) }));
    };

    const save = async () => {
        if (guide.enabled && (guide.pages.length === 0 || guide.pages.some((item) => !item.trim()))) {
            message.warning("启用前请填写全部引导页面");
            return;
        }
        if (totalLength > MAX_TOTAL_LENGTH) {
            message.warning("页面总字数不能超过 100000");
            return;
        }
        setSavingSurface(surface);
        try {
            const saved = await saveAdminFeatureGuide(surface, { enabled: guide.enabled, title: guide.title, pages: guide.pages });
            setDrafts((current) => ({ ...current, [surface]: saved }));
            setBaselines((current) => ({ ...current, [surface]: { ...saved, pages: [...saved.pages] } }));
            message.success("功能引导已保存");
        } catch (error) {
            message.error(errorMessage(error, "保存功能引导失败"));
        } finally {
            setSavingSurface(null);
        }
    };

    if (!isSuperAdmin) return null;
    if (loadState === "loading") return <div className="flex h-64 items-center justify-center"><Spin size="large" /></div>;
    if (loadState === "error") {
        return (
            <div className="flex h-64 items-center justify-center">
                <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={<div className="space-y-1"><div>功能引导加载失败</div><Typography.Text type="secondary">{loadError}</Typography.Text></div>}>
                    <Button icon={<RefreshCw className="size-4" />} onClick={() => void load()}>重新加载</Button>
                </Empty>
            </div>
        );
    }

    return (
        <div className="mx-auto max-w-[1500px]">
            <div className="mb-5 flex flex-wrap items-end justify-between gap-3">
                <div>
                    <Typography.Title level={4} className="!mb-1">功能引导</Typography.Title>
                    <Typography.Text type="secondary">维护各工作台首次使用时展示的多页说明。</Typography.Text>
                </div>
                <div className="flex items-center gap-2">
                    <Typography.Text type="secondary" className="text-xs">版本 {guide.version}</Typography.Text>
                    <Button icon={<Eye className="size-4" />} onClick={() => setPreviewOpen(true)}>预览</Button>
                    <Button type="primary" icon={<Save className="size-4" />} loading={disabled} onClick={() => void save()}>保存</Button>
                </div>
            </div>

            <Tabs
                activeKey={surface}
                onChange={(key) => {
                    if (!savingSurface) setSurface(key as FeatureGuideSurface);
                }}
                items={surfaces.map(({ surface: item, label }) => ({
                    key: item,
                    label: <Badge dot={dirtySurfaces.has(item)} offset={[4, 0]}>{label}</Badge>,
                    children: <div />,
                }))}
            />

            <div className="grid gap-6 lg:grid-cols-[220px_minmax(0,1fr)]">
                <aside className="min-w-0 border-r pr-4 max-lg:border-r-0 max-lg:border-b max-lg:pb-4" style={{ borderColor: token.colorBorderSecondary }}>
                    <div className="mb-3 flex items-center justify-between gap-2">
                        <Typography.Text strong>引导页面</Typography.Text>
                        <Tooltip title="新增页面"><Button type="text" size="small" icon={<FilePlus2 className="size-4" />} disabled={disabled || guide.pages.length >= MAX_PAGES} onClick={addPage} /></Tooltip>
                    </div>
                    <div className="flex gap-2 overflow-x-auto pb-1 lg:flex-col lg:overflow-y-auto">
                        {guide.pages.map((_, index) => (
                            <Button key={index} type={index === pageIndex ? "primary" : "text"} className="shrink-0 !justify-start" onClick={() => setPageIndexes((current) => ({ ...current, [surface]: index }))}>
                                第 {index + 1} 页
                            </Button>
                        ))}
                    </div>
                    <Typography.Text type="secondary" className="mt-3 block text-xs">{guide.pages.length} / {MAX_PAGES} 页</Typography.Text>
                </aside>

                <div className="min-w-0 space-y-5">
                    <div className="grid gap-4 md:grid-cols-[minmax(0,1fr)_auto] md:items-center">
                        <div>
                            <div className="flex items-center justify-between gap-3"><Typography.Text strong>总标题</Typography.Text><Typography.Text type="secondary" className="text-xs">{codePointLength(guide.title)} / {MAX_TITLE_LENGTH}</Typography.Text></div>
                            <Input className="mt-2" value={guide.title} placeholder="请输入引导标题" disabled={disabled} onChange={(event) => updateGuide((current) => ({ ...current, title: truncateCodePoints(event.target.value, MAX_TITLE_LENGTH) }))} />
                        </div>
                        <div className="flex items-center justify-between gap-3 md:pt-6">
                            <Typography.Text type="secondary">启用引导</Typography.Text>
                            <Switch checked={guide.enabled} disabled={disabled} onChange={(enabled) => updateGuide((current) => ({ ...current, enabled }))} />
                        </div>
                    </div>

                    {page === undefined ? (
                        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂未创建引导页面"><Button type="primary" icon={<FilePlus2 className="size-4" />} onClick={addPage}>新建页面</Button></Empty>
                    ) : (
                        <>
                            <div className="flex flex-wrap items-center justify-between gap-2">
                                <div className="flex items-center gap-1">
                                    <Typography.Text strong>第 {pageIndex + 1} 页</Typography.Text>
                                    <Tooltip title="上移"><Button type="text" size="small" icon={<ArrowUp className="size-4" />} disabled={disabled || pageIndex === 0} onClick={() => movePage(-1)} /></Tooltip>
                                    <Tooltip title="下移"><Button type="text" size="small" icon={<ArrowDown className="size-4" />} disabled={disabled || pageIndex === guide.pages.length - 1} onClick={() => movePage(1)} /></Tooltip>
                                    <Tooltip title="删除页面"><Button type="text" danger size="small" icon={<Trash2 className="size-4" />} disabled={disabled} onClick={deletePage} /></Tooltip>
                                </div>
                                <Typography.Text type="secondary" className="text-xs">{totalLength.toLocaleString()} / {MAX_TOTAL_LENGTH.toLocaleString()} 字</Typography.Text>
                            </div>
                            <div className="grid gap-4 xl:grid-cols-2">
                                <div className="min-w-0">
                                    <div className="mb-2 flex items-center justify-between gap-3"><Typography.Text type="secondary" className="text-xs">Markdown</Typography.Text><Typography.Text type="secondary" className="text-xs">{codePointLength(page)} / {pageLengthLimit}</Typography.Text></div>
                                    <TextArea value={page} rows={18} disabled={disabled} placeholder="请输入页面正文" onChange={(event) => updatePage(event.target.value)} />
                                </div>
                                <div className="min-w-0">
                                    <Typography.Text type="secondary" className="mb-2 block text-xs">实时预览</Typography.Text>
                                    <div className="min-h-[390px] overflow-auto border p-4" style={{ borderColor: token.colorBorderSecondary }}>
                                        {page.trim() ? <MarkdownContent content={page} /> : <Typography.Text type="secondary">页面内容将在这里显示。</Typography.Text>}
                                    </div>
                                </div>
                            </div>
                        </>
                    )}
                </div>
            </div>

            <FeatureGuideDialog open={previewOpen} guide={guide} mode="preview" onClose={() => setPreviewOpen(false)} />
        </div>
    );
}
