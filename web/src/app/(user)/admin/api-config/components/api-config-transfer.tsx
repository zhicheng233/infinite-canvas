"use client";

import { useRef, useState } from "react";
import { Alert, App, Button, Form, Input, List, Modal, Space, Table, Typography } from "antd";
import { Download, FileJson, Upload } from "lucide-react";
import { saveAs } from "file-saver";

import {
    exportApiConfig,
    importApiConfig,
    previewApiConfigImport,
    readApiConfigTransferFile,
    type ApiConfigTransferEnvelope,
    type ApiConfigTransferResult,
    type ApiConfigTransferStats,
} from "@/services/api/api-config-transfer";

type ApiConfigTransferProps = {
    readonly disabled: boolean;
    readonly onImported: () => Promise<void>;
};

type ExportFormValues = {
    readonly password: string;
    readonly confirmPassword: string;
};

const resourceLabels: Record<keyof ApiConfigTransferStats, string> = {
    channels: "渠道",
    models: "模型",
    pricing: "定价",
    merge_groups: "合并组",
    video_config_presets: "视频配置预设",
    auto_routing_pools: "智能路由池",
};

export function ApiConfigTransfer({ disabled, onImported }: ApiConfigTransferProps) {
    const { message } = App.useApp();
    const [exportOpen, setExportOpen] = useState(false);
    const [exporting, setExporting] = useState(false);
    const [exportForm] = Form.useForm<ExportFormValues>();
    const [importOpen, setImportOpen] = useState(false);
    const [importing, setImporting] = useState(false);
    const [previewing, setPreviewing] = useState(false);
    const [password, setPassword] = useState("");
    const [fileName, setFileName] = useState("");
    const [envelope, setEnvelope] = useState<ApiConfigTransferEnvelope>();
    const [preview, setPreview] = useState<ApiConfigTransferResult>();
    const fileInputRef = useRef<HTMLInputElement>(null);

    const closeExport = () => {
        setExportOpen(false);
        exportForm.resetFields();
    };

    const handleExport = async ({ password: nextPassword }: ExportFormValues) => {
        setExporting(true);
        try {
            const result = await exportApiConfig(nextPassword);
            saveAs(new Blob([JSON.stringify(result.envelope, null, 2)], { type: "application/json;charset=utf-8" }), result.file_name);
            closeExport();
            if (result.warnings.length > 0) message.warning(`配置已导出，${result.warnings.length} 项无效引用已跳过`);
            else message.success("模型 API 配置已导出");
        } catch (error) {
            message.error(errorMessage(error, "导出配置失败"));
        } finally {
            setExporting(false);
        }
    };

    const resetImport = () => {
        setImportOpen(false);
        setPassword("");
        setFileName("");
        setEnvelope(undefined);
        setPreview(undefined);
        if (fileInputRef.current) fileInputRef.current.value = "";
    };

    const handleFileChange = async (file: File | undefined) => {
        setPreview(undefined);
        setEnvelope(undefined);
        setFileName(file?.name || "");
        if (!file) return;
        try {
            setEnvelope(await readApiConfigTransferFile(file));
        } catch (error) {
            setFileName("");
            if (fileInputRef.current) fileInputRef.current.value = "";
            message.error(errorMessage(error, "读取配置文件失败"));
        }
    };

    const handlePreview = async () => {
        if (!envelope) {
            message.warning("请选择配置文件");
            return;
        }
        if ([...password].length < 8) {
            message.warning("密码至少需要 8 个字符");
            return;
        }
        setPreviewing(true);
        try {
            setPreview(await previewApiConfigImport(password, envelope));
        } catch (error) {
            message.error(errorMessage(error, "配置预览失败"));
        } finally {
            setPreviewing(false);
        }
    };

    const handleImport = async () => {
        if (!envelope || !preview) return;
        setImporting(true);
        try {
            const result = await completeConfigImport(() => importApiConfig(password, envelope), onImported);
            resetImport();
            const skipped = totalSkipped(result.stats);
            if (skipped > 0) message.warning(`配置已导入，${skipped} 项冲突已跳过`);
            else message.success("模型 API 配置已导入");
        } catch (error) {
            message.error(errorMessage(error, "导入配置失败"));
        } finally {
            setImporting(false);
        }
    };

    return (
        <>
            <Space>
                <Button icon={<Upload className="size-4" />} disabled={disabled} onClick={() => setImportOpen(true)}>
                    导入配置
                </Button>
                <Button icon={<Download className="size-4" />} disabled={disabled} onClick={() => setExportOpen(true)}>
                    导出配置
                </Button>
            </Space>

            <Modal title="导出模型 API 配置" open={exportOpen} confirmLoading={exporting} okText="加密并导出" onCancel={closeExport} onOk={() => exportForm.submit()} destroyOnHidden>
                <Alert className="mb-4" type="warning" showIcon message="请妥善保管导出密码" description="配置文件包含渠道 API Key，整个文件会使用此密码加密。密码遗失后无法恢复。" />
                <Form form={exportForm} layout="vertical" preserve={false} onFinish={handleExport}>
                    <Form.Item name="password" label="导出密码" rules={[{ required: true, message: "请输入导出密码" }, { min: 8, message: "密码至少需要 8 个字符" }]}>
                        <Input.Password autoComplete="new-password" />
                    </Form.Item>
                    <Form.Item
                        name="confirmPassword"
                        label="确认密码"
                        dependencies={["password"]}
                        rules={[
                            { required: true, message: "请再次输入密码" },
                            ({ getFieldValue }) => ({ validator: (_, value) => (!value || value === getFieldValue("password") ? Promise.resolve() : Promise.reject(new Error("两次输入的密码不一致"))) }),
                        ]}
                    >
                        <Input.Password autoComplete="new-password" />
                    </Form.Item>
                </Form>
            </Modal>

            <Modal
                title="导入模型 API 配置"
                open={importOpen}
                width={720}
                destroyOnHidden
                onCancel={resetImport}
                footer={
                    <Space>
                        <Button onClick={resetImport}>取消</Button>
                        {preview ? (
                            <Button type="primary" loading={importing} onClick={handleImport}>
                                确认导入
                            </Button>
                        ) : (
                            <Button type="primary" loading={previewing} onClick={handlePreview}>
                                预览变更
                            </Button>
                        )}
                    </Space>
                }
            >
                <div className="space-y-4">
                    <input ref={fileInputRef} className="hidden" type="file" accept="application/json,.json" onChange={(event) => void handleFileChange(event.target.files?.[0])} />
                    <div className="flex flex-wrap items-center gap-2">
                        <Button icon={<FileJson className="size-4" />} onClick={() => fileInputRef.current?.click()}>
                            选择配置文件
                        </Button>
                        <Typography.Text type={fileName ? undefined : "secondary"}>{fileName || "未选择文件"}</Typography.Text>
                    </div>
                    <div>
                        <Typography.Text className="mb-1 block">导出密码</Typography.Text>
                        <Input.Password
                            value={password}
                            autoComplete="current-password"
                            onChange={(event) => {
                                setPassword(event.target.value);
                                setPreview(undefined);
                            }}
                            onPressEnter={() => !preview && void handlePreview()}
                        />
                    </div>
                    {preview ? <ImportPreview result={preview} /> : null}
                </div>
            </Modal>
        </>
    );
}

export async function completeConfigImport(importer: () => Promise<ApiConfigTransferResult>, onImported: () => Promise<void>) {
    const result = await importer();
    await onImported();
    return result;
}

function ImportPreview({ result }: { readonly result: ApiConfigTransferResult }) {
    const rows = (Object.keys(resourceLabels) as Array<keyof ApiConfigTransferStats>).map((key) => ({ key, label: resourceLabels[key], ...result.stats[key] }));
    return (
        <div className="space-y-3">
            <Table
                size="small"
                pagination={false}
                rowKey="key"
                dataSource={rows}
                columns={[
                    { title: "配置类型", dataIndex: "label" },
                    { title: "新增", dataIndex: "create", width: 80 },
                    { title: "更新", dataIndex: "update", width: 80 },
                    { title: "跳过", dataIndex: "skip", width: 80 },
                ]}
            />
            {result.conflicts.length > 0 ? (
                <Alert
                    type="warning"
                    showIcon
                    message={`${result.conflicts.length} 项冲突不会导入`}
                    description={
                        <List
                            className="max-h-48 overflow-auto"
                            size="small"
                            dataSource={result.conflicts}
                            renderItem={(item) => (
                                <List.Item>
                                    <Typography.Text className="break-all">
                                        {item.identifier || resourceLabel(item.resource)}：{item.reason}
                                    </Typography.Text>
                                </List.Item>
                            )}
                        />
                    }
                />
            ) : (
                <Alert type="success" showIcon message="配置校验通过，可以导入" />
            )}
        </div>
    );
}

function resourceLabel(resource: string) {
    const labels: Record<string, string> = { channel: "渠道", model: "模型", pricing: "定价", merge_group: "合并组", video_preset: "视频配置预设" };
    return labels[resource] || resource;
}

function totalSkipped(stats: ApiConfigTransferStats) {
    return Object.values(stats).reduce((total, item) => total + item.skip, 0);
}

function errorMessage(error: unknown, fallback: string) {
    return error instanceof Error && error.message ? error.message : fallback;
}
