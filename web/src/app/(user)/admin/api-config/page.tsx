"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Alert, App, Button, Card, Checkbox, Form, Input, InputNumber, Modal, Select, Space, Table, Tag, Switch, Tabs, Popover, Popconfirm, Tooltip } from "antd";
import { Settings, Plus, RefreshCw, Lock, X, Play, Save } from "lucide-react";
import type { ColumnsType } from "antd/es/table";

import { useUserStore } from "@/stores/use-user-store";
import { listAllChannels, createChannel, updateChannel, disableChannel, enableChannel, deleteChannel, type ChannelAdminInfo, type SaveChannelInput, type UpdateChannelInput } from "@/services/api/channels-admin";
import { listChannelModelsAdmin, syncChannelModels, updateChannelModel, enableChannelModel, disableChannelModel } from "@/services/api/channel-models-admin";

import { type ChannelModelInfo } from "@/services/api/channel";
import { listPricing, savePricing, comparePricing, type PricingItem } from "@/services/api/pricing";
import { testApiModel, type ApiModelTestResult } from "@/services/api/api-config";
import { listWebhookConfigs, saveWebhookConfig, testWebhookSend, startPoller, stopPoller, getPollerStatus, listWebhookLogs } from "@/services/api/webhook";
import type { WebhookConfig, WebhookLogItem, PollerStatus, TestSendResult } from "@/services/api/webhook";

const imageRouteOptions = [
    { label: "自动判断", value: "auto" },
    { label: "/v1/images/generations", value: "generations" },
    { label: "/v1/images/edits", value: "edits" },
    { label: "/v1/chat/completions（多模态生图）", value: "chat" },
    { label: "/v1/chat/completions（Banana 参数）", value: "banana" },
];

const videoRouteOptions = [
    { label: "默认 /v1/videos", value: "auto" },
    { label: "/v1/videos", value: "openai" },
    { label: "/v1/videos（JSON / veo）", value: "veo_json" },
    { label: "/v1/videos（JSON / yijia）", value: "yijia" },
    { label: "/v1/videos JSON / Waninter", value: "waninter" },
    { label: "/v1/videos/generations", value: "xai" },
    { label: "/v1/video/generations", value: "newapi" },
    { label: "Seedance /contents/generations/tasks", value: "seedance" },
];

const WEBHOOK_PLATFORMS = ["feishu", "dtalk", "wecom", "telegram"];
const PLATFORM_LABELS: Record<string, string> = {
  feishu: "飞书",
  dtalk: "钉钉",
    wecom: "企业微信",
    telegram: "Telegram",
};

function parseDurationInput(value: string) {
    return Array.from(
        new Set(
            String(value || "")
                .split(",")
                .map((item) => Math.floor(Number(item.trim()) || 0))
                .filter((item) => item > 0),
        ),
    ).sort((left, right) => left - right);
}

function formatDurationInput(values?: number[]) {
    return (values || []).join(",");
}

function PricingScopeModal({
    open,
    channels,
    onApplyGlobal,
    onApplyLocal,
    onCancel,
}: {
    open: boolean;
    channels: Array<{ channel_id: number; channel_name: string }>;
    onApplyGlobal: () => void;
    onApplyLocal: () => void;
    onCancel: () => void;
}) {
    return (
        <Modal
            title="该模型存在于多个渠道"
            open={open}
            onCancel={onCancel}
            footer={null}
        >
            <p className="mb-3 text-sm text-stone-600 dark:text-stone-400">
                该模型在以下渠道均存在，请选择计费设置的作用范围：
            </p>
            <ul className="mb-4 space-y-1">
                {channels.map((ch) => (
                    <li key={ch.channel_id} className="text-sm text-stone-800 dark:text-stone-200">
                        {ch.channel_name}
                    </li>
                ))}
            </ul>
            <div className="flex justify-end gap-2">
                <Button onClick={onApplyLocal}>仅本渠道</Button>
                <Button type="primary" onClick={onApplyGlobal}>应用到所有渠道</Button>
            </div>
        </Modal>
    );
}

/** Wraps InputNumber with local state so typing doesn't trigger full page re-render. */
function PricingInput({ value, onChange, ...rest }: { value: number; onChange: (v: number) => void } & Omit<React.ComponentProps<typeof InputNumber>, "value" | "onChange">) {
    const [localVal, setLocalVal] = useState(value);
    // Sync external changes (e.g., after save resets pricingData)
    useEffect(() => { setLocalVal(value); }, [value]);
    return <InputNumber size="small" min={0} value={localVal} onChange={(v) => setLocalVal(v ?? 0)} onBlur={() => onChange(localVal)} {...rest} />;
}

export default function AdminApiConfigPage() {
    const { message } = App.useApp();
    const user = useUserStore((s) => s.user);
    const isSuperAdmin = user?.role === "super_admin";

    const [channels, setChannels] = useState<ChannelAdminInfo[]>([]);
    const [loadingChannels, setLoadingChannels] = useState(false);
    const [syncingChannelId, setSyncingChannelId] = useState<number | null>(null);

    // Channel creation/edit modals
    const [isChannelModalOpen, setIsChannelModalOpen] = useState(false);
    const [editingChannel, setEditingChannel] = useState<ChannelAdminInfo | null>(null);
    const [channelForm] = Form.useForm();

    // Models Panel
    const [selectedChannel, setSelectedChannel] = useState<ChannelAdminInfo | null>(null);
    const [models, setModels] = useState<ChannelModelInfo[]>([]);
    const [loadingModels, setLoadingModels] = useState(false);

    // Model configuration modal
    const [editingModel, setEditingModel] = useState<ChannelModelInfo | null>(null);
    const [isModelModalOpen, setIsModelModalOpen] = useState(false);
    const [modelForm] = Form.useForm();

    // Capability editing state
    const [modelCapabilities, setModelCapabilities] = useState<Record<number, string[]>>({});
    const [savingCapabilities, setSavingCapabilities] = useState<Record<number, boolean>>({});
    const [savingAll, setSavingAll] = useState(false);

    // Pricing editing state
    const [modelPricing, setModelPricing] = useState<Record<number, { unit_type: string; pricing_mode: string; credits_per_unit: number; pricing_rule: string }>>({});
    const [savingPricing, setSavingPricing] = useState<Record<number, boolean>>({});
    const [pricingData, setPricingData] = useState<PricingItem[]>([]);
    const [pricingModal, setPricingModal] = useState<{ open: boolean; channels: { channel_id: number; channel_name: string }[]; model: ChannelModelInfo | null }>({
        open: false,
        channels: [],
        model: null,
    });

    // Model test state
    const [testModalOpen, setTestModalOpen] = useState(false);
    const [testingModel, setTestingModel] = useState<ChannelModelInfo | null>(null);
    const [testLoading, setTestLoading] = useState(false);
    const [testResult, setTestResult] = useState<ApiModelTestResult | null>(null);
    const [testGeneration, setTestGeneration] = useState("text");

    // Webhook tab state
    const [webhookConfigs, setWebhookConfigs] = useState<WebhookConfig[]>([]);
    const [localConfigs, setLocalConfigs] = useState<Record<string, Partial<WebhookConfig>>>({});
    const [loadingConfigs, setLoadingConfigs] = useState(false);
    const [savingConfigPlatform, setSavingConfigPlatform] = useState<string | null>(null);
    const [pollerStatus, setPollerStatus] = useState<PollerStatus | null>(null);
    const [startingPoller, setStartingPoller] = useState(false);
    const [stoppingPoller, setStoppingPoller] = useState(false);
    const [pollerInterval, setPollerInterval] = useState(30);
    const [savingInterval, setSavingInterval] = useState(false);
    const [cooldownMinutes, setCooldownMinutes] = useState(10);
    const [savingCooldown, setSavingCooldown] = useState(false);
    const [webhookLogs, setWebhookLogs] = useState<WebhookLogItem[]>([]);
    const [loadingLogs, setLoadingLogs] = useState(false);
    const [webhookTestModalOpen, setWebhookTestModalOpen] = useState(false);
    const [webhookTestPlatform, setWebhookTestPlatform] = useState("");
    const [webhookTestMessage, setWebhookTestMessage] = useState("");
    const [webhookTestSending, setWebhookTestSending] = useState(false);
    const [webhookTestSendResult, setWebhookTestSendResult] = useState<TestSendResult | null>(null);

    // Load initial data
    const fetchChannels = async () => {
        setLoadingChannels(true);
        try {
            const data = await listAllChannels();
            setChannels(data || []);
            // Update selectedChannel info if it is open to keep sync status current
            if (selectedChannel) {
                const updated = data.find((c) => c.id === selectedChannel.id);
                if (updated) {
                    setSelectedChannel(updated);
                }
            }
        } catch (err: any) {
            message.error(err?.message || "获取渠道列表失败");
        } finally {
            setLoadingChannels(false);
        }
    };

    const fetchModels = async (channelId: number) => {
        setLoadingModels(true);
        try {
            const data = await listChannelModelsAdmin(channelId);
            setModels(data || []);
            // Initialize local capability editing state from fetched data
            const init: Record<number, string[]> = {};
            for (const m of data || []) {
                init[m.id] = [...m.capabilities];
            }
            setModelCapabilities(init);
        } catch (err: any) {
            message.error(err?.message || "获取渠道模型失败");
        } finally {
            setLoadingModels(false);
        }
    };

    const fetchPricing = async () => {
        try {
            const data = await listPricing();
            setPricingData(data || []);
        } catch {
            // pricing fetch advisory; silently continue
        }
    };

    const fetchWebhookConfigs = async () => {
        setLoadingConfigs(true);
        try {
            const data = await listWebhookConfigs();
            setWebhookConfigs(data || []);
            const init: Record<string, Partial<WebhookConfig>> = {};
            for (const c of data || []) {
                init[c.platform] = { ...c };
            }
            setLocalConfigs(init);
            // Read cooldown from first feishu config (all platforms share the same cooldown)
            const feishuCfg = (data || []).find(c => c.platform === "feishu");
            if (feishuCfg?.cooldown_minutes != null) {
                setCooldownMinutes(feishuCfg.cooldown_minutes);
            }
        } catch (err: any) {
            message.error(err?.message || "获取推送配置失败");
        } finally {
            setLoadingConfigs(false);
        }
    };

    const fetchPollerStatus = async () => {
        try {
            const status = await getPollerStatus();
            setPollerStatus(status);
            setPollerInterval(status.interval_seconds || 30);
        } catch {
            // advisory
        }
    };

    const fetchWebhookLogs = async () => {
        setLoadingLogs(true);
        try {
            const data = await listWebhookLogs(50);
            setWebhookLogs(data || []);
        } catch (err: any) {
            message.error(err?.message || "获取推送日志失败");
        } finally {
            setLoadingLogs(false);
        }
    };

    useEffect(() => {
        void fetchChannels();
        void fetchWebhookConfigs();
        void fetchPollerStatus();
    }, []);

    // Handles synchronization
    const handleSync = async (channelId: number) => {
        setSyncingChannelId(channelId);
        try {
            const res = await syncChannelModels(channelId);
            if (res.synced) {
                message.success("同步渠道模型成功");
            } else {
                message.warning("同步完成，没有模型更新");
            }
            await fetchChannels();
            if (selectedChannel && selectedChannel.id === channelId) {
                setModels([]); // Force-clear to trigger React re-render
                await fetchModels(channelId);
            }
        } catch (err: any) {
            message.error(err?.message || "同步失败");
            // Keep the existing models listed, but refresh status to expose the failure error.
            await fetchChannels();
            if (selectedChannel && selectedChannel.id === channelId) {
                setModels([]); // Force-clear to trigger React re-render
                await fetchModels(channelId);
            }
        } finally {
            setSyncingChannelId(null);
        }
    };

    // Toggles channel enablement
    const handleToggleChannel = async (record: ChannelAdminInfo, checked: boolean) => {
        try {
            if (checked) {
                await enableChannel(record.id);
                message.success(`已启用渠道 "${record.name}"`);
            } else {
                await disableChannel(record.id);
                message.success(`已禁用渠道 "${record.name}"`);
            }
            await fetchChannels();
        } catch (err: any) {
            message.error(err?.message || "操作失败");
        }
    };

    // Deletes a channel
    const handleDelete = async (channelId: number) => {
        try {
            await deleteChannel(channelId);
            message.success("渠道已删除");
            await fetchChannels();
            if (selectedChannel?.id === channelId) {
                setSelectedChannel(null);
                setModels([]);
            }
        } catch (err: any) {
            message.error(err?.response?.data?.msg || err?.message || "删除失败");
        }
    };

    // Toggles model enablement
    const handleToggleModel = async (model: ChannelModelInfo, checked: boolean) => {
        if (!selectedChannel) return;
        try {
            if (checked) {
                await enableChannelModel(selectedChannel.id, model.id);
                message.success(`已启用模型 "${model.model_name}"`);
            } else {
                await disableChannelModel(selectedChannel.id, model.id);
                message.success(`已禁用模型 "${model.model_name}"`);
            }
            await fetchModels(selectedChannel.id);
        } catch (err: any) {
            message.error(err?.message || "操作失败");
        }
    };

    // Open create/edit modal
    const openChannelModal = (channel?: ChannelAdminInfo) => {
        setEditingChannel(channel || null);
        if (channel) {
            channelForm.setFieldsValue({
                name: channel.name,
                base_url: channel.base_url,
                api_key: "", // Write-only: blank initially
                new_api_channel_id: channel.new_api_channel_id || undefined,
                metrics_base_url: channel.metrics_base_url || undefined,
                enabled: channel.enabled,
            });
        } else {
            channelForm.resetFields();
            channelForm.setFieldsValue({ enabled: true });
        }
        setIsChannelModalOpen(true);
    };

    // Save channel (create or update)
    const handleSaveChannel = async (values: any) => {
        try {
            if (editingChannel) {
                const payload: UpdateChannelInput = {
                    name: values.name,
                    base_url: values.base_url,
                    new_api_channel_id: values.new_api_channel_id != null ? Number(values.new_api_channel_id) : null,
                    metrics_base_url: values.metrics_base_url || undefined,
                    enabled: values.enabled,
                };
                if (values.api_key) {
                    payload.api_key = values.api_key;
                }
                await updateChannel(editingChannel.id, payload);
                message.success("修改渠道成功");
            } else {
                const payload: SaveChannelInput = {
                    name: values.name,
                    base_url: values.base_url,
                    api_key: values.api_key || "",
                    enabled: values.enabled,
                    new_api_channel_id: values.new_api_channel_id != null ? Number(values.new_api_channel_id) : null,
                    metrics_base_url: values.metrics_base_url || undefined,
                };
                await createChannel(payload);
                message.success("创建渠道成功");
            }
            setIsChannelModalOpen(false);
            void fetchChannels();
        } catch (err: any) {
            message.error(err?.message || "保存渠道失败");
        }
    };

    // Open model manage panel
    const openModelsPanel = (channel: ChannelAdminInfo) => {
        setSelectedChannel(channel);
        setPricingData([]);
        void fetchModels(channel.id);
        void fetchPricing();
    };

    const closePanel = () => {
        setSelectedChannel(null);
    };

    // -- Capability editing handlers --

    const toggleCap = (model: ChannelModelInfo, cap: string) => {
        setModelCapabilities((prev) => {
            const current = prev[model.id] || [...model.capabilities];
            const next = current.includes(cap) ? current.filter((c) => c !== cap) : [...current, cap];
            return { ...prev, [model.id]: next };
        });
    };

    const handleSaveCapabilities = async (model: ChannelModelInfo) => {
        if (!selectedChannel) return;
        const caps = modelCapabilities[model.id];
        if (!caps || caps.length === 0) {
            message.warning("至少选择一个能力");
            return;
        }
        setSavingCapabilities((prev) => ({ ...prev, [model.id]: true }));
        try {
            await updateChannelModel(selectedChannel.id, model.id, { capabilities: caps });
            message.success("保存能力成功");
            await fetchModels(selectedChannel.id);
        } catch (err: any) {
            message.error(err?.message || "保存能力失败");
        } finally {
            setSavingCapabilities((prev) => ({ ...prev, [model.id]: false }));
        }
    };

    const handleAutoDetect = () => {
        setModelCapabilities((prev) => {
            const next: Record<number, string[]> = {};
            for (const model of models) {
                const name = model.model_name.toLowerCase();
                const existing = prev[model.id] || [...model.capabilities];
                const existingText = existing.filter((c) => c === "text");

                if (name.includes("video") || name.includes("sora") || name.includes("omni")) {
                    // Video model: video + audio + preserve existing text
                    next[model.id] = ["video", "audio", ...existingText];
                } else if (name.includes("image") || name.includes("seedance")) {
                    // Image model: image + preserve existing text
                    next[model.id] = ["image", ...existingText];
                } else {
                    // No match: keep existing
                    next[model.id] = existing;
                }
            }
            return next;
        });
    };

    const handleSaveAll = async () => {
        if (!selectedChannel || models.length === 0) return;
        setSavingAll(true);
        const results = await Promise.allSettled(
            models.map((model) => {
                const caps = modelCapabilities[model.id] || model.capabilities;
                return updateChannelModel(selectedChannel.id, model.id, { capabilities: caps });
            })
        );
        const succeeded = results.filter((r) => r.status === "fulfilled").length;
        const failed = results.filter((r) => r.status === "rejected").length;
        if (failed === 0) {
            message.success(`已保存全部 ${succeeded} 个模型`);
        } else {
            message.warning(`成功保存 ${succeeded}/${models.length} 个模型，${failed} 个失败`);
        }
        await fetchModels(selectedChannel.id);
        setSavingAll(false);
    };

    // -- Pricing editing helpers and handlers --

    const getPricingDefaults = () => ({
        unit_type: "per_image",
        pricing_mode: "per_unit",
        credits_per_unit: 0,
        pricing_rule: "",
    });

    const getPricingForModel = (modelId: number, modelName: string, channelId?: number) => {
        if (modelPricing[modelId]) return modelPricing[modelId];
        // Try channel-specific pricing first
        if (channelId) {
            const cp = pricingData.find((p) => p.model === modelName && p.channel_id === channelId);
            if (cp) return formatPricing(cp);
        }
        // Fallback to global (channel_id=0 or undefined)
        const gp = pricingData.find((p) => p.model === modelName && (!p.channel_id || p.channel_id === 0));
        if (gp) return formatPricing(gp);
        return getPricingDefaults();
    };

    const formatPricing = (p: PricingItem) => ({
        unit_type: p.unit_type || "per_image",
        pricing_mode: p.pricing_mode || "per_unit",
        credits_per_unit: p.credits_per_unit || 0,
        pricing_rule: p.pricing_rule || "",
    });

    const parsePricingRule = (ruleStr?: string) => {
        if (!ruleStr) return { base_credits: 0, resolution_second_rates: {} as Record<string, number> };
        try {
            return JSON.parse(ruleStr);
        } catch {
            return { base_credits: 0, resolution_second_rates: {} };
        }
    };

    const handlePricingChange = (modelId: number, field: string, value: string | number) => {
        setModelPricing((prev) => {
            const current = prev[modelId] || getPricingDefaults();
            const next = { ...current, [field]: value };
            if (field === "unit_type" && value === "per_video_second") {
                next.pricing_mode = "video_dynamic";
            }
            return { ...prev, [modelId]: next };
        });
    };

    const handlePricingRuleChange = (modelId: number, field: string, value: number) => {
        setModelPricing((prev) => {
            const current = prev[modelId] || getPricingDefaults();
            const rule = parsePricingRule(current.pricing_rule);
            if (field === "base_credits") {
                rule.base_credits = value;
            } else {
                rule.resolution_second_rates[field] = value;
            }
            return { ...prev, [modelId]: { ...current, pricing_rule: JSON.stringify(rule) } };
        });
    };

    const doSavePricing = async (model: ChannelModelInfo, channelId: number) => {
        const pricing = modelPricing[model.id];
        if (!pricing) return;
        setSavingPricing((prev) => ({ ...prev, [model.id]: true }));
        try {
            await savePricing({
                model: model.model_name,
                credits_per_unit: pricing.credits_per_unit || 0,
                unit_type: pricing.unit_type || "per_image",
                pricing_mode: pricing.pricing_mode || "per_unit",
                pricing_rule: pricing.pricing_rule || "",
                channel_id: channelId,
            });
            message.success("保存计费成功");
            await fetchPricing();
            setModelPricing((prev) => {
                const next = { ...prev };
                delete next[model.id];
                return next;
            });
        } catch (err: any) {
            message.error(err?.message || "保存计费失败");
        } finally {
            setSavingPricing((prev) => ({ ...prev, [model.id]: false }));
        }
    };

    const handleSavePricing = async (model: ChannelModelInfo) => {
        if (!selectedChannel) return;
        const pricing = modelPricing[model.id];
        if (!pricing) return;

        // Check for cross-channel duplicates
        try {
            const result = await comparePricing(model.model_name);
            if (result.channels.length > 1) {
                setPricingModal({ open: true, channels: result.channels, model });
                return; // Wait for user choice in modal
            }
        } catch {
            // If compare fails, fall through to single-channel save
        }

        // Single-channel flow: save immediately
        await doSavePricing(model, selectedChannel.id);
    };

    const handlePricingApplyGlobal = async () => {
        const model = pricingModal.model;
        if (!model) return;
        setPricingModal((prev) => ({ ...prev, open: false }));
        await doSavePricing(model, 0);
    };

    const handlePricingApplyLocal = async () => {
        const model = pricingModal.model;
        if (!model) return;
        setPricingModal((prev) => ({ ...prev, open: false }));
        await doSavePricing(model, selectedChannel!.id);
    };

    const handlePricingCancel = () => {
        setPricingModal({ open: false, channels: [], model: null });
    };

    // -- Model test handlers --

    const handleOpenTest = (model: ChannelModelInfo) => {
        setTestingModel(model);
        setTestGeneration(model.capabilities.includes("image") ? "image" : model.capabilities.includes("video") ? "video" : "text");
        setTestResult(null);
        setTestModalOpen(true);
    };

    const handleRunTest = async () => {
        if (!testingModel || !selectedChannel) return;
        setTestLoading(true);
        setTestResult(null);
        try {
            const result = await testApiModel({
                model: testingModel.model_name,
                generation: testGeneration,
            });
            setTestResult(result);
        } catch (err: any) {
            message.error(err?.message || "模型测试失败");
        } finally {
            setTestLoading(false);
        }
    };

    // Webhook handlers
    const handleConfigChange = (platform: string, field: string, value: any) => {
        setLocalConfigs((prev) => ({
            ...prev,
            [platform]: { ...(prev[platform] || { platform, webhook_url: "", enabled: false }), [field]: value },
        }));
    };

    const handleSaveConfig = async (platform: string) => {
        const config = localConfigs[platform];
        if (!config || !config.webhook_url) {
            message.warning("请输入 Webhook URL");
            return;
        }
        setSavingConfigPlatform(platform);
        try {
            await saveWebhookConfig({
                platform,
                webhook_url: config.webhook_url,
                enabled: config.enabled ?? false,
                template_down: config.template_down || "",
                template_up: config.template_up || "",
            });
            message.success("保存成功");
            await fetchWebhookConfigs();
        } catch (err: any) {
            message.error(err?.message || "保存失败");
        } finally {
            setSavingConfigPlatform(null);
        }
    };

    const handleTestSend = async () => {
        if (!webhookTestMessage.trim()) {
            message.warning("请输入测试消息");
            return;
        }
        setWebhookTestSending(true);
        setWebhookTestSendResult(null);
        try {
            const result = await testWebhookSend({ platform: webhookTestPlatform, message: webhookTestMessage });
            setWebhookTestSendResult(result);
        } catch (err: any) {
            message.error(err?.message || "发送测试失败");
        } finally {
            setWebhookTestSending(false);
        }
    };

    // Open model edit modal
    const openModelModal = (model: ChannelModelInfo) => {
        setEditingModel(model);
        modelForm.setFieldsValue({
            sort_order: model.sort_order,
            image_generate_route: model.image_generate_route || "auto",
            image_edit_route: model.image_edit_route || "auto",
            video_route: model.video_route || "auto",
            video_durations: formatDurationInput(model.video_durations),
            video_customizable: model.video_customizable,
        });
        setIsModelModalOpen(true);
    };

    // Save model metadata
    const handleSaveModel = async (values: any) => {
        if (!selectedChannel || !editingModel) return;
        try {
            const durations = parseDurationInput(values.video_durations);
            await updateChannelModel(selectedChannel.id, editingModel.id, {
                sort_order: values.sort_order,
                image_generate_route: values.image_generate_route,
                image_edit_route: values.image_edit_route,
                video_route: values.video_route,
                video_durations: durations,
                video_customizable: values.video_customizable,
            });
            message.success("修改模型配置成功");
            setIsModelModalOpen(false);
            void fetchModels(selectedChannel.id);
        } catch (err: any) {
            message.error(err?.message || "保存模型配置失败");
        }
    };

    // Channel table columns
    const channelColumns: ColumnsType<ChannelAdminInfo> = [
        { title: "ID", dataIndex: "id", key: "id", width: 70 },
        { title: "渠道名称", dataIndex: "name", key: "name", width: 150 },
        { title: "接口地址", dataIndex: "base_url", key: "base_url", width: 250, ellipsis: true },
        {
            title: "API Key",
            dataIndex: "has_key",
            key: "has_key",
            width: 100,
            render: (hasKey: boolean) => (hasKey ? <Tag color="green">已配置</Tag> : <Tag color="gold">未配置</Tag>),
        },
        {
            title: "New-API ID",
            dataIndex: "new_api_channel_id",
            key: "new_api_channel_id",
            width: 120,
            render: (val: any) => val ?? "-",
        },
        {
            title: "同步状态",
            key: "sync_status",
            width: 180,
            render: (_, record) => {
                const isSyncing = syncingChannelId === record.id;
                if (isSyncing) {
                    return (
                        <Tag color="blue" icon={<RefreshCw className="animate-spin size-3 mr-1" />}>
                            同步中
                        </Tag>
                    );
                }
                switch (record.sync_status) {
                    case "success":
                        return (
                            <Space direction="vertical" size={0}>
                                <Tag color="green">同步成功</Tag>
                                {record.synced_at && <span className="text-xs text-stone-400">{new Date(record.synced_at).toLocaleString("zh-CN")}</span>}
                            </Space>
                        );
                    case "failed":
                        return (
                            <Space direction="vertical" size={0}>
                                <Popover title="同步失败原因" content={<div className="max-w-xs text-xs text-rose-600 font-mono whitespace-pre-wrap break-all">{record.sync_error || "未知错误"}</div>} trigger="hover">
                                    <Tag color="red" className="cursor-pointer">
                                        同步失败
                                    </Tag>
                                </Popover>
                                {record.synced_at && <span className="text-xs text-stone-400">上次: {new Date(record.synced_at).toLocaleString("zh-CN")}</span>}
                            </Space>
                        );
                    default:
                        return <Tag>未同步</Tag>;
                }
            },
        },
        {
            title: "状态",
            dataIndex: "enabled",
            key: "enabled",
            width: 100,
            render: (enabled: boolean, record) => <Switch checked={enabled} disabled={!isSuperAdmin} onChange={(checked) => handleToggleChannel(record, checked)} />,
        },
        {
            title: "操作",
            key: "actions",
            width: 320,
            render: (_, record) => (
                <Space size="small">
                    <Button size="small" onClick={() => openChannelModal(record)} disabled={!isSuperAdmin && !record.enabled}>
                        编辑
                    </Button>
                    <Button size="small" type="primary" onClick={() => openModelsPanel(record)}>
                        模型管理
                    </Button>
                    <Button size="small" onClick={() => handleSync(record.id)} loading={syncingChannelId === record.id} disabled={!isSuperAdmin}>
                        同步模型
                    </Button>
                    <Popconfirm
                        title={`确定删除渠道 "${record.name}"？如有关联模型则无法删除`}
                        onConfirm={() => handleDelete(record.id)}
                        okText="确定"
                        cancelText="取消"
                        okButtonProps={{ danger: true }}
                    >
                        <Button size="small" danger disabled={!isSuperAdmin}>
                            删除
                        </Button>
                    </Popconfirm>
                </Space>
            ),
        },
    ];

    // Model table columns inside panel
    const modelColumns: ColumnsType<ChannelModelInfo> = useMemo(() => [
        { title: "模型名称", dataIndex: "model_name", key: "model_name", width: 200, ellipsis: true },
        {
            title: "能力",
            dataIndex: "capabilities",
            key: "capabilities",
            width: 220,
            render: (caps: string[], record) => {
                const current = modelCapabilities[record.id] || caps || [];
                return (
                    <Space size={4} wrap>
                        {(["image", "video", "text", "audio"] as const).map((cap) => {
                            const labels: Record<string, string> = { image: "图片", video: "视频", text: "文本", audio: "音频" };
                            return (
                                <Checkbox
                                    key={cap}
                                    checked={current.includes(cap)}
                                    disabled={!isSuperAdmin}
                                    onChange={() => toggleCap(record, cap)}
                                >
                                    {labels[cap]}
                                </Checkbox>
                            );
                        })}
                    </Space>
                );
            },
        },
        { title: "权重", dataIndex: "sort_order", key: "sort_order", width: 80 },
        {
            title: "计费方式",
            key: "pricing_mode",
            width: 200,
            render: (_, record) => {
                const pricing = getPricingForModel(record.id, record.model_name, selectedChannel?.id);
                const rule = parsePricingRule(pricing.pricing_rule);
                const isDynamic = pricing.pricing_mode === "video_dynamic";
                return (
                    <Space direction="vertical" size={4} style={{ width: "100%" }}>
                        <Select
                            value={pricing.pricing_mode}
                            onChange={(v) => handlePricingChange(record.id, "pricing_mode", v)}
                            style={{ width: "100%" }}
                            size="small"
                            disabled={!isSuperAdmin}
                            options={[
                                { label: "按次计费", value: "per_unit" },
                                { label: "视频动态计费", value: "video_dynamic" },
                            ]}
                        />
                        {isDynamic && (
                            <div className="border-t pt-1 space-y-1 w-full">
                                <div className="text-xs text-stone-500">基础积分:</div>
                                <PricingInput
                                    value={rule.base_credits}
                                    onChange={(v) => handlePricingRuleChange(record.id, "base_credits", v)}
                                    disabled={!isSuperAdmin}
                                    style={{ width: "100%" }}
                                />
                                <div className="text-xs text-stone-500 mt-1">分辨率速率:</div>
                                {["720p", "1080p"].map((res) => (
                                    <div key={res} className="flex items-center gap-1">
                                        <span className="text-xs text-stone-500 w-10">{res}:</span>
                                        <PricingInput
                                            value={rule.resolution_second_rates[res] || 0}
                                            onChange={(v) => handlePricingRuleChange(record.id, res, v)}
                                            disabled={!isSuperAdmin}
                                            style={{ width: "70%" }}
                                        />
                                    </div>
                                ))}
                            </div>
                        )}
                    </Space>
                );
            },
        },
        {
            title: "计费单位",
            key: "unit_type",
            width: 150,
            render: (_, record) => {
                const pricing = getPricingForModel(record.id, record.model_name, selectedChannel?.id);
                return (
                    <Select
                        value={pricing.unit_type}
                        onChange={(v) => handlePricingChange(record.id, "unit_type", v)}
                        style={{ width: "100%" }}
                        size="small"
                        disabled={!isSuperAdmin}
                        options={[
                            { label: "每次图片", value: "per_image" },
                            { label: "每次视频", value: "per_video" },
                            { label: "每秒视频", value: "per_video_second" },
                            { label: "每Token", value: "per_token" },
                        ]}
                    />
                );
            },
        },
        {
            title: "积分",
            key: "credits_per_unit",
            width: 100,
            render: (_, record) => {
                const pricing = getPricingForModel(record.id, record.model_name, selectedChannel?.id);
                return (
                    <PricingInput
                        value={pricing.credits_per_unit}
                        onChange={(v) => handlePricingChange(record.id, "credits_per_unit", v)}
                        disabled={!isSuperAdmin}
                        style={{ width: "100%" }}
                    />
                );
            },
        },
        {
            title: "路由与定制配置",
            key: "route_config",
            width: 250,
            render: (_, record) => {
                const configItems = [];
                if (record.capabilities.includes("image")) {
                    configItems.push(`生图: ${record.image_generate_route || "auto"}`);
                    configItems.push(`修图: ${record.image_edit_route || "auto"}`);
                }
                if (record.capabilities.includes("video")) {
                    configItems.push(`视频: ${record.video_route || "auto"}`);
                    if (record.video_durations?.length) {
                        configItems.push(`时长: [${record.video_durations.join(",")}]`);
                    }
                    if (record.video_customizable) {
                        configItems.push("允许自定义");
                    }
                }
                return (
                    <div className="text-xs text-stone-500 space-y-0.5">
                        {configItems.map((item, idx) => (
                            <div key={idx}>{item}</div>
                        ))}
                    </div>
                );
            },
        },
        {
            title: "状态",
            dataIndex: "enabled",
            key: "enabled",
            width: 100,
            render: (enabled: boolean, record) => <Switch checked={enabled} disabled={!isSuperAdmin} onChange={(checked) => handleToggleModel(record, checked)} />,
        },
        {
            title: "操作",
            key: "actions",
            width: 340,
            render: (_, record) => {
                const caps = modelCapabilities[record.id] || record.capabilities || [];
                return (
                    <Space size="small" wrap>
                        <Button size="small" onClick={() => openModelModal(record)} disabled={!isSuperAdmin}>
                            配置
                        </Button>
                        <Button
                            size="small"
                            onClick={() => handleSaveCapabilities(record)}
                            disabled={!isSuperAdmin || caps.length === 0}
                            loading={savingCapabilities[record.id]}
                        >
                            保存能力
                        </Button>
                        <Button
                            size="small"
                            onClick={() => handleSavePricing(record)}
                            disabled={!isSuperAdmin}
                            loading={savingPricing[record.id]}
                        >
                            保存计费
                        </Button>
                        <Button
                            size="small"
                            icon={<Play className="size-3" />}
                            onClick={() => handleOpenTest(record)}
                        >
                            测试
                        </Button>
                    </Space>
                );
            },
        },
    ], [modelCapabilities, modelPricing, pricingData, selectedChannel, isSuperAdmin, toggleCap, handlePricingChange, handlePricingRuleChange, handleSavePricing, handleSaveCapabilities, openModelModal, handleToggleModel, handleOpenTest]);

    return (
        <div>
            <h2 className="mb-4 text-xl font-semibold text-stone-950 dark:text-stone-100">
                <Settings className="mr-2 inline size-5" />
                API 与模型配置
            </h2>
            <Alert
                className="mb-6 !bg-yellow-50 !border-yellow-200"
                type="info"
                showIcon
                message="这里统一管理上游全局渠道、模型目录和指标服务配置"
                description="超级管理员(SuperAdmin)可创建与编辑渠道，并同步渠道模型。同步失败时，已有模型仍会保留在列表中，并显示同步失败原因。非超级管理员仅可查看，无修改权限。"
            />

            {!isSuperAdmin && <Alert className="mb-4" type="warning" showIcon icon={<Lock className="size-4 text-amber-500" />} message="只读模式" description="您当前的权限为非超级管理员，无法进行任何配置修改、同步模型或启用/禁用操作。" />}

            <Tabs defaultActiveKey="channels">
                <Tabs.TabPane tab="渠道与模型管理" key="channels">
                    <Card
                        title="全局渠道列表"
                        extra={
                            <Button type="primary" icon={<Plus className="size-4" />} onClick={() => openChannelModal()} disabled={!isSuperAdmin}>
                                新增渠道
                            </Button>
                        }
                    >
                        <Table rowKey="id" columns={channelColumns} dataSource={channels} loading={loadingChannels} pagination={false} scroll={{ x: 1000 }} />
                    </Card>
                </Tabs.TabPane>

                <Tabs.TabPane tab="消息推送" key="webhook">
                    {/* Card 1: 平台配置 */}
                    <Card title="平台配置" className="mb-4">
                        <Table
                            rowKey="platform"
                            dataSource={WEBHOOK_PLATFORMS.map((p) => localConfigs[p] || { platform: p, webhook_url: "", enabled: false, template_down: "", template_up: "" })}
                            columns={[
                                {
                                    title: "平台",
                                    dataIndex: "platform",
                                    key: "platform",
                                    width: 100,
                                    render: (p: string) => <span className="font-medium text-stone-700 dark:text-stone-300">{PLATFORM_LABELS[p] || p}</span>,
                                },
                                {
                                    title: "Webhook URL",
                                    dataIndex: "webhook_url",
                                    key: "webhook_url",
                                    width: 250,
                                    render: (_, record) => {
                                        const val = localConfigs[record.platform]?.webhook_url ?? "";
                                        return <Input size="small" value={val} onChange={(e) => handleConfigChange(record.platform, "webhook_url", e.target.value)} />;
                                    },
                                },
                                {
                                    title: "启用",
                                    dataIndex: "enabled",
                                    key: "enabled",
                                    width: 70,
                                    render: (_, record) => {
                                        const checked = localConfigs[record.platform]?.enabled ?? false;
                                        return <Switch checked={checked} onChange={(v) => handleConfigChange(record.platform, "enabled", v)} />;
                                    },
                                },
                                {
                                    title: "Down 模板",
                                    key: "template_down",
                                    width: 200,
                                    render: (_, record) => {
                                        const val = localConfigs[record.platform]?.template_down ?? "";
                                        return (
                                            <Input.TextArea
                                                rows={2}
                                                size="small"
                                                value={val}
                                                placeholder="模型 {{model}} 在所有渠道均不可用，时间: {{time}}"
                                                onChange={(e) => handleConfigChange(record.platform, "template_down", e.target.value)}
                                            />
                                        );
                                    },
                                },
                                {
                                    title: "Up 模板",
                                    key: "template_up",
                                    width: 200,
                                    render: (_, record) => {
                                        const val = localConfigs[record.platform]?.template_up ?? "";
                                        return (
                                            <Input.TextArea
                                                rows={2}
                                                size="small"
                                                value={val}
                                                placeholder="模型 {{model}} 已恢复可用，时间: {{time}}"
                                                onChange={(e) => handleConfigChange(record.platform, "template_up", e.target.value)}
                                            />
                                        );
                                    },
                                },
                                {
                                    title: "操作",
                                    key: "actions",
                                    width: 150,
                                    render: (_, record) => (
                                        <Space>
                                            <Button size="small" type="primary" loading={savingConfigPlatform === record.platform} onClick={() => handleSaveConfig(record.platform)}>
                                                保存
                                            </Button>
                                            <Button
                                                size="small"
                                                onClick={() => {
                                                    setWebhookTestPlatform(record.platform);
                                                    setWebhookTestMessage("");
                                                    setWebhookTestSendResult(null);
                                                    setWebhookTestModalOpen(true);
                                                }}
                                            >
                                                测试
                                            </Button>
                                        </Space>
                                    ),
                                },
                            ]}
                            loading={loadingConfigs}
                            pagination={false}
                            scroll={{ x: 1000 }}
                        />
                    </Card>
                    {/* Card 2: 轮询控制 */}
                    <Card title="轮询控制" className="mb-4">
                        <div className="space-y-4">
                            <div className="flex items-center gap-4">
                                <span className="text-sm text-stone-500">轮询状态:</span>
                                <Tag color={pollerStatus?.running ? "green" : "red"}>
                                    {pollerStatus ? (pollerStatus.running ? "运行中" : "已停止") : "加载中"}
                                </Tag>
                                <Button
                                    type={pollerStatus?.running ? "default" : "primary"}
                                    loading={startingPoller || stoppingPoller}
                                    onClick={async () => {
                                        if (pollerStatus?.running) {
                                            setStoppingPoller(true);
                                            try {
                                                await stopPoller();
                                                message.success("轮询已停止");
                                                void fetchPollerStatus();
                                            } catch (err: any) {
                                                message.error(err?.message || "停止轮询失败");
                                            } finally {
                                                setStoppingPoller(false);
                                            }
                                        } else {
                                            setStartingPoller(true);
                                            try {
                                                await startPoller();
                                                message.success("轮询已启动");
                                                void fetchPollerStatus();
                                            } catch (err: any) {
                                                message.error(err?.message || "启动轮询失败");
                                            } finally {
                                                setStartingPoller(false);
                                            }
                                        }
                                    }}
                                >
                                    {pollerStatus?.running ? "停止" : "启动"}
                                </Button>
                            </div>
                            <div className="flex items-center gap-4">
                                <span className="text-sm text-stone-500">轮询间隔(秒):</span>
                                <InputNumber min={1} value={pollerInterval} onChange={(v) => setPollerInterval(v ?? 30)} disabled={savingInterval} />
                                <Button loading={savingInterval} onClick={async () => {
                                    setSavingInterval(true);
                                    try {
                                        await saveWebhookConfig({ interval_seconds: pollerInterval });
                                        message.success("间隔已保存");
                                    } catch (err: any) {
                                        message.error(err?.message || "保存间隔失败");
                                    } finally {
                                        setSavingInterval(false);
                                    }
                                }}>
                                    保存间隔
                                </Button>
                            </div>
                            <div className="flex items-center gap-4">
                                <span className="text-sm text-stone-500">冷却时间(分钟):</span>
                                <InputNumber min={0} value={cooldownMinutes} onChange={(v) => setCooldownMinutes(v ?? 10)} disabled={savingCooldown} />
                                <Button loading={savingCooldown} onClick={async () => {
                                    setSavingCooldown(true);
                                    try {
                                        // Save cooldown to each platform's config
                                        for (const platform of WEBHOOK_PLATFORMS) {
                                            await saveWebhookConfig({ platform, cooldown_minutes: cooldownMinutes });
                                        }
                                        message.success("冷却时间已保存");
                                    } catch (err: any) {
                                        message.error(err?.message || "保存冷却时间失败");
                                    } finally {
                                        setSavingCooldown(false);
                                    }
                                }}>
                                    保存冷却
                                </Button>
                            </div>
                        </div>
                    </Card>
                    {/* Card 3: 推送日志 */}
                    <Card
                        title="推送日志"
                        extra={
                            <Button icon={<RefreshCw className="size-4" />} onClick={fetchWebhookLogs} loading={loadingLogs}>
                                刷新
                            </Button>
                        }
                    >
                        <Table
                            rowKey="id"
                            dataSource={webhookLogs}
                            columns={[
                                { title: "时间", dataIndex: "created_at", key: "created_at", width: 170, render: (val: string) => val ? new Date(val).toLocaleString("zh-CN") : "-" },
                                { title: "平台", dataIndex: "platform", key: "platform", width: 100, render: (p: string) => PLATFORM_LABELS[p] || p },
                                { title: "模型", dataIndex: "model_name", key: "model_name", width: 150, ellipsis: true },
                                {
                                    title: "状态",
                                    dataIndex: "status",
                                    key: "status",
                                    width: 80,
                                    render: (status: string) => {
                                        const labels: Record<string, string> = { down: "宕机", up: "恢复" };
                                        return <Tag color={status === "down" ? "red" : "green"}>{labels[status] || status}</Tag>;
                                    },
                                },
                                { title: "消息内容", dataIndex: "message", key: "message", width: 300, ellipsis: true },
                                {
                                    title: "推送结果",
                                    key: "success",
                                    width: 100,
                                    render: (_, record) => <Tag color={record.success ? "green" : "red"}>{record.success ? "成功" : "失败"}</Tag>,
                                },
                            ]}
                            loading={loadingLogs}
                            pagination={false}
                            scroll={{ x: 1000 }}
                        />
                    </Card>
                </Tabs.TabPane>
            </Tabs>

            {/* Channel Create/Edit Modal */}
            <Modal
                title={editingChannel ? "编辑渠道" : "新增渠道"}
                open={isChannelModalOpen}
                onCancel={() => setIsChannelModalOpen(false)}
                footer={[
                    <Button key="cancel" onClick={() => setIsChannelModalOpen(false)}>
                        取消
                    </Button>,
                    <Button key="submit" type="primary" disabled={!isSuperAdmin} onClick={() => channelForm.submit()}>
                        保存
                    </Button>,
                ]}
            >
                <Form form={channelForm} layout="vertical" onFinish={handleSaveChannel}>
                    <Form.Item name="name" label="渠道名称" rules={[{ required: true, message: "请输入渠道名称" }]}>
                        <Input placeholder="例如: 官方 OpenAI 渠道" disabled={!isSuperAdmin} />
                    </Form.Item>
                    <Form.Item name="base_url" label="接口地址 (Base URL)" rules={[{ required: true, message: "请输入接口 Base URL" }]}>
                        <Input placeholder="https://api.openai.com" disabled={!isSuperAdmin} />
                    </Form.Item>
                    <Form.Item
                        name="api_key"
                        label="API Key"
                        extra={editingChannel && editingChannel.has_key ? "已设置 API Key；留空表示使用现有 Key，输入则覆盖" : "配置需要输入 API Key"}
                        rules={[{ required: !editingChannel, message: "请输入 API Key" }]}
                    >
                        <Input.Password placeholder="sk-..." disabled={!isSuperAdmin} />
                    </Form.Item>
                    <Form.Item name="new_api_channel_id" label="New-API 渠道映射 ID (可选)" extra="对应 New-API 系统中该渠道 of ID，用于拉取指标数据">
                        <InputNumber min={0} className="w-full" placeholder="例如: 5" disabled={!isSuperAdmin} />
                    </Form.Item>
                    <Form.Item name="metrics_base_url" label="指标服务地址 (可选)" extra="为空则使用渠道 BaseUrl + /api。配置旧版 New-API 时可留空。">
                        <Input placeholder="https://old-api.example.com" disabled={!isSuperAdmin} />
                    </Form.Item>
                    <Form.Item name="enabled" valuePropName="checked" label="启用渠道">
                        <Switch disabled={!isSuperAdmin} />
                    </Form.Item>
                </Form>
            </Modal>

            {/* Model Management Panel (inline) */}
            {selectedChannel && (
                <Card
                    className="mt-4"
                    title={`模型管理 - ${selectedChannel.name}`}
                    extra={
                        <Space>
                            <Button
                                icon={<RefreshCw className="size-4" />}
                                onClick={() => handleSync(selectedChannel.id)}
                                loading={syncingChannelId === selectedChannel.id}
                                disabled={!isSuperAdmin}
                            >
                                同步模型
                            </Button>
                            <Tooltip title="根据模型名称自动推断能力（匹配 video/sora/omni → 视频+音频，image/seedance → 图片）">
                                <Button
                                    onClick={handleAutoDetect}
                                    disabled={!isSuperAdmin || models.length === 0}
                                >
                                    自动检测
                                </Button>
                            </Tooltip>
                            <Button
                                icon={<Save className="size-4" />}
                                onClick={handleSaveAll}
                                disabled={!isSuperAdmin || models.length === 0}
                                loading={savingAll}
                            >
                                保存全部
                            </Button>
                            <Button icon={<X className="size-4" />} onClick={closePanel}>
                                关闭
                            </Button>
                        </Space>
                    }
                >
                    {selectedChannel.sync_status === "failed" && (
                        <Alert
                            type="error"
                            showIcon
                            message="模型同步失败"
                            description={selectedChannel.sync_error || "同步失败，请检查渠道接口地址和 API Key 是否正确。"}
                            action={
                                <Button size="small" danger onClick={() => handleSync(selectedChannel.id)} loading={syncingChannelId === selectedChannel.id} disabled={!isSuperAdmin}>
                                    立即重试
                                </Button>
                            }
                            className="mb-4"
                        />
                    )}
                    <Table rowKey="id" columns={modelColumns} dataSource={models} loading={loadingModels} pagination={false} scroll={{ x: 1400 }} />
                </Card>
            )}

            {/* Model Metadata Edit Modal */}
            <Modal
                title={`编辑模型配置：${editingModel?.model_name || ""}`}
                open={isModelModalOpen}
                onCancel={() => setIsModelModalOpen(false)}
                footer={[
                    <Button key="cancel" onClick={() => setIsModelModalOpen(false)}>
                        取消
                    </Button>,
                    <Button key="submit" type="primary" disabled={!isSuperAdmin} onClick={() => modelForm.submit()}>
                        保存
                    </Button>,
                ]}
            >
                <Form form={modelForm} layout="vertical" onFinish={handleSaveModel}>
                    <Form.Item name="sort_order" label="排序权重" rules={[{ required: true, message: "请输入排序权重" }]}>
                        <InputNumber min={0} className="w-full" disabled={!isSuperAdmin} />
                    </Form.Item>

                    {editingModel?.capabilities.includes("image") && (
                        <>
                            <Form.Item name="image_generate_route" label="文生图接口路由">
                                <Select options={imageRouteOptions} disabled={!isSuperAdmin} />
                            </Form.Item>
                            <Form.Item name="image_edit_route" label="图生图接口路由">
                                <Select options={imageRouteOptions} disabled={!isSuperAdmin} />
                            </Form.Item>
                        </>
                    )}

                    {editingModel?.capabilities.includes("video") && (
                        <>
                            <Form.Item name="video_route" label="视频生成接口路由">
                                <Select options={videoRouteOptions} disabled={!isSuperAdmin} />
                            </Form.Item>
                            <Form.Item name="video_durations" label="可选视频时长 (逗号分隔)" help="多个时长用半角逗号分隔，例如: 5,10">
                                <Input placeholder="如: 5,10" disabled={!isSuperAdmin} />
                            </Form.Item>
                            <Form.Item name="video_customizable" valuePropName="checked" label="允许用户自定义视频时长">
                                <Switch disabled={!isSuperAdmin} />
                            </Form.Item>
                        </>
                    )}
                </Form>
            </Modal>

            {/* Pricing Scope Modal */}
            <PricingScopeModal
                open={pricingModal.open}
                channels={pricingModal.channels}
                onApplyGlobal={handlePricingApplyGlobal}
                onApplyLocal={handlePricingApplyLocal}
                onCancel={handlePricingCancel}
            />

            {/* Model Test Modal */}
            <Modal
                title={`模型测试 — ${testingModel?.model_name || ""}`}
                open={testModalOpen}
                onCancel={() => setTestModalOpen(false)}
                footer={
                    <Space>
                        <Button onClick={() => setTestModalOpen(false)}>关闭</Button>
                        <Button type="primary" icon={<Play className="size-4" />} loading={testLoading} onClick={handleRunTest}>
                            运行测试
                        </Button>
                    </Space>
                }
            >
                <div className="space-y-4">
                    <div>
                        <label className="text-sm text-stone-500 mb-1 block">生成类型</label>
                        <Select
                            value={testGeneration}
                            onChange={setTestGeneration}
                            style={{ width: 200 }}
                            options={[
                                { label: "文本", value: "text" },
                                { label: "图片", value: "image" },
                                { label: "视频", value: "video" },
                                { label: "音频", value: "audio" },
                            ]}
                        />
                    </div>

                    {testResult && (
                        <div className={`rounded-lg border p-3 text-sm ${testResult.success ? "border-green-200 bg-green-50 dark:border-green-800 dark:bg-green-950" : "border-red-200 bg-red-50 dark:border-red-800 dark:bg-red-950"}`}>
                            <div className="flex items-center gap-2 mb-2">
                                <Tag color={testResult.success ? "green" : "red"}>{testResult.success ? "成功" : "失败"}</Tag>
                                <span className="text-stone-500">HTTP {testResult.status_code}</span>
                                <span className="text-stone-500">{testResult.response_time_ms}ms</span>
                            </div>
                            {testResult.route && (
                                <div className="mb-1">
                                    <span className="text-stone-400">路由: </span>
                                    <code className="text-xs bg-stone-100 dark:bg-stone-800 px-1 rounded">{testResult.method} {testResult.path}</code>
                                </div>
                            )}
                            {testResult.error_message && (
                                <div className="mt-2">
                                    <span className="text-red-500 font-medium">错误: </span>
                                    <pre className="mt-1 whitespace-pre-wrap text-xs font-mono bg-stone-100 dark:bg-stone-800 p-2 rounded max-h-40 overflow-auto">{testResult.error_message}</pre>
                                </div>
                            )}
                            {testResult.response_body && (
                                <div className="mt-2">
                                    <span className="text-stone-500 font-medium">响应: </span>
                                    <pre className="mt-1 whitespace-pre-wrap text-xs font-mono bg-stone-100 dark:bg-stone-800 p-2 rounded max-h-60 overflow-auto">{testResult.response_body}</pre>
                                </div>
                            )}
                        </div>
                    )}
                </div>
            </Modal>

            {/* Webhook Test Modal */}
            <Modal
                title={`测试推送 - ${PLATFORM_LABELS[webhookTestPlatform] || webhookTestPlatform}`}
                open={webhookTestModalOpen}
                onCancel={() => setWebhookTestModalOpen(false)}
                footer={
                    <Space>
                        <Button onClick={() => setWebhookTestModalOpen(false)}>关闭</Button>
                        <Button type="primary" loading={webhookTestSending} onClick={handleTestSend}>
                            发送测试
                        </Button>
                    </Space>
                }
            >
                <div className="space-y-4">
                    <div>
                        <label className="text-sm text-stone-500 mb-1 block">测试消息</label>
                        <Input.TextArea
                            rows={4}
                            value={webhookTestMessage}
                            onChange={(e) => setWebhookTestMessage(e.target.value)}
                            placeholder="输入要发送的测试消息内容"
                        />
                    </div>
                    {webhookTestSendResult && (
                        <div className={`rounded-lg border p-3 text-sm ${webhookTestSendResult.success ? "border-green-200 bg-green-50 dark:border-green-800 dark:bg-green-950" : "border-red-200 bg-red-50 dark:border-red-800 dark:bg-red-950"}`}>
                            <Tag color={webhookTestSendResult.success ? "green" : "red"}>
                                {webhookTestSendResult.success ? "发送成功" : "发送失败"}
                            </Tag>
                            {webhookTestSendResult.error && (
                                <pre className="mt-2 whitespace-pre-wrap text-xs font-mono bg-stone-100 dark:bg-stone-800 p-2 rounded max-h-40 overflow-auto">
                                    {webhookTestSendResult.error}
                                </pre>
                            )}
                        </div>
                    )}
                </div>
            </Modal>
        </div>
    );
}
