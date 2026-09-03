import { afterEach, describe, expect, it, jest, mock } from "bun:test";
import { createElement, type ReactNode } from "react";
import type { ReactTestRenderer } from "react-test-renderer";
import { act, create } from "react-test-renderer";

import type { FeatureGuide } from "@/services/api/feature-guide";

Object.defineProperty(globalThis, "IS_REACT_ACT_ENVIRONMENT", { configurable: true, value: true });

type Props = Readonly<Record<string, unknown>> & { readonly children?: ReactNode };
const message = { error: jest.fn(), success: jest.fn(), warning: jest.fn() };
const getAdminFeatureGuides = jest.fn<() => Promise<FeatureGuide[]>>();
const saveAdminFeatureGuide = jest.fn();
let currentRole = "super_admin";

function deferred<T>() {
    let resolve: (value: T) => void = () => undefined;
    const promise = new Promise<T>((resolvePromise) => {
        resolve = resolvePromise;
    });
    return { promise, resolve };
}

function MockComponent({ children, ...props }: Props) {
    return createElement("guide-component", props, children);
}

function MockButton({ children, ...props }: Props) {
    return createElement("guide-button", props, children);
}

function MockInput(props: Props) {
    return createElement("guide-input", props);
}

function MockTextArea(props: Props) {
    return createElement("guide-textarea", props);
}

function MockTabs({ items = [], activeKey, ...props }: Props & { readonly items?: Array<{ key: string; children: ReactNode }>; readonly activeKey?: string }) {
    return createElement("guide-tabs", props, items.find((item) => item.key === activeKey)?.children);
}

mock.module("antd", () => ({
    App: { useApp: () => ({ message }) },
    Badge: MockComponent,
    Button: MockButton,
    Empty: Object.assign(MockComponent, { PRESENTED_IMAGE_SIMPLE: "empty" }),
    Input: Object.assign(MockInput, { TextArea: MockTextArea }),
    Spin: MockComponent,
    Switch: MockComponent,
    Tabs: MockTabs,
    Tooltip: MockComponent,
    Typography: { Text: MockComponent, Title: MockComponent },
    theme: { useToken: () => ({ token: { colorBorderSecondary: "#ddd" } }) },
}));
mock.module("next/navigation", () => ({ useRouter: () => ({ replace: jest.fn() }) }));
mock.module("@/stores/use-user-store", () => ({ useUserStore: <T,>(selector: (state: { user: { role: string } }) => T) => selector({ user: { role: currentRole } }) }));
mock.module("@/services/api/feature-guide", () => ({ getAdminFeatureGuides, saveAdminFeatureGuide }));
mock.module("@/components/feature-guide/markdown-content", () => ({ MarkdownContent: MockComponent }));
mock.module("@/components/feature-guide/feature-guide-dialog", () => ({ FeatureGuideDialog: MockComponent }));

const { default: AdminFeatureGuidesPage } = await import("./page");

const guides: FeatureGuide[] = [
    { surface: "canvas", enabled: true, title: "画布说明", pages: ["画布内容"], version: 2 },
    { surface: "image", enabled: false, title: "生图说明", pages: ["生图内容"], version: 3 },
    { surface: "video", enabled: false, title: "视频说明", pages: ["视频内容"], version: 4 },
];

async function renderPage() {
    let renderer: ReactTestRenderer | undefined;
    await act(async () => {
        renderer = create(<AdminFeatureGuidesPage />);
    });
    if (!renderer) throw new Error("feature guide page did not render");
    return renderer;
}

async function renderLoadedPage() {
    getAdminFeatureGuides.mockResolvedValue(guides);
    return renderPage();
}

afterEach(() => {
    currentRole = "super_admin";
    getAdminFeatureGuides.mockReset();
    saveAdminFeatureGuide.mockReset();
    message.error.mockClear();
    message.success.mockClear();
    message.warning.mockClear();
});

describe("admin feature guides", () => {
    it("keeps drafts per surface and saves only the active surface", async () => {
        const renderer = await renderLoadedPage();
        await act(async () => {
            renderer.root.findByType(MockInput).props.onChange({ target: { value: "新的画布说明" } });
            renderer.root.findByType(MockTabs).props.onChange("image");
        });
        expect(renderer.root.findByType(MockInput).props.value).toBe("生图说明");

        await act(async () => {
            renderer.root.findByType(MockTabs).props.onChange("canvas");
        });
        expect(renderer.root.findByType(MockInput).props.value).toBe("新的画布说明");

        saveAdminFeatureGuide.mockResolvedValue({ ...guides[0], title: "新的画布说明", version: 5 });
        await act(async () => {
            renderer.root.findAllByType(MockButton).find((button) => button.props.children === "保存")?.props.onClick();
        });
        expect(saveAdminFeatureGuide).toHaveBeenCalledWith("canvas", { enabled: true, title: "新的画布说明", pages: ["画布内容"] });
        await act(async () => renderer.unmount());
    });

    it("blocks saving an enabled guide that has an empty page", async () => {
        const renderer = await renderLoadedPage();
        await act(async () => {
            renderer.root.findByType(MockTextArea).props.onChange({ target: { value: "" } });
            renderer.root.findAllByType(MockButton).find((button) => button.props.children === "保存")?.props.onClick();
        });
        expect(message.warning).toHaveBeenCalledWith("启用前请填写全部引导页面");
        expect(saveAdminFeatureGuide).not.toHaveBeenCalled();
        await act(async () => renderer.unmount());
    });

    it("does not expose saving until a failed load is retried successfully", async () => {
        getAdminFeatureGuides.mockRejectedValueOnce(new Error("network down")).mockResolvedValueOnce(guides);
        const renderer = await renderPage();
        expect(renderer.root.findAllByType(MockButton).some((button) => button.props.children === "保存")).toBe(false);
        expect(saveAdminFeatureGuide).not.toHaveBeenCalled();

        await act(async () => {
            renderer.root.findAllByType(MockButton).find((button) => button.props.children === "重新加载")?.props.onClick();
            await Promise.resolve();
        });
        expect(getAdminFeatureGuides).toHaveBeenCalledTimes(2);
        expect(renderer.root.findAllByType(MockButton).some((button) => button.props.children === "保存")).toBe(true);
        await act(async () => renderer.unmount());
    });

    it("ignores an older load response after the latest draft is edited", async () => {
        const older = deferred<FeatureGuide[]>();
        const latest = deferred<FeatureGuide[]>();
        getAdminFeatureGuides.mockImplementationOnce(() => older.promise).mockImplementationOnce(() => latest.promise);
        const renderer = await renderPage();

        currentRole = "tenant_admin";
        await act(async () => {
            renderer.update(<AdminFeatureGuidesPage />);
        });
        currentRole = "super_admin";
        await act(async () => {
            renderer.update(<AdminFeatureGuidesPage />);
        });
        expect(getAdminFeatureGuides).toHaveBeenCalledTimes(2);

        const newestGuides = guides.map((guide) => (guide.surface === "canvas" ? { ...guide, title: "最新画布说明" } : guide));
        await act(async () => {
            latest.resolve(newestGuides);
            await latest.promise;
        });
        await act(async () => {
            renderer.root.findByType(MockInput).props.onChange({ target: { value: "保留的草稿" } });
        });

        await act(async () => {
            older.resolve(guides);
            await older.promise;
        });
        expect(renderer.root.findByType(MockInput).props.value).toBe("保留的草稿");
        await act(async () => renderer.unmount());
    });
});
