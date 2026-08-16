import { expect, mock, test } from "bun:test";
import { createElement, type ReactNode } from "react";
import { act, create } from "react-test-renderer";

import { createDefaultCustomVideoConfig, normalizeAndValidateCustomVideoConfig, type CustomVideoConfig } from "@/lib/custom-video-config";

Object.defineProperty(globalThis, "IS_REACT_ACT_ENVIRONMENT", { configurable: true, value: true });

const presetConfig = {
    seconds: { enabled: true, key: "seconds", mode: "options", options: [5, 8], default: 5 },
    dimensions: { enabled: true, mode: "size", key: "size", options: ["1280x720", "720x1280"], default: "1280x720" },
    images: { enabled: false, required: false, key: "images", max_count: 1 },
    input_reference: { enabled: true, required: false, key: "input_reference", max_count: 1 },
    style_references: { enabled: true, required: true, key: "style_references", max_count: 4 },
    element_references: { enabled: true, required: false, key: "element_references", max_count: 3 },
    reference_images: { enabled: true, required: true, key: "reference_images", max_count: 1 },
    reference_mode: { enabled: false, key: "reference_mode", options: [], default: "" },
    input_video: { enabled: true, required: true, key: "input_video", max_count: 1 },
    audio: { enabled: false, key: "audio", mode: "fixed", value: false },
    n: { enabled: true, key: "n", value: 1 },
} satisfies CustomVideoConfig;

const preset = { id: 7, name: "Omni", config: presetConfig, created_at: "", updated_at: "" } as const;
let appliedConfig: CustomVideoConfig | undefined;
let currentConfig: CustomVideoConfig | undefined;

type MockComponentProps = Readonly<Record<string, unknown>> & { readonly children?: ReactNode };

function MockComponent({ children }: MockComponentProps) {
    return createElement("div", null, children);
}

function MockFormItem({ children, noStyle, ...props }: MockComponentProps) {
    return createElement("form-item", props, noStyle && typeof children === "function" ? children() : children);
}

const formInstance = {
    getFieldValue: (path?: string | string[]) => {
        if (path === "video_custom_config" || path === undefined) return currentConfig;
        if (!Array.isArray(path)) return undefined;
        return path.reduce<unknown>((value, key) => (value && typeof value === "object" ? (value as Record<string, unknown>)[key] : undefined), { video_custom_config: currentConfig });
    },
    resetFields: () => undefined,
    setFieldValue: (path: string | string[], value: unknown) => {
        if (path === "video_custom_config" && value && typeof value === "object") {
            appliedConfig = value as CustomVideoConfig;
            currentConfig = value as CustomVideoConfig;
        }
        if (Array.isArray(path) && path[0] === "video_custom_config" && typeof path[1] === "string" && value && typeof value === "object") currentConfig = { ...currentConfig, [path[1]]: value } as CustomVideoConfig;
    },
    submit: () => undefined,
};
const MockForm = Object.assign(MockComponent, { Item: MockFormItem, useForm: () => [formInstance], useWatch: () => currentConfig });
const MockSelect = Object.assign(({ children, ...props }: MockComponentProps) => createElement("preset-select", props, children), { Option: MockComponent });
const MockModal = Object.assign(MockComponent, { confirm: () => undefined });

mock.module("antd", () => ({
    App: { useApp: () => ({ message: { error: () => undefined, success: () => undefined } }) },
    Alert: MockComponent,
    Button: MockComponent,
    Form: MockForm,
    Input: MockComponent,
    InputNumber: MockComponent,
    Modal: MockModal,
    Popover: MockComponent,
    Segmented: MockComponent,
    Select: MockSelect,
    Switch: (props: MockComponentProps) => createElement("switch", props),
    Tooltip: MockComponent,
    Typography: { Text: MockComponent },
}));
mock.module("@/services/api/video-config-presets", () => ({
    createVideoConfigPreset: async () => preset,
    deleteVideoConfigPreset: async () => ({ deleted: true as const }),
    listVideoConfigPresets: async () => [preset],
}));

const { Form } = await import("antd");
const { CustomVideoConfigPresets, summarizeMediaRequirements } = await import("./custom-video-config-presets");
const { CustomVideoConfigEditor } = await import("./custom-video-config-editor");

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
    expect(appliedConfig.images).not.toBe(presetConfig.images);
    expect(appliedConfig.input_reference).not.toBe(presetConfig.input_reference);
    expect(appliedConfig.style_references).not.toBe(presetConfig.style_references);
    expect(appliedConfig.element_references).not.toBe(presetConfig.element_references);
    expect(appliedConfig.reference_images).not.toBe(presetConfig.reference_images);
    expect(appliedConfig.input_video).not.toBe(presetConfig.input_video);
    expect([appliedConfig.images.required, appliedConfig.input_reference.required, appliedConfig.style_references.required, appliedConfig.element_references.required, appliedConfig.reference_images.required, appliedConfig.input_video.required]).toEqual([
        false,
        false,
        true,
        false,
        true,
        true,
    ]);
});

test("preset summary distinguishes optional and required media roles", () => {
    expect(
        summarizeMediaRequirements({
            enabled: ["images", "style_references"],
            aliases: { images: "images", style_references: "style_references" },
            seconds: null,
            dimensions: null,
            media_limits: { images: 1, style_references: 4 },
            media_required: { images: false, style_references: true },
            reference_mode: null,
            audio: null,
            n: null,
        }),
    ).toBe("普通参考图上限 1，可选；风格参考图上限 4，必填");
});

test("media required switch is visible only while the media role is enabled and disabling clears it", async () => {
    currentConfig = { ...createDefaultCustomVideoConfig(), input_reference: { enabled: true, required: true, key: "input_reference", max_count: 1 } };
    let renderer: ReturnType<typeof create> | undefined;
    await act(async () => {
        renderer = create(createElement(CustomVideoConfigEditor, { form: formInstance, disabled: false }));
    });
    if (!renderer) throw new Error("editor did not render");

    expect(renderer.root.findAllByType("form-item").filter((item) => item.props.label === "必填")).toHaveLength(1);
    const enabledItem = renderer.root.findAllByType("form-item").find((item) => JSON.stringify(item.props.name) === JSON.stringify(["video_custom_config", "input_reference", "enabled"]));
    if (!enabledItem) throw new Error("input reference switch was not rendered");

    await act(async () => {
        enabledItem.findByType("switch").props.onChange(false);
    });

    expect(currentConfig?.input_reference).toEqual({ enabled: false, required: false, key: "input_reference", max_count: 1 });
    const result = normalizeAndValidateCustomVideoConfig(currentConfig);
    expect(result.ok).toBe(true);
    if (result.ok) expect(result.config.input_reference.required).toBe(false);
});
