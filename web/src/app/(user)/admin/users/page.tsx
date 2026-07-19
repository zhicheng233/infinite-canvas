"use client";

import { useEffect, useState, useCallback } from "react";
import { Table, Tag, Typography, App } from "antd";
import type { ColumnsType } from "antd/es/table";
import { listUsersWithBalance, type UserWithBalance } from "@/services/api/admin";

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
    const { message } = App.useApp();
    const [users, setUsers] = useState<UserWithBalance[]>([]);
    const [loading, setLoading] = useState(false);
    const [pagination, setPagination] = useState({ current: 1, pageSize: 20, total: 0 });

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
    ];

    return (
        <div>
            <Title level={4} className="!mb-4">
                用户管理
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
        </div>
    );
}
