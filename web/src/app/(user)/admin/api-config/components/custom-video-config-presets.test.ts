import { expect, mock, test } from "bun:test";
import { createElement, type ReactNode } from "react";
import { act, create } from "react-test-renderer";

import type { CustomVideoConfig } from "@/lib/custom-video-config";

Object.defineProperty(globalThis, "IS_REACT_ACT_ENVIRONMENT", { configurable: true, value: true });

const presetConfig = {
    seconds: { enabled: true, key: "seconds", mode: "options", options: [5, 8], default: 5 },
    dimensions: { enabled: true, mode: "size", key: "size", options: ["1280x720", "720x1280"], default: "1280x720" },
    images: { enabled: false, key: "images", max_count: 1 },
    input_reference: { enabled: false, key: "input_reference", max_count: 1 },
    style_references: { enabled: true, key: "style_references", max_count: 4 },
    element_references: { enabled: false, key: "element_references", max_count: 3 },
    reference_images: { enabled: false, key: "reference_images", max_count: 1 },
    reference_mode: { enabled: false, key: "reference_mode", options: [], default: "" },
    input_video: { enabled: false, key: "input_video", max_count: 1 },
    audio: { enabled: false, key: "audio", mode: "fixed", value: false },
    n: { enabled: true, key: "n", value: 1 },
} satisfies CustomVideoConfig;

const preset = { id: 7, name: "Omni", config: presetConfig, created_at: "", updated_at: "" } as const;
let appliedConfig: CustomVideoConfig | undefined;

type MockComponentProps = Readonly<Record<string, unknown>> & { readonly children?: ReactNode };

function MockComponent({ children }: MockComponentProps) {
    return createElement("div", null, children);
}

const formInstance = {
    getFieldValue: () => undefined,
    resetFields: () => undefined,
    setFieldValue: (_name: string, value: CustomVideoConfig) => {
        appliedConfig = value;
    },
    submit: () => undefined,
};
const MockForm = Object.assign(MockComponent, { Item: MockComponent, useForm: () => [formInstance], useWatch: () => undefined });
const MockSelect = Object.assign(({ children, ...props }: MockComponentProps) => createElement("preset-select", props, children), { Option: MockComponent });
const MockModal = Object.assign(MockComponent, { confirm: () => undefined });

mock.module("antd", () => ({
    App: { useApp: () => ({ message: { error: () => undefined, success: () => undefined } }) },
    Button: MockComponent,
    Form: MockForm,
    Input: MockComponent,
    Modal: MockModal,
    Popover: MockComponent,
    Select: MockSelect,
    Tooltip: MockComponent,
}));
mock.module("@/services/api/video-config-presets", () => ({
    createVideoConfigPreset: async () => preset,
    deleteVideoConfigPreset: async () => ({ deleted: true as const }),
    listVideoConfigPresets: async () => [preset],
}));

const { Form } = await import("antd");
const { CustomVideoConfigPresets } = await import("./custom-video-config-presets");

test("applying a preset deep-copies nested configuration values", async () => {
    const [form] = Form.useForm();
    let renderer: ReturnType<typeof create> | undefined;
    await act(async () => {
        renderer = create(createElement(CustomVideoConfigPresets, { form, disabled: false }));
    });
    if (!renderer) throw new Error("preset component did not render");

    await act(async () => {
        renderer.root.findByType("preset-select").props.onChange(preset.id);
    });

    if (!appliedConfig) throw new Error("preset config was not applied");
    expect(appliedConfig).toEqual(presetConfig);
    expect(appliedConfig).not.toBe(presetConfig);
    expect(appliedConfig.dimensions.options).not.toBe(presetConfig.dimensions.options);
});
