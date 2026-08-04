"use client";

import { useEffect, useRef } from "react";
import Link from "next/link";
import { Button } from "antd";
import { Zap } from "lucide-react";

import { useRechargeDialog } from "@/hooks/use-recharge-dialog";

export default function RechargePage() {
    const openRechargeDialog = useRechargeDialog();
    const openedRef = useRef(false);

    useEffect(() => {
        if (openedRef.current) return;
        openedRef.current = true;
        openRechargeDialog();
    }, [openRechargeDialog]);

    return (
        <main className="flex h-full items-center justify-center px-6">
            <section className="w-full max-w-md rounded-lg border border-stone-200 bg-background p-6 text-center shadow-sm dark:border-stone-800">
                <div className="mx-auto mb-4 flex size-12 items-center justify-center rounded-lg bg-amber-50 text-amber-600 dark:bg-amber-950/40 dark:text-amber-400">
                    <Zap className="size-6" />
                </div>
                <h1 className="text-xl font-semibold text-stone-950 dark:text-stone-100">积分充值</h1>
                <p className="mt-2 text-sm text-stone-500 dark:text-stone-400">请在弹窗中扫码联系客服获取充值方案，当前比例为 1 元 = 10 积分。</p>
                <div className="mt-5 flex justify-center gap-2">
                    <Button type="primary" onClick={openRechargeDialog}>
                        打开充值二维码
                    </Button>
                    <Link href="/credits">
                        <Button>查看积分明细</Button>
                    </Link>
                </div>
            </section>
        </main>
    );
}
