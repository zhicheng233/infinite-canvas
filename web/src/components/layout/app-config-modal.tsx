"use client";

import { App, Button, Form, Input, Modal, Select, Tabs } from "antd";
import { useState } from "react";

import { ModelPicker } from "@/components/model-picker";
import { filterModelsByCapability, modelOptionLabel, normalizeModelOptionValue, useConfigStore, type AiConfig, type ModelCapability } from "@/stores/use-config-store";

type ModelGroup = {
    capability: ModelCapability;
    modelKey: "imageModel" | "videoModel" | "textModel" | "audioModel";
    modelsKey: "imageModels" | "videoModels" | "textModels" | "audioModels";
    defaultLabel: string;
    optionsLabel: string;
};

const modelGroups: ModelGroup[] = [
    { capability: "image", modelKey: "imageModel", modelsKey: "imageModels", defaultLabel: "默认生图模型", optionsLabel: "生图模型可选项" },
    { capability: "text", modelKey: "textModel", modelsKey: "textModels", defaultLabel: "默认文本模型", optionsLabel: "文本模型可选项" },
];

export function AppConfigModal() {
    const { message } = App.useApp();
    const [activeTab, setActiveTab] = useState("preferences");
    const config = useConfigStore((state) => state.config);
    const updateConfig = useConfigStore((state) => state.updateConfig);
    const isConfigOpen = useConfigStore((state) => state.isConfigOpen);
    const shouldPromptContinue = useConfigStore((state) => state.shouldPromptContinue);
    const setConfigDialogOpen = useConfigStore((state) => state.setConfigDialogOpen);
    const clearPromptContinue = useConfigStore((state) => state.clearPromptContinue);
    const modelOptions = config.models.map((model) => ({ label: modelOptionLabel(config, model), value: model }));

    const finishConfig = () => {
        setConfigDialogOpen(false);
        if (shouldPromptContinue) {
            message.success("配置已保存，请继续刚才的请求");
            clearPromptContinue();
        }
    };

    const updateCapabilityModels = (group: ModelGroup, models: string[]) => {
        const next = Array.from(new Set(models.map((model) => normalizeModelOptionValue(model, config.channels)).filter(Boolean)));
        updateConfig(group.modelsKey, next);
        if (!next.includes(config[group.modelKey])) updateConfig(group.modelKey, next[0] || "");
    };

    function normalizeImageCount(value: string) {
        const n = parseInt(value, 10);
        if (isNaN(n) || n < 1) return "3";
        return String(Math.min(n, 15));
    }

    return (
        <Modal
            title={
                <div>
                    <div className="text-lg font-semibold">用户偏好设置</div>
                    <div className="mt-1 text-xs font-normal text-stone-500">模型选择和生成偏好</div>
                </div>
            }
            open={isConfigOpen}
            width={780}
            centered
            onCancel={() => setConfigDialogOpen(false)}
            styles={{ body: { maxHeight: "72vh", overflowY: "auto", paddingRight: 12 } }}
            footer={
                <Button type="primary" onClick={finishConfig}>
                    完成
                </Button>
            }
        >
            <Tabs
                activeKey={activeTab}
                onChange={setActiveTab}
                items={[
                    {
                        key: "models",
                        label: "模型",
                        children: (
                            <Form layout="vertical" requiredMark={false}>
                                <div className="mb-4 rounded-lg border border-stone-200 p-3 dark:border-stone-800">
                                    <div className="text-sm font-semibold">默认模型和可选项</div>
                                    <div className="mt-1 text-xs leading-5 text-stone-500">可选项决定各处下拉框展示哪些模型；管理员可在后台配置上游 API。</div>
                                </div>
                                <div className="grid gap-4 md:grid-cols-2">
                                    {modelGroups.map((group) => (
                                        <Form.Item key={group.modelsKey} label={group.optionsLabel} className="mb-0">
                                            <Select
                                                mode="tags"
                                                showSearch
                                                allowClear
                                                maxTagCount="responsive"
                                                placeholder={config.models.length ? "请选择或输入模型" : "请选择模型"}
                                                value={config[group.modelsKey]}
                                                options={modelOptions}
                                                onChange={(models) => updateCapabilityModels(group, models)}
                                            />
                                        </Form.Item>
                                    ))}
                                </div>
                                <div className="mt-4 grid gap-4 md:grid-cols-2">
                                    {modelGroups.map((group) => (
                                        <Form.Item key={group.modelKey} label={group.defaultLabel} className="mb-0">
                                            <ModelPicker config={config} value={config[group.modelKey]} onChange={(model) => updateConfig(group.modelKey, model)} capability={group.capability} fullWidth />
                                        </Form.Item>
                                    ))}
                                </div>
                            </Form>
                        ),
                    },
                    {
                        key: "preferences",
                        label: "生成偏好",
                        children: (
                            <Form layout="vertical" requiredMark={false}>
                                <Form.Item label="画布默认生图张数" extra="新建画布生图和配置节点默认使用，单个节点仍可单独覆盖。" className="mb-4">
                                    <Input
                                        type="number"
                                        min={1}
                                        max={15}
                                        value={config.canvasImageCount}
                                        onChange={(event) => updateConfig("canvasImageCount", event.target.value)}
                                        onBlur={(event) => updateConfig("canvasImageCount", normalizeImageCount(event.target.value))}
                                    />
                                </Form.Item>
                                <Form.Item label="系统提示词" className="mb-0">
                                    <Input.TextArea rows={4} value={config.systemPrompt} placeholder="例如：你是一位擅长电影感写实摄影的视觉导演。" onChange={(event) => updateConfig("systemPrompt", event.target.value)} />
                                </Form.Item>
                            </Form>
                        ),
                    },
                ]}
            />
        </Modal>
    );
}
