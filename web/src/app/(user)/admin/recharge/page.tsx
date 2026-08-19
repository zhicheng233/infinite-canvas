"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { App, Button, Form, Input, InputNumber, Modal, Segmented, Table, Tag, Typography } from "antd";
import type { ColumnsType } from "antd/es/table";
import { Minus, Plus, Zap } from "lucide-react";
import { listUsersWithBalance, type UserWithBalance } from "@/services/api/admin";
import { adjustCredits, buildCreditAdjustmentRequest, getCreditAdjustmentPreview, type AdjustMode } from "@/services/api/pricing";

const { Title } = Typography;

type AdjustmentFormValues = {
    readonly note?: string;
};

const adjustmentModeOptions = [
    { label: "增加", value: "add" },
    { label: "扣减", value: "deduct" },
    { label: "设为目标值", value: "set" },
] satisfies readonly { readonly label: string; readonly value: AdjustMode }[];

const adjustmentModeLabels: Record<AdjustMode, string> = {
    add: "增加",
    deduct: "扣减",
    set: "设为目标值",
};

export default function AdminRechargePage() {
    const { message } = App.useApp();
    const [users, setUsers] = useState<UserWithBalance[]>([]);
    const [loading, setLoading] = useState(false);
    const [modalOpen, setModalOpen] = useState(false);
    const [selectedUser, setSelectedUser] = useState<UserWithBalance | null>(null);
    const [adjustMode, setAdjustMode] = useState<AdjustMode>("add");
    const [adjustmentValue, setAdjustmentValue] = useState<number | null>(null);
    const [saving, setSaving] = useState(false);
    const [pagination, setPagination] = useState({ current: 1, pageSize: 20, total: 0 });
    const [keywordInput, setKeywordInput] = useState("");
    const [keyword, setKeyword] = useState("");
    const [form] = Form.useForm<AdjustmentFormValues>();
    const requestSequence = useRef(0);

    const fetchUsers = useCallback(
        async (page = 1, pageSize = 20, searchKeyword = "") => {
            const requestID = ++requestSequence.current;
            setLoading(true);
            try {
                const data = await listUsersWithBalance(page, pageSize, searchKeyword);
                if (requestID !== requestSequence.current) return;
                setUsers(data.items);
                setPagination({ current: data.page, pageSize: data.page_size, total: data.total });
            } catch (error: unknown) {
                if (requestID !== requestSequence.current) return;
                message.error(error instanceof Error && error.message ? error.message : "获取用户列表失败");
            } finally {
                if (requestID === requestSequence.current) setLoading(false);
            }
        },
        [message],
    );

    useEffect(() => {
        void fetchUsers();
        return () => {
            requestSequence.current += 1;
        };
    }, [fetchUsers]);

    const searchUsers = (value: string) => {
        const normalizedKeyword = value.trim();
        setKeywordInput(normalizedKeyword);
        setKeyword(normalizedKeyword);
        setPagination((current) => ({ ...current, current: 1 }));
        void fetchUsers(1, pagination.pageSize, normalizedKeyword);
    };

    const openAdjust = (user: UserWithBalance, mode: AdjustMode) => {
        setSelectedUser(user);
        setAdjustMode(mode);
        setAdjustmentValue(null);
        form.resetFields();
        setModalOpen(true);
    };

    const handleAdjust = async () => {
        if (!selectedUser || !preview.valid || adjustmentValue === null) return;
        try {
            const values = await form.validateFields();
            setSaving(true);
            const result = await adjustCredits(buildCreditAdjustmentRequest(adjustMode, selectedUser.id, adjustmentValue, values.note));
            message.success(`${adjustmentModeLabels[adjustMode]}成功！用户 ${selectedUser.username} 当前余额：${result.balance} 积分`);
            setModalOpen(false);
            void fetchUsers(pagination.current, pagination.pageSize, keyword);
        } catch (error) {
            if (error instanceof Error) message.error(error.message);
        } finally {
            setSaving(false);
        }
    };

    const columns: ColumnsType<UserWithBalance> = [
        { title: "ID", dataIndex: "id", key: "id", width: 80 },
        { title: "用户名", dataIndex: "username", key: "username" },
        { title: "显示名称", dataIndex: "display_name", key: "display_name" },
        {
            title: "积分余额",
            dataIndex: "balance",
            key: "balance",
            width: 120,
            render: (balance: number) => (
                <span className="inline-flex items-center gap-1 font-mono font-semibold text-amber-600">
                    <Zap className="size-3.5 fill-amber-400 text-amber-400" />
                    {balance.toLocaleString()}
                </span>
            ),
        },
        {
            title: "状态",
            dataIndex: "status",
            key: "status",
            width: 80,
            render: (status: string) => <Tag color={status === "active" ? "green" : "red"}>{status === "active" ? "正常" : "禁用"}</Tag>,
        },
        {
            title: "操作",
            key: "actions",
            width: 160,
            render: (_, record) => (
                <div className="flex items-center gap-2">
                    <Button type="primary" size="small" icon={<Plus className="size-3" />} onClick={() => openAdjust(record, "add")}>
                        增加
                    </Button>
                    <Button danger size="small" icon={<Minus className="size-3" />} onClick={() => openAdjust(record, "deduct")} disabled={record.balance <= 0}>
                        扣减
                    </Button>
                </div>
            ),
        },
    ];

    const modalActionText = adjustmentModeLabels[adjustMode];
    const currentBalance = selectedUser?.balance ?? 0;
    const preview = getCreditAdjustmentPreview(adjustMode, currentBalance, adjustmentValue);

    return (
        <div>
            <Title level={4} className="!mb-4">
                积分管理
            </Title>
            <Input.Search allowClear enterButton="搜索" className="!mb-4 !max-w-sm" placeholder="输入用户 ID 或用户名" value={keywordInput} onChange={(event) => setKeywordInput(event.target.value)} onSearch={searchUsers} />
            <Table
                rowKey="id"
                columns={columns}
                dataSource={users}
                loading={loading}
                pagination={{
                    ...pagination,
                    showSizeChanger: true,
                    showTotal: (total) => `共 ${total} 个用户`,
                    onChange: (page, pageSize) => fetchUsers(page, pageSize, keyword),
                }}
            />

            <Modal
                title={`为 ${selectedUser?.username} ${modalActionText}积分（当前余额：${currentBalance}）`}
                open={modalOpen}
                onCancel={() => setModalOpen(false)}
                onOk={handleAdjust}
                confirmLoading={saving}
                destroyOnHidden
                okButtonProps={{ disabled: !preview.valid }}
                okText={modalActionText}
            >
                <Form form={form} layout="vertical" className="mt-4">
                    <Form.Item label="调整方式">
                        <Segmented
                            block
                            options={adjustmentModeOptions}
                            value={adjustMode}
                            onChange={(value) => {
                                if (value === "add" || value === "deduct" || value === "set") {
                                    setAdjustMode(value);
                                    setAdjustmentValue(null);
                                }
                            }}
                        />
                    </Form.Item>
                    <Form.Item label={adjustMode === "set" ? "目标积分" : `${modalActionText}积分`}>
                        <InputNumber
                            className="w-full"
                            min={adjustMode === "set" ? 0 : 1}
                            placeholder={adjustMode === "set" ? "例如: 1000" : adjustMode === "deduct" ? `最多可扣减 ${currentBalance}` : "例如: 1000"}
                            precision={0}
                            value={adjustmentValue}
                            onChange={(value) => setAdjustmentValue(typeof value === "number" ? value : null)}
                        />
                    </Form.Item>
                    <div className={`mb-6 rounded-lg border px-3 py-2 text-sm ${preview.valid ? "border-stone-200 text-stone-700 dark:border-stone-700 dark:text-stone-200" : "border-amber-200 text-amber-700 dark:border-amber-900 dark:text-amber-300"}`}>
                        {preview.text}
                    </div>
                    <Form.Item name="note" label="备注">
                        <Input.TextArea rows={2} placeholder={`${modalActionText}说明（可选）`} />
                    </Form.Item>
                </Form>
            </Modal>
        </div>
    );
}
