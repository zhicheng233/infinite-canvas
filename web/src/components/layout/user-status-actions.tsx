"use client";

import type { CSSProperties } from "react";
import { Keyboard, Settings2, LogIn, LogOut, Shield, User, ChevronDown } from "lucide-react";
import { Dropdown } from "antd";

import { AnimatedThemeToggler } from "@/components/ui/animated-theme-toggler";
import { UserCreditDisplay } from "@/components/layout/user-credit-display";
import { canvasThemes } from "@/lib/canvas-theme";
import { useUserStore } from "@/stores/use-user-store";
import { useConfigStore } from "@/stores/use-config-store";
import { useThemeStore } from "@/stores/use-theme-store";
import { getStoredToken } from "@/services/api/client";
import { useRouter } from "next/navigation";

type UserStatusActionsProps = {
    showConfig?: boolean;
    variant?: "default" | "canvas";
    onOpenShortcuts?: () => void;
};

export function UserStatusActions({ showConfig = true, variant = "default", onOpenShortcuts }: UserStatusActionsProps) {
    const router = useRouter();
    const theme = useThemeStore((state) => state.theme);
    const setTheme = useThemeStore((state) => state.setTheme);
    const user = useUserStore((state) => state.user);
    const clearSession = useUserStore((state) => state.clearSession);
    const openConfigDialog = useConfigStore((state) => state.openConfigDialog);
    const canvasTheme = canvasThemes[theme];
    const naturalIconClass = "inline-flex size-7 shrink-0 items-center justify-center text-stone-600 transition hover:text-stone-950 dark:text-stone-300 dark:hover:text-white [&_svg]:size-4";
    const iconStyle: CSSProperties | undefined = variant === "canvas" ? { color: canvasTheme.node.text } : undefined;

    const handleLogout = () => {
        clearSession();
        router.push("/auth/login");
    };

    const isAdmin = user?.role === "super_admin" || user?.role === "tenant_admin";
    const userMenuItems = [
        {
            key: "profile",
            icon: <User className="size-4" />,
            label: "个人中心",
            onClick: () => router.push("/settings"),
        },
        ...(isAdmin
            ? [
                  {
                      key: "admin",
                      icon: <Shield className="size-4" />,
                      label: "管理后台",
                      onClick: () => router.push("/admin"),
                  },
              ]
            : []),
        {
            key: "logout",
            icon: <LogOut className="size-4" />,
            label: "退出登录",
            onClick: handleLogout,
            danger: true,
        },
    ];

    return (
        <div className="inline-flex shrink-0 items-center gap-1">
            <UserCreditDisplay />
            {showConfig && isAdmin ? (
                <button type="button" className={naturalIconClass} style={iconStyle} onClick={() => openConfigDialog(false)} aria-label="配置" title="配置">
                    <Settings2 className="size-4" />
                </button>
            ) : null}
            <AnimatedThemeToggler theme={theme} onThemeChange={setTheme} className={naturalIconClass} style={iconStyle} aria-label={theme === "dark" ? "切换到浅色主题" : "切换到深色主题"} title={theme === "dark" ? "切换到浅色主题" : "切换到深色主题"} />
            {onOpenShortcuts ? (
                <button type="button" className={naturalIconClass} style={iconStyle} onClick={onOpenShortcuts} aria-label="快捷键" title="快捷键">
                    <Keyboard className="size-4" />
                </button>
            ) : null}
            {getStoredToken() ? (
                <>
                    {user ? (
                        <Dropdown menu={{ items: userMenuItems }} trigger={["click"]} placement="bottomRight">
                            <button
                                type="button"
                                className="ml-1 inline-flex items-center gap-1 rounded-md px-1.5 py-1 text-xs text-stone-500 transition hover:bg-stone-100 hover:text-stone-900 dark:text-stone-400 dark:hover:bg-stone-800 dark:hover:text-white"
                                aria-label="用户菜单"
                                title="用户菜单"
                            >
                                <span>{user.displayName}</span>
                                <ChevronDown className="size-3.5" />
                            </button>
                        </Dropdown>
                    ) : null}
                </>
            ) : (
                <button type="button" className={naturalIconClass} style={iconStyle} onClick={() => router.push("/auth/login")} aria-label="登录" title="登录">
                    <LogIn className="size-4" />
                </button>
            )}
        </div>
    );
}
