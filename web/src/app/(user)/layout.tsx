"use client";

import type { ReactNode } from "react";
import { useEffect } from "react";
import { usePathname, useRouter } from "next/navigation";
import { AppTopNav } from "@/components/layout/app-top-nav";
import { useUserStore } from "@/stores/use-user-store";
import { getStoredToken } from "@/services/api/client";

export default function UserLayout({ children }: { children: ReactNode }) {
    const fetchUser = useUserStore((s) => s.fetchUser);
    const pathname = usePathname();
    const router = useRouter();

    useEffect(() => {
        const token = getStoredToken();
        if (!token && !pathname.startsWith("/auth/")) {
            router.replace("/auth/login");
            return;
        }
        if (token && !pathname.startsWith("/auth/")) {
            fetchUser();
        }
    }, [fetchUser, pathname, router]);

    if (pathname.startsWith("/auth/")) {
        return <div className="flex h-dvh flex-col overflow-hidden bg-background text-foreground">{children}</div>;
    }

    return (
        <div className="flex h-dvh flex-col overflow-hidden bg-background text-foreground">
            <AppTopNav />
            <div className="min-h-0 flex-1 overflow-hidden">{children}</div>
        </div>
    );
}
