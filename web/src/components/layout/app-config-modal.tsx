"use client";

import { App, Button, Form, Input, Modal, Tabs } from "antd";
import { useState } from "react";

import { ModelPicker } from "@/components/model-picker";
import { modelOptionName, useConfigStore, type ModelCapability } from "@/stores/use-config-store";

type ModelGroup = {
    capability: ModelCapability;
    modelKey: "imageModel" | "videoModel" | "textModel" | "audioModel";
    defaultLabel: string;
};

const modelGroups: ModelGroup[] = [
    { capability: "image", modelKey: "imageModel", defaultLabel: "生图渠道和默认模型" },
    { capability: "video", modelKey: "videoModel", defaultLabel: "视频渠道和默认模型" },
    { capability: "text", modelKey: "textModel", defaultLabel: "文本渠道和默认模型" },
    { capability: "audio", modelKey: "audioModel", defaultLabel: "音频渠道和默认模型" },
];

export function AppConfigModal() {
    const { message } = App.useApp();
    const [activeTab, setActiveTab] = useState("preferences");
    const config = useConfigStore((state) => state.config);
    const updateConfig = useConfigStore((state) => state.updateConfig);
    const serverCatalogLoading = useConfigStore((state) => state.serverCatalogLoading);
    const serverCatalogError = useConfigStore((state) => state.serverCatalogError);
    const isConfigOpen = useConfigStore((state) => state.isConfigOpen);
    const shouldPromptContinue = useConfigStore((state) => state.shouldPromptContinue);
    const setConfigDialogOpen = useConfigStore((state) => state.setConfigDialogOpen);
    const clearPromptContinue = useConfigStore((state) => state.clearPromptContinue);

    const finishConfig = () => {
        setConfigDialogOpen(false);
        if (shouldPromptContinue) {
            message.success("配置已保存，请继续刚才的请求");
            clearPromptContinue();
        }
    };

    function normalizeImageCount(value: string) {
        const n = parseInt(value, 10);
        if (isNaN(n) || n < 1) return "1";
        return String(Math.min(n, 15));
    }

    return (
        <Modal
            title={
                <div>
                    <div className="text-lg font-semibold">用户偏好设置</div>
                    <div className="mt-1 text-xs font-normal text-stone-500">这里统一管理默认模型和生成偏好</div>
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
                                    <div className="mt-1 text-xs leading-5 text-stone-500">这里只显示后台已配置计费的模型；未定价模型不会出现在这里。</div>
                                </div>
                                <div className="grid gap-4 md:grid-cols-2">
                                    {modelGroups.map((group) => (
                                        <Form.Item key={group.modelKey} label={group.defaultLabel} className="mb-0">
                                            <ModelPicker config={config} value={config[group.modelKey]} onChange={(model) => updateConfig(group.modelKey, model)} capability={group.capability} fullWidth />
                                            <div className="mt-1 truncate text-xs text-stone-500">
                                                {serverCatalogLoading ? "正在加载渠道模型…" : serverCatalogError ? `加载失败：${serverCatalogError}` : config[group.modelKey] ? modelOptionName(config[group.modelKey]) : "所选渠道暂无可用模型"}
                                            </div>
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
