"use client";

import { Drawer } from "antd";
import Link from "next/link";

import { navigationTools, type NavigationToolSlug } from "@/constant/navigation-tools";
import { useRechargeDialog } from "@/hooks/use-recharge-dialog";
import { useUserStore } from "@/stores/use-user-store";
import { cn } from "@/lib/utils";

type MobileNavDrawerProps = {
    open: boolean;
    activeToolSlug?: NavigationToolSlug;
    onClose: () => void;
};

export function MobileNavDrawer({ open, activeToolSlug, onClose }: MobileNavDrawerProps) {
    const user = useUserStore((s) => s.user);
    const openRechargeDialog = useRechargeDialog();
    const isAdmin = user?.role === "super_admin" || user?.role === "tenant_admin";
    const filteredTools = navigationTools.filter((t) => !(t as any).adminOnly || isAdmin);
    return (
        <Drawer title="导航" placement="left" size={280} open={open} onClose={onClose} className="md:hidden">
            <div className="space-y-1">
                {filteredTools.map((tool) => {
                    const Icon = tool.icon;
                    const active = tool.slug === activeToolSlug;
                    if (tool.slug === "recharge") {
                        return (
                            <button
                                key={tool.slug}
                                type="button"
                                onClick={() => {
                                    onClose();
                                    openRechargeDialog();
                                }}
                                className="flex w-full items-center gap-3 rounded-lg border-0 bg-transparent px-3 py-3 text-left text-base text-stone-600 transition hover:bg-stone-100 hover:text-stone-950 dark:text-stone-300 dark:hover:bg-stone-800 dark:hover:text-stone-100"
                            >
                                <Icon className="size-5" />
                                <span>{tool.label}</span>
                            </button>
                        );
                    }
                    return (
                        <Link
                            key={tool.slug}
                            href={`/${tool.slug}`}
                            onClick={onClose}
                            className={cn(
                                "flex items-center gap-3 rounded-lg px-3 py-3 text-base transition",
                                active ? "bg-stone-100 font-medium text-stone-950 dark:bg-stone-800 dark:text-stone-100" : "text-stone-600 hover:bg-stone-100 hover:text-stone-950 dark:text-stone-300 dark:hover:bg-stone-800 dark:hover:text-stone-100",
                            )}
                        >
                            <Icon className="size-5" />
                            <span>{tool.label}</span>
                        </Link>
                    );
                })}
            </div>
        </Drawer>
    );
}
