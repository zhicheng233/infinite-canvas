import { describe, expect, it, jest, mock } from "bun:test";
import { createElement, type ComponentProps, type ReactNode } from "react";
import type { ReactTestRenderer } from "react-test-renderer";
import { act, create } from "react-test-renderer";

import type { FeatureGuide } from "@/services/api/feature-guide";

type MockProps = Record<string, unknown> & { children?: ReactNode };

function MockButton(props: MockProps) {
    return createElement("feature-guide-button", props, props.children);
}

function MockModal({ children, footer, ...props }: MockProps) {
    return createElement("feature-guide-modal", props, children, footer as ReactNode);
}

function MockMarkdownContent(props: MockProps) {
    return createElement("markdown-content", props);
}

mock.module("antd", () => ({ Button: MockButton, Modal: MockModal }));
mock.module("./markdown-content", () => ({ MarkdownContent: MockMarkdownContent }));

const { FeatureGuideDialog } = await import("./feature-guide-dialog");

Object.defineProperty(globalThis, "IS_REACT_ACT_ENVIRONMENT", { configurable: true, value: true });

const guide: FeatureGuide = { surface: "canvas", enabled: true, title: "画布引导", pages: ["第一页", "第二页"], version: 2 };

async function renderDialog(props: Partial<ComponentProps<typeof FeatureGuideDialog>> = {}) {
    let renderer: ReactTestRenderer | undefined;
    await act(async () => {
        renderer = create(<FeatureGuideDialog open guide={guide} {...props} />);
    });
    if (!renderer) throw new Error("feature guide dialog did not render");
    return renderer;
}

function findButton(renderer: ReactTestRenderer, label: string) {
    return renderer.root.findAllByType(MockButton).find((button) => button.props.children === label);
}

describe("FeatureGuideDialog", () => {
    it("uses the standard dialog size with a scrollable body", async () => {
        const renderer = await renderDialog();
        const modal = renderer.root.findByType(MockModal);
        const body = renderer.root.findByProps({ className: "thin-scrollbar max-h-[min(62vh,640px)] min-h-56 overflow-y-auto pr-1" });

        expect(modal.props.width).toBe(760);
        expect(modal.props.styles).toBeUndefined();
        expect(body.props.className).toContain("max-h-[min(62vh,640px)]");
        await act(async () => renderer.unmount());
    });

    it("locks required mode and completes only from the final page", async () => {
        const onComplete = jest.fn();
        const renderer = await renderDialog({ onComplete });
        const modal = renderer.root.findByType(MockModal);

        expect(modal.props.closable).toBe(false);
        expect(modal.props.maskClosable).toBe(false);
        expect(modal.props.keyboard).toBe(false);
        expect(renderer.root.findByType(MockMarkdownContent).props.content).toBe("第一页");
        expect(findButton(renderer, "完成")).toBeUndefined();

        await act(async () => findButton(renderer, "下一页")?.props.onClick());
        expect(renderer.root.findByType(MockMarkdownContent).props.content).toBe("第二页");
        expect(onComplete).not.toHaveBeenCalled();

        await act(async () => findButton(renderer, "完成")?.props.onClick());
        expect(onComplete).toHaveBeenCalledTimes(1);
        await act(async () => renderer.unmount());
    });

    it("resets to the first page when the guide version changes", async () => {
        const renderer = await renderDialog();
        await act(async () => findButton(renderer, "下一页")?.props.onClick());

        await act(async () => {
            renderer.update(<FeatureGuideDialog open guide={{ ...guide, pages: ["新版第一页", "新版第二页"], version: 3 }} />);
        });
        expect(renderer.root.findByType(MockMarkdownContent).props.content).toBe("新版第一页");
        await act(async () => renderer.unmount());
    });

    it("allows preview mode to close without marking completion", async () => {
        const onClose = jest.fn();
        const renderer = await renderDialog({ mode: "preview", onClose });
        const modal = renderer.root.findByType(MockModal);

        expect(modal.props.closable).toBe(true);
        expect(modal.props.maskClosable).toBe(true);
        expect(modal.props.keyboard).toBe(true);
        await act(async () => modal.props.onCancel());
        expect(onClose).toHaveBeenCalledTimes(1);
        await act(async () => renderer.unmount());
    });

    it("shows a stable placeholder for an empty preview draft", async () => {
        const renderer = await renderDialog({ mode: "preview" });
        await act(async () => findButton(renderer, "下一页")?.props.onClick());
        await act(async () => {
            renderer.update(<FeatureGuideDialog open guide={{ ...guide, pages: [] }} mode="preview" />);
        });

        expect(renderer.root.findAllByType(MockMarkdownContent)).toHaveLength(0);
        expect(renderer.root.findByProps({ children: "暂无引导内容" })).toBeTruthy();
        expect(findButton(renderer, "关闭")).toBeTruthy();
        await act(async () => renderer.unmount());
    });
});
