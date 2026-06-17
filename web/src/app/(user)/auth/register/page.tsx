"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { Button, Form, Input, App } from "antd";
import { register } from "@/services/api/auth";
import { setStoredToken } from "@/services/api/client";
import { useUserStore } from "@/stores/use-user-store";

export default function RegisterPage() {
  const router = useRouter();
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);
  const fetchUser = useUserStore((s) => s.fetchUser);

  const onFinish = async (values: { tenant_name?: string; username: string; password: string; confirm: string }) => {
    if (values.password !== values.confirm) {
      message.error("两次密码不一致");
      return;
    }
    setLoading(true);
    try {
      const result = await register({
        tenant_name: values.tenant_name || undefined,
        username: values.username,
        password: values.password,
      });
      setStoredToken(result.token);
      await fetchUser();
      message.success("注册成功");
      router.replace("/");
    } catch (err: any) {
      message.error(err?.message || "注册失败");
    } finally {
      setLoading(false);
    }
  };

  return (
    <main className="flex min-h-full items-center justify-center bg-background px-4">
      <div className="w-full max-w-sm">
        <h1 className="mb-8 text-center text-2xl font-semibold text-stone-950 dark:text-stone-100">注册</h1>
        <Form layout="vertical" onFinish={onFinish} autoComplete="off">
          <Form.Item name="tenant_name">
            <Input placeholder="团队名称（可选，默认与用户名相同）" size="large" />
          </Form.Item>
          <Form.Item name="username" rules={[{ required: true, message: "请输入用户名" }]}>
            <Input placeholder="用户名" size="large" />
          </Form.Item>
          <Form.Item name="password" rules={[{ required: true, min: 6, message: "密码至少6位" }]}>
            <Input.Password placeholder="密码" size="large" />
          </Form.Item>
          <Form.Item name="confirm" rules={[{ required: true, message: "请确认密码" }]}>
            <Input.Password placeholder="确认密码" size="large" />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={loading} block size="large">
              注册
            </Button>
          </Form.Item>
        </Form>
        <p className="text-center text-sm text-stone-500">
          已有账号？<Link href="/auth/login" className="text-blue-600 hover:underline">登录</Link>
        </p>
      </div>
    </main>
  );
}
