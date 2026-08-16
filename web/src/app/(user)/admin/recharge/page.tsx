"use client";

import { useEffect, useState, useCallback } from "react";
import { Table, Button, Modal, Form, InputNumber, Input, Typography, App, Tag } from "antd";
import type { ColumnsType } from "antd/es/table";
import { Minus, Plus, Zap } from "lucide-react";
import { listUsersWithBalance, type UserWithBalance } from "@/services/api/admin";
import { adjustCredits } from "@/services/api/pricing";

const { Title } = Typography;
type AdjustMode = "add" | "deduct";

export default function AdminRechargePage() {
    const { message } = App.useApp();
    const [users, setUsers] = useState<UserWithBalance[]>([]);
    const [loading, setLoading] = useState(false);
    const [modalOpen, setModalOpen] = useState(false);
    const [selectedUser, setSelectedUser] = useState<UserWithBalance | null>(null);
    const [adjustMode, setAdjustMode] = useState<AdjustMode>("add");
    const [saving, setSaving] = useState(false);
    const [pagination, setPagination] = useState({ current: 1, pageSize: 20, total: 0 });
    const [form] = Form.useForm();

    const fetchUsers = useCallback(
        async (page = 1, pageSize = 20) => {
            setLoading(true);
            try {
                const data = await listUsersWithBalance(page, pageSize);
                setUsers(data.items);
                setPagination({ current: data.page, pageSize: data.page_size, total: data.total });
            } catch (err: any) {
                message.error(err?.message || "获取用户列表失败");
            } finally {
                setLoading(false);
            }
        },
        [message],
    );

    useEffect(() => {
        fetchUsers();
    }, [fetchUsers]);

    const openAdjust = (user: UserWithBalance, mode: AdjustMode) => {
        setSelectedUser(user);
        setAdjustMode(mode);
        form.resetFields();
        setModalOpen(true);
    };

    const handleAdjust = async () => {
        try {
            const values = await form.validateFields();
            setSaving(true);
            const amount = Number(values.amount) || 0;
            const result = await adjustCredits({
                user_id: selectedUser!.id,
                amount: adjustMode === "deduct" ? -amount : amount,
                note: values.note,
            });
            message.success(`${adjustMode === "deduct" ? "扣减" : "增加"}成功！用户 ${selectedUser!.username} 当前余额：${result.balance} 积分`);
            setModalOpen(false);
            fetchUsers(pagination.current, pagination.pageSize);
        } catch (err: any) {
            if (err?.message) message.error(err.message);
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

    const modalActionText = adjustMode === "deduct" ? "扣减" : "增加";
    const currentBalance = selectedUser?.balance ?? 0;

    return (
        <div>
            <Title level={4} className="!mb-4">
                积分管理
            </Title>
            <Table
                rowKey="id"
                columns={columns}
                dataSource={users}
                loading={loading}
                pagination={{
                    ...pagination,
                    showSizeChanger: true,
                    showTotal: (total) => `共 ${total} 个用户`,
                    onChange: (page, pageSize) => fetchUsers(page, pageSize),
                }}
            />

            <Modal title={`为 ${selectedUser?.username} ${modalActionText}积分（当前余额：${currentBalance}）`} open={modalOpen} onCancel={() => setModalOpen(false)} onOk={handleAdjust} confirmLoading={saving} destroyOnClose okText={modalActionText}>
                <Form form={form} layout="vertical" className="mt-4">
                    <Form.Item name="amount" label={`${modalActionText}积分`} rules={[{ required: true, message: `请输入${modalActionText}积分` }]}>
                        <InputNumber min={1} max={adjustMode === "deduct" ? currentBalance : undefined} className="w-full" placeholder={adjustMode === "deduct" ? `最多可扣减 ${currentBalance}` : "例如: 1000"} />
                    </Form.Item>
                    <Form.Item name="note" label="备注">
                        <Input.TextArea rows={2} placeholder={`${modalActionText}说明（可选）`} />
                    </Form.Item>
                </Form>
            </Modal>
        </div>
    );
}
