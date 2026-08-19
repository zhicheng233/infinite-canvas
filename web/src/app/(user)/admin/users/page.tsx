"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { App, Button, Form, Input, Modal, Table, Tag, Typography } from "antd";
import type { ColumnsType } from "antd/es/table";
import { KeyRound, Trash2 } from "lucide-react";
import { deleteUser, listUsersWithBalance, resetUserPassword, type UserWithBalance } from "@/services/api/admin";
import { getPasswordPolicyError } from "@/lib/auth-policy";
import { useUserStore } from "@/stores/use-user-store";

const { Title } = Typography;

const roleLabels: Record<string, string> = {
    super_admin: "超级管理员",
    tenant_admin: "租户管理员",
    user: "普通用户",
};

const roleColors: Record<string, string> = {
    super_admin: "red",
    tenant_admin: "blue",
    user: "default",
};

const statusLabels: Record<string, string> = {
    active: "正常",
    inactive: "禁用",
};

export default function AdminUsersPage() {
    const { message, modal } = App.useApp();
    const currentUser = useUserStore((state) => state.user);
    const [users, setUsers] = useState<UserWithBalance[]>([]);
    const [loading, setLoading] = useState(false);
    const [pagination, setPagination] = useState({ current: 1, pageSize: 20, total: 0 });
    const [keywordInput, setKeywordInput] = useState("");
    const [keyword, setKeyword] = useState("");
    const [resetTarget, setResetTarget] = useState<UserWithBalance | null>(null);
    const [resetting, setResetting] = useState(false);
    const [resetForm] = Form.useForm<{ new_password: string; confirm_password: string }>();
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

    const closeResetModal = () => {
        setResetTarget(null);
        resetForm.resetFields();
    };

    const resetPassword = async () => {
        if (!resetTarget) return;
        let values: { new_password: string; confirm_password: string };
        try {
            values = await resetForm.validateFields();
        } catch (error) {
            return;
        }
        setResetting(true);
        try {
            await resetUserPassword(resetTarget.id, values.new_password);
            message.success(`用户 ${resetTarget.username} 的密码已重置`);
            closeResetModal();
            await fetchUsers(pagination.current, pagination.pageSize, keyword);
        } catch (error) {
            message.error(error instanceof Error && error.message ? error.message : "重置密码失败");
        } finally {
            setResetting(false);
        }
    };

    const confirmDelete = (target: UserWithBalance) => {
        modal.confirm({
            title: "永久删除用户？",
            content: `确定永久删除用户“${target.username}”吗？该账号的积分记录、画布、生成记录、充值订单和模型调用日志将被彻底删除，且无法恢复。`,
            okText: "永久删除",
            cancelText: "取消",
            okType: "danger",
            focusable: { autoFocusButton: "cancel" },
            onOk: async () => {
                try {
                    await deleteUser(target.id);
                    message.success(`用户 ${target.username} 已永久删除`);
                    const nextPage = users.length === 1 && pagination.current > 1 ? pagination.current - 1 : pagination.current;
                    await fetchUsers(nextPage, pagination.pageSize, keyword);
                } catch (error) {
                    message.error(error instanceof Error && error.message ? error.message : "删除用户失败");
                    throw error;
                }
            },
        });
    };

    const columns: ColumnsType<UserWithBalance> = [
        { title: "ID", dataIndex: "id", key: "id", width: 80 },
        { title: "用户名", dataIndex: "username", key: "username" },
        { title: "显示名称", dataIndex: "display_name", key: "display_name" },
        {
            title: "角色",
            dataIndex: "role",
            key: "role",
            render: (role: string) => <Tag color={roleColors[role] || "default"}>{roleLabels[role] || role}</Tag>,
        },
        {
            title: "状态",
            dataIndex: "status",
            key: "status",
            render: (status: string) => <Tag color={status === "active" ? "green" : "red"}>{statusLabels[status] || status}</Tag>,
        },
        {
            title: "积分余额",
            dataIndex: "balance",
            key: "balance",
            width: 120,
            render: (balance: number) => <span className="font-mono font-semibold text-blue-600">{balance}</span>,
        },
        {
            title: "操作",
            key: "actions",
            width: 180,
            render: (_, record) => {
                if (String(record.id) === currentUser?.id) return null;
                return (
                    <div className="flex items-center gap-2">
                        <Button size="small" icon={<KeyRound className="size-3.5" />} onClick={() => setResetTarget(record)}>
                            重置密码
                        </Button>
                        <Button danger size="small" icon={<Trash2 className="size-3.5" />} onClick={() => confirmDelete(record)}>
                            删除
                        </Button>
                    </div>
                );
            },
        },
    ];

    return (
        <div>
            <Title level={4} className="!mb-4">
                用户管理
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

            <Modal title={`重置 ${resetTarget?.username ?? "用户"} 的密码`} open={Boolean(resetTarget)} onCancel={closeResetModal} onOk={resetPassword} confirmLoading={resetting} destroyOnHidden okText="确认重置" cancelText="取消">
                <Form form={resetForm} layout="vertical" preserve={false} className="mt-4">
                    <Form.Item
                        name="new_password"
                        label="新密码"
                        rules={[
                            { required: true, message: "请输入新密码" },
                            {
                                validator: (_, value?: string) => {
                                    const error = getPasswordPolicyError(value ?? "");
                                    return error ? Promise.reject(new Error(error)) : Promise.resolve();
                                },
                            },
                        ]}
                    >
                        <Input.Password autoComplete="new-password" placeholder="至少 8 位，且包含字母和数字" />
                    </Form.Item>
                    <Form.Item
                        name="confirm_password"
                        label="确认新密码"
                        dependencies={["new_password"]}
                        rules={[
                            { required: true, message: "请再次输入新密码" },
                            ({ getFieldValue }) => ({ validator: (_, value?: string) => (value === getFieldValue("new_password") ? Promise.resolve() : Promise.reject(new Error("两次输入的新密码不一致"))) }),
                        ]}
                    >
                        <Input.Password autoComplete="new-password" placeholder="再次输入新密码" />
                    </Form.Item>
                </Form>
            </Modal>
        </div>
    );
}
