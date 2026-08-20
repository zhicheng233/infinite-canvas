import { describe, expect, mock, test } from "bun:test";
import { createElement, type ReactNode } from "react";
import { act, create } from "react-test-renderer";

import { createDefaultCustomVideoConfig, customVideoMediaFeatureNames, normalizeAndValidateCustomVideoConfig, type CustomVideoConfig } from "@/lib/custom-video-config";

Object.defineProperty(globalThis, "IS_REACT_ACT_ENVIRONMENT", { configurable: true, value: true });

const presetConfig = {
    seconds: { enabled: true, key: "seconds", mode: "options", options: [5, 8], default: 5 },
    dimensions: { enabled: true, mode: "size", key: "size", options: ["1280x720", "720x1280"], default: "1280x720" },
    images: { enabled: true, required: false, key: "images", max_count: 2 },
    input_reference: { enabled: true, required: false, key: "input_reference", max_count: 2 },
    style_references: { enabled: true, required: true, key: "style_references", max_count: 5 },
    element_references: { enabled: true, required: false, key: "element_references", max_count: 4 },
    reference_images: { enabled: true, required: true, key: "reference_images", max_count: 5 },
    reference_mode: { enabled: false, key: "reference_mode", options: [], default: "" },
    input_video: { enabled: true, required: true, key: "input_video", max_count: 2 },
    audio: { enabled: false, key: "audio", mode: "fixed", value: false },
    n: { enabled: true, key: "n", value: 1 },
} satisfies CustomVideoConfig;

const preset = { id: 7, name: "Omni", config: presetConfig, created_at: "", updated_at: "" } as const;
const aboveFormerCapMediaCountValues = [2, 2, 5, 4, 5, 2] as const;
let appliedConfig: CustomVideoConfig | undefined;
let currentConfig: CustomVideoConfig | undefined;
let presetCreateCalls = 0;

type MockComponentProps = Readonly<Record<string, unknown>> & { readonly children?: ReactNode };

function MockComponent({ children }: MockComponentProps) {
    return createElement("div", null, children);
}

function MockFormComponent({ children, ...props }: MockComponentProps) {
    return createElement("mock-form", props, children);
}

function MockInputNumber(props: MockComponentProps) {
    return createElement("input-number", props);
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
const MockForm = Object.assign(MockFormComponent, { Item: MockFormItem, useForm: () => [formInstance], useWatch: () => currentConfig });
const MockSelect = Object.assign(({ children, ...props }: MockComponentProps) => createElement("preset-select", props, children), { Option: MockComponent });
const MockModal = Object.assign(MockComponent, { confirm: () => undefined });

mock.module("antd", () => ({
    App: { useApp: () => ({ message: { error: () => undefined, success: () => undefined } }) },
    Alert: MockComponent,
    Button: MockComponent,
    Form: MockForm,
    Input: MockComponent,
    InputNumber: MockInputNumber,
    Modal: MockModal,
    Popover: MockComponent,
    Segmented: MockComponent,
    Select: MockSelect,
    Switch: (props: MockComponentProps) => createElement("switch", props),
    Tooltip: MockComponent,
    Typography: { Text: MockComponent },
}));
mock.module("@/services/api/video-config-presets", () => ({
    createVideoConfigPreset: async () => {
        presetCreateCalls += 1;
        return preset;
    },
    deleteVideoConfigPreset: async () => ({ deleted: true as const }),
    listVideoConfigPresets: async () => [preset],
}));

const { Form } = await import("antd");
const { CustomVideoConfigPresets, summarizeMediaRequirements } = await import("./custom-video-config-presets");
const { CustomVideoConfigEditor } = await import("./custom-video-config-editor");
const { normalizeVideoModelFormValues } = await import("./model-video-config-fields");

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
    expect(mediaCounts(appliedConfig)).toEqual(aboveFormerCapMediaCountValues);
    expect(mediaCounts(appliedConfig)).not.toEqual([1, 1, 1, 1, 1, 1]);
    expect("id" in appliedConfig).toBe(false);
    expect("preset_id" in appliedConfig).toBe(false);
    expect(appliedConfig).not.toBe(presetConfig);
    expect(appliedConfig.seconds).not.toBe(presetConfig.seconds);
    if (appliedConfig.seconds.mode === "options") expect(appliedConfig.seconds.options).not.toBe(presetConfig.seconds.options);
    expect(appliedConfig.dimensions).not.toBe(presetConfig.dimensions);
    expect(appliedConfig.dimensions.options).not.toBe(presetConfig.dimensions.options);
    expect(appliedConfig.images).not.toBe(presetConfig.images);
    expect(appliedConfig.input_reference).not.toBe(presetConfig.input_reference);
    expect(appliedConfig.style_references).not.toBe(presetConfig.style_references);
    expect(appliedConfig.element_references).not.toBe(presetConfig.element_references);
    expect(appliedConfig.reference_images).not.toBe(presetConfig.reference_images);
    expect(appliedConfig.reference_mode).not.toBe(presetConfig.reference_mode);
    expect(appliedConfig.reference_mode.options).not.toBe(presetConfig.reference_mode.options);
    expect(appliedConfig.input_video).not.toBe(presetConfig.input_video);
    expect(appliedConfig.audio).not.toBe(presetConfig.audio);
    expect(appliedConfig.n).not.toBe(presetConfig.n);
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
    currentConfig = { ...createDefaultCustomVideoConfig(), input_reference: { enabled: true, required: true, key: "input_reference", max_count: 0 } };
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

    expect(currentConfig?.input_reference).toEqual(createDefaultCustomVideoConfig().input_reference);
    const result = normalizeAndValidateCustomVideoConfig(currentConfig);
    expect(result.ok).toBe(true);
    if (result.ok) expect(result.config.input_reference.required).toBe(false);
});

test("media count controls are positive integer fields without model policy ceilings", async () => {
    const defaults = createDefaultCustomVideoConfig();
    currentConfig = {
        ...defaults,
        images: { ...defaults.images, enabled: true },
        input_reference: { ...defaults.input_reference, enabled: true },
        style_references: { ...defaults.style_references, enabled: true },
        element_references: { ...defaults.element_references, enabled: true },
        reference_images: { ...defaults.reference_images, enabled: true },
        input_video: { ...defaults.input_video, enabled: true },
    };
    let renderer: ReturnType<typeof create> | undefined;
    await act(async () => {
        renderer = create(createElement(CustomVideoConfigEditor, { form: formInstance, disabled: true }));
    });
    if (!renderer) throw new Error("editor did not render");

    const mediaCountItems = renderer.root.findAllByType("form-item").filter((item) => Array.isArray(item.props.name) && item.props.name[2] === "max_count");
    expect(mediaCountItems).toHaveLength(6);
    for (const item of mediaCountItems) {
        const input = item.findByType("input-number");
        expect(input.props.min).toBe(1);
        expect(input.props.precision).toBe(0);
        expect("max" in input.props).toBe(false);
        expect(input.props.disabled).toBe(true);
    }
});

test("custom model normalization preserves administrator media counts and localizes unsafe values", () => {
    const defaults = createDefaultCustomVideoConfig();
    const accepted = normalizeVideoModelFormValues({
        video_route: "custom",
        video_custom_config: {
            ...defaults,
            images: { ...defaults.images, enabled: true, max_count: 2 },
            reference_images: { ...defaults.reference_images, enabled: true, max_count: 5 },
        },
    });
    expect(accepted.video_custom_config.images.max_count).toBe(2);
    expect(accepted.video_custom_config.reference_images.max_count).toBe(5);

    expect(() =>
        normalizeVideoModelFormValues({
            video_route: "custom",
            video_custom_config: { ...defaults, images: { ...defaults.images, enabled: true, max_count: 1.5 } },
        }),
    ).toThrow("普通参考图.最大数量 必须是正安全整数");
    expect(() =>
        normalizeVideoModelFormValues({
            video_route: "custom",
            video_custom_config: { ...defaults, reference_images: { ...defaults.reference_images, enabled: true, max_count: Number.MAX_SAFE_INTEGER + 1 } },
        }),
    ).toThrow("兼容参考图.最大数量 必须是正安全整数");
});

describe("preset creation validation boundary", () => {
    for (const role of customVideoMediaFeatureNames) {
        for (const maxCount of [0, Number.MAX_SAFE_INTEGER + 1]) {
            test(`${role} rejects enabled max_count ${maxCount} before preset payload creation`, async () => {
                const defaults = createDefaultCustomVideoConfig();
                currentConfig = { ...defaults, [role]: { ...defaults[role], enabled: true, max_count: maxCount } };
                presetCreateCalls = 0;
                let renderer: ReturnType<typeof create> | undefined;
                await act(async () => {
                    renderer = create(createElement(CustomVideoConfigPresets, { form: formInstance, disabled: false }));
                });
                if (!renderer) throw new Error("preset component did not render");

                await act(async () => {
                    await renderer.root.findByType("mock-form").props.onFinish({ name: "invalid" });
                });

                expect(presetCreateCalls).toBe(0);
                await act(async () => renderer.unmount());
            });
        }
    }
});

function mediaCounts(config: CustomVideoConfig): readonly number[] {
    return [config.images.max_count, config.input_reference.max_count, config.style_references.max_count, config.element_references.max_count, config.reference_images.max_count, config.input_video.max_count];
}
