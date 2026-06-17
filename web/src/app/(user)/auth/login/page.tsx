"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { Button, Form, Input, App } from "antd";
import { login } from "@/services/api/auth";
import { setStoredToken } from "@/services/api/client";
import { useUserStore } from "@/stores/use-user-store";

export default function LoginPage() {
  const router = useRouter();
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);
  const fetchUser = useUserStore((s) => s.fetchUser);

  const onFinish = async (values: { username: string; password: string }) => {
    setLoading(true);
    try {
      const result = await login(values);
      setStoredToken(result.token);
      await fetchUser();
      message.success("登录成功");
      router.replace("/");
    } catch (err: any) {
      message.error(err?.message || "登录失败");
    } finally {
      setLoading(false);
    }
  };

  return (
    <main className="flex min-h-full items-center justify-center bg-background px-4">
      <div className="w-full max-w-sm">
        <h1 className="mb-8 text-center text-2xl font-semibold text-stone-950 dark:text-stone-100">登录</h1>
        <Form layout="vertical" onFinish={onFinish} autoComplete="off">
          <Form.Item name="username" rules={[{ required: true, message: "请输入用户名" }]}>
            <Input placeholder="用户名" size="large" />
          </Form.Item>
          <Form.Item name="password" rules={[{ required: true, message: "请输入密码" }]}>
            <Input.Password placeholder="密码" size="large" />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={loading} block size="large">
              登录
            </Button>
          </Form.Item>
        </Form>
        <p className="text-center text-sm text-stone-500">
          还没有账号？<Link href="/auth/register" className="text-blue-600 hover:underline">注册</Link>
        </p>
      </div>
    </main>
  );
}
