import { describe, expect, it, mock } from "bun:test";
import { createElement, type ReactNode } from "react";
import type { ReactTestRenderer } from "react-test-renderer";
import { act, create } from "react-test-renderer";

function MockTooltip({ children, ...props }: { children?: ReactNode; title?: ReactNode }) {
    return createElement("channel-remark-tooltip", props, children);
}

mock.module("antd", () => ({ Tooltip: MockTooltip }));

const { ChannelNameWithRemark } = await import("./channel-name-with-remark");

Object.defineProperty(globalThis, "IS_REACT_ACT_ENVIRONMENT", { configurable: true, value: true });

describe("ChannelNameWithRemark", () => {
    it("shows the channel remark in a tooltip", async () => {
        let renderer: ReactTestRenderer | undefined;
        await act(async () => { renderer = create(<ChannelNameWithRemark name="渠道一" remark="  主线路  " />); });
        expect(renderer?.root.findByType(MockTooltip).props.title).toBe("主线路");
        await act(async () => renderer?.unmount());
    });

    it("does not render an empty tooltip", async () => {
        let renderer: ReactTestRenderer | undefined;
        await act(async () => { renderer = create(<ChannelNameWithRemark name="渠道一" remark="  " />); });
        expect(renderer?.root.findAllByType(MockTooltip)).toHaveLength(0);
        await act(async () => renderer?.unmount());
    });
});
