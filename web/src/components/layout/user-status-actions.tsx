"use client";

import type { CSSProperties } from "react";
import { Keyboard, Settings2, LogIn, LogOut, Shield } from "lucide-react";

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

    return (
        <div className="inline-flex shrink-0 items-center gap-1">
            <UserCreditDisplay />
            {showConfig ? (
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
                        <span className="ml-1 text-xs text-stone-500 dark:text-stone-400">{user.displayName}</span>
                    ) : null}
                    {isAdmin ? (
                        <button
                            type="button"
                            className={naturalIconClass}
                            style={iconStyle}
                            onClick={() => router.push("/admin")}
                            aria-label="管理后台"
                            title="管理后台"
                        >
                            <Shield className="size-4" />
                        </button>
                    ) : null}
                    <button type="button" className={naturalIconClass} style={iconStyle} onClick={handleLogout} aria-label="退出登录" title="退出登录">
                        <LogOut className="size-4" />
                    </button>
                </>
            ) : (
                <button
                    type="button"
                    className={naturalIconClass}
                    style={iconStyle}
                    onClick={() => router.push("/auth/login")}
                    aria-label="登录"
                    title="登录"
                >
                    <LogIn className="size-4" />
                </button>
            )}
        </div>
    );
}
