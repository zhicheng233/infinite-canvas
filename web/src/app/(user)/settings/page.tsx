"use client";

import { useEffect, useState } from "react";
import { App, Avatar, Button, Card, Form, Input, Tabs } from "antd";
import { Key, User } from "lucide-react";

import { changePassword, updateProfile } from "@/services/api/auth";
import { getPasswordPolicyError } from "@/lib/auth-policy";
import { useUserStore } from "@/stores/use-user-store";

export default function SettingsPage() {
    const user = useUserStore((state) => state.user);

    const tabItems = [
        { key: "profile", label: "个人资料", icon: <User className="size-4" /> },
        { key: "password", label: "修改密码", icon: <Key className="size-4" /> },
    ];

    return (
        <main className="mx-auto max-w-3xl overflow-y-auto px-6 py-8">
            <h1 className="mb-6 text-2xl font-semibold text-stone-950 dark:text-stone-100">个人中心</h1>
            <Tabs
                items={tabItems.map((tab) => ({
                    key: tab.key,
                    label: (
                        <span className="flex items-center gap-2">
                            {tab.icon}
                            {tab.label}
                        </span>
                    ),
                    children: tab.key === "profile" ? <ProfileTab /> : <PasswordTab />,
                }))}
            />
            {!user ? null : null}
        </main>
    );
}

function ProfileTab() {
    const user = useUserStore((state) => state.user);
    const fetchUser = useUserStore((state) => state.fetchUser);
    const { message } = App.useApp();
    const [loading, setLoading] = useState(false);
    const [form] = Form.useForm();

    useEffect(() => {
        if (user) {
            form.setFieldsValue({
                display_name: user.displayName || user.username,
                avatar_url: user.avatarUrl || "",
            });
        }
    }, [user, form]);

    const handleSave = async (values: { display_name: string; avatar_url: string }) => {
        setLoading(true);
        try {
            await updateProfile(values);
            await fetchUser();
            message.success("个人资料已更新");
        } catch (err: any) {
            message.error(err?.message || "更新失败");
        } finally {
            setLoading(false);
        }
    };

    return (
        <Card>
            <div className="mb-6 flex items-center gap-4">
                <Avatar size={64} src={user?.avatarUrl || undefined}>
                    {(user?.displayName || user?.username || "U")[0]?.toUpperCase()}
                </Avatar>
                <div>
                    <div className="text-lg font-semibold">{user?.displayName || user?.username}</div>
                    <div className="text-sm text-stone-500">@{user?.username}</div>
                    <div className="text-xs text-stone-400">{user?.role === "super_admin" ? "超级管理员" : user?.role === "tenant_admin" ? "管理员" : "普通用户"}</div>
                </div>
            </div>
            <Form form={form} layout="vertical" onFinish={handleSave}>
                <Form.Item name="display_name" label="显示名称" rules={[{ required: true, message: "请输入显示名称" }]}>
                    <Input placeholder="输入昵称" />
                </Form.Item>
                <Form.Item name="avatar_url" label="头像链接">
                    <Input placeholder="https://example.com/avatar.jpg（可选）" />
                </Form.Item>
                <Button type="primary" htmlType="submit" loading={loading}>
                    保存修改
                </Button>
            </Form>
        </Card>
    );
}

function PasswordTab() {
    const { message } = App.useApp();
    const [loading, setLoading] = useState(false);
    const [form] = Form.useForm();

    const handleChange = async (values: { old_password: string; new_password: string; confirm_password: string }) => {
        if (values.new_password !== values.confirm_password) {
            message.error("两次输入的新密码不一致");
            return;
        }
        setLoading(true);
        try {
            await changePassword({
                old_password: values.old_password,
                new_password: values.new_password,
            });
            message.success("密码修改成功，请重新登录");
            form.resetFields();
        } catch (err: any) {
            message.error(err?.message || "修改失败");
        } finally {
            setLoading(false);
        }
    };

    return (
        <Card>
            <Form form={form} layout="vertical" onFinish={handleChange} style={{ maxWidth: 420 }}>
                <Form.Item name="old_password" label="当前密码" rules={[{ required: true, message: "请输入当前密码" }]}>
                    <Input.Password placeholder="当前密码" />
                </Form.Item>
                <Form.Item
                    name="new_password"
                    label="新密码"
                    rules={[
                        {
                            validator: (_, value?: string) => {
                                const error = getPasswordPolicyError(value ?? "");
                                return error ? Promise.reject(new Error(error)) : Promise.resolve();
                            },
                        },
                    ]}
                >
                    <Input.Password placeholder="新密码" />
                </Form.Item>
                <Form.Item name="confirm_password" label="确认新密码" rules={[{ required: true, message: "请再次输入新密码" }]}>
                    <Input.Password placeholder="确认新密码" />
                </Form.Item>
                <Button type="primary" htmlType="submit" loading={loading}>
                    修改密码
                </Button>
            </Form>
        </Card>
    );
}
