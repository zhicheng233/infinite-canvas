"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { Button, Form, Input, App } from "antd";
import Image from "next/image";
import { ReloadOutlined } from "@ant-design/icons";
import { register, fetchCaptcha } from "@/services/api/auth";
import { setStoredToken } from "@/services/api/client";
import { useUserStore } from "@/stores/use-user-store";

export default function RegisterPage() {
    const router = useRouter();
    const { message } = App.useApp();
    const [loading, setLoading] = useState(false);
    const [captchaId, setCaptchaId] = useState("");
    const [captchaSvg, setCaptchaSvg] = useState("");
    const [captchaLoading, setCaptchaLoading] = useState(false);
    const fetchUser = useUserStore((s) => s.fetchUser);

    const loadCaptcha = async () => {
        setCaptchaLoading(true);
        try {
            const data = await fetchCaptcha();
            setCaptchaId(data.captcha_id);
            setCaptchaSvg(data.svg);
        } catch {
            // captcha failed silently
        } finally {
            setCaptchaLoading(false);
        }
    };

    useEffect(() => {
        loadCaptcha();
    }, []);

    const onFinish = async (values: { username: string; password: string; confirm: string; captcha_answer?: string }) => {
        if (values.password !== values.confirm) {
            message.error("两次密码不一致");
            return;
        }
        if (!captchaId) {
            message.error("验证码加载失败，请刷新后重试");
            return;
        }
        if (!values.captcha_answer || values.captcha_answer.trim() === "") {
            message.error("请输入验证码");
            return;
        }
        setLoading(true);
        try {
            const result = await register({
                username: values.username,
                password: values.password,
                captcha_id: captchaId,
                captcha_answer: values.captcha_answer.trim(),
            });
            setStoredToken(result.token);
            await fetchUser();
            message.success("注册成功");
            router.replace("/");
        } catch (err: any) {
            message.error(err?.message || "注册失败");
            loadCaptcha(); // refresh captcha on failure
        } finally {
            setLoading(false);
        }
    };

    return (
        <main className="flex min-h-full items-center justify-center bg-background px-4">
            <div className="w-full max-w-sm">
                <div className="mb-6 flex justify-center">
                    <Image src="/logo.png" alt="无限画布" width={120} height={137} className="rounded-xl" />
                </div>
                <h1 className="mb-8 text-center text-2xl font-semibold text-stone-950 dark:text-stone-100">注册</h1>
                <Form layout="vertical" onFinish={onFinish} autoComplete="off">
                    <Form.Item name="username" rules={[{ required: true, message: "请输入用户名" }]}>
                        <Input placeholder="用户名" size="large" />
                    </Form.Item>
                    <Form.Item name="password" rules={[{ required: true, min: 8, message: "密码至少8位，需包含字母和数字" }]}>
                        <Input.Password placeholder="密码" size="large" />
                    </Form.Item>
                    <Form.Item name="confirm" rules={[{ required: true, message: "请确认密码" }]}>
                        <Input.Password placeholder="确认密码" size="large" />
                    </Form.Item>

                    {/* Captcha */}
                    <Form.Item label="验证码" required>
                        <div className="flex items-center gap-2">
                            <Form.Item name="captcha_answer" noStyle rules={[{ required: true, message: "请输入验证码结果" }]}>
                                <Input placeholder="计算结果" size="large" className="flex-1" autoComplete="off" />
                            </Form.Item>
                            <div
                                className="flex h-10 shrink-0 cursor-pointer items-center overflow-hidden rounded border border-stone-200 bg-white dark:border-stone-600 dark:bg-stone-800"
                                style={{ width: 130 }}
                                onClick={loadCaptcha}
                                title="点击刷新验证码"
                            >
                                {captchaLoading ? (
                                    <span className="w-full text-center text-xs text-stone-400">加载中...</span>
                                ) : captchaSvg ? (
                                    <div dangerouslySetInnerHTML={{ __html: captchaSvg }} className="flex h-full w-full items-center justify-center [&_svg]:h-full [&_svg]:w-full" />
                                ) : (
                                    <span className="w-full text-center text-xs text-stone-400">点击获取</span>
                                )}
                            </div>
                            <Button type="text" size="small" icon={<ReloadOutlined />} onClick={loadCaptcha} loading={captchaLoading} title="刷新验证码" />
                        </div>
                    </Form.Item>

                    <Form.Item>
                        <Button type="primary" htmlType="submit" loading={loading} block size="large">
                            注册
                        </Button>
                    </Form.Item>
                </Form>
                <p className="text-center text-sm text-stone-500">
                    已有账号？
                    <Link href="/auth/login" className="text-blue-600 hover:underline">
                        登录
                    </Link>
                </p>
            </div>
        </main>
    );
}
