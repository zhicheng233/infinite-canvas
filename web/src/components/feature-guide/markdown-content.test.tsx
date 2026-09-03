import { describe, expect, it } from "bun:test";
import type { ReactTestRenderer } from "react-test-renderer";
import { act, create } from "react-test-renderer";

import { MarkdownContent } from "./markdown-content";

Object.defineProperty(globalThis, "IS_REACT_ACT_ENVIRONMENT", { configurable: true, value: true });

async function renderMarkdown(content: string) {
    let renderer: ReactTestRenderer | undefined;
    await act(async () => {
        renderer = create(<MarkdownContent content={content} />);
    });
    if (!renderer) throw new Error("markdown content did not render");
    return renderer;
}

describe("MarkdownContent", () => {
    it("drops raw HTML and applies safe external link attributes", async () => {
        const renderer = await renderMarkdown('<script>alert("x")</script>\n\n[外部链接](https://example.com)');
        const link = renderer.root.findByType("a");

        expect(renderer.root.findAllByType("script")).toHaveLength(0);
        expect(link.props.target).toBe("_blank");
        expect(link.props.rel).toBe("noopener noreferrer");
        await act(async () => renderer.unmount());
    });

    it("lazy-loads responsive images", async () => {
        const renderer = await renderMarkdown("![示例图片](https://example.com/image.png)");
        const image = renderer.root.findByType("img");

        expect(image.props.loading).toBe("lazy");
        expect(image.props.className).toContain("max-w-full");
        await act(async () => renderer.unmount());
    });

    it("renders GFM tables inside a horizontal scroll container", async () => {
        const renderer = await renderMarkdown("| 名称 | 状态 |\n| --- | --- |\n| 引导 | 启用 |");
        const table = renderer.root.findByType("table");

        expect(table).toBeTruthy();
        expect(table.parent?.props.className).toContain("overflow-x-auto");
        await act(async () => renderer.unmount());
    });
});
