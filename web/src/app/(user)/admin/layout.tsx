"use client";

import type { ReactNode } from "react";
import { usePathname, useRouter } from "next/navigation";
import { Layout, Menu, Typography } from "antd";
import { Users, CreditCard, ReceiptText, ArrowLeft, Shield, LayoutDashboard, Settings, AlertTriangle } from "lucide-react";
import { useUserStore } from "@/stores/use-user-store";
import { useEffect } from "react";

const { Sider, Content } = Layout;
const { Text } = Typography;

const menuItems = [
    { key: "/admin", icon: <LayoutDashboard className="size-4" />, label: "管理概览" },
    { key: "/admin/users", icon: <Users className="size-4" />, label: "用户管理" },
    { key: "/admin/api-config", icon: <Settings className="size-4" />, label: "API 与模型配置" },
    { key: "/admin/model-logs", icon: <AlertTriangle className="size-4" />, label: "模型失败日志" },
    { key: "/admin/recharge", icon: <CreditCard className="size-4" />, label: "积分充值" },
    { key: "/admin/transactions", icon: <ReceiptText className="size-4" />, label: "积分流水" },
];

export default function AdminLayout({ children }: { children: ReactNode }) {
    const pathname = usePathname();
    const router = useRouter();
    const user = useUserStore((s) => s.user);

    useEffect(() => {
        if (user && user.role !== "super_admin" && user.role !== "tenant_admin") {
            router.replace("/");
        }
    }, [user, router]);

    const selectedKey =
        menuItems.find((item) => {
            if (item.key === "/admin") return pathname === "/admin";
            return pathname.startsWith(item.key);
        })?.key || "/admin";

    return (
        <Layout className="h-full bg-background" hasSider>
            <Sider width={200} className="border-r border-stone-200 dark:border-stone-800 bg-background" theme="light">
                <div className="flex flex-col h-full">
                    <div className="flex items-center gap-2 px-4 py-4 border-b border-stone-200 dark:border-stone-800">
                        <Shield className="size-5 text-blue-600" />
                        <Text strong className="text-sm">
                            管理后台
                        </Text>
                    </div>
                    <Menu
                        mode="inline"
                        selectedKeys={[selectedKey]}
                        items={menuItems.map((item) => ({
                            key: item.key,
                            icon: item.icon,
                            label: item.label,
                        }))}
                        onClick={({ key }) => router.push(key)}
                        className="flex-1 border-r-0 pt-2"
                    />
                    <div className="border-t border-stone-200 dark:border-stone-800 p-3">
                        <button onClick={() => router.push("/")} className="flex items-center gap-2 text-xs text-stone-500 hover:text-stone-800 dark:hover:text-stone-200 w-full px-3 py-2 rounded transition-colors">
                            <ArrowLeft className="size-3" />
                            返回画布
                        </button>
                    </div>
                </div>
            </Sider>
            <Content className="overflow-auto p-6">{children}</Content>
        </Layout>
    );
}
