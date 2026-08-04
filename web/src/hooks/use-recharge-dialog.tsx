"use client";

import { App } from "antd";
import { useCallback } from "react";

export function useRechargeDialog() {
    const { modal } = App.useApp();

    return useCallback(() => {
        modal.info({
            title: "积分充值",
            icon: null,
            centered: true,
            width: 420,
            okText: "我知道了",
            content: (
                <div className="pt-2 text-center">
                    <div className="mx-auto mb-4 flex size-56 items-center justify-center rounded-lg border border-stone-200 bg-white p-3 shadow-sm dark:border-stone-700">
                        <img src="/recharge-contact-qr.png" alt="联系客服充值二维码" className="h-full w-full object-contain" />
                    </div>
                    <div className="text-base font-medium text-stone-950 dark:text-stone-100">联系客服获取充值方案</div>
                    <div className="mt-2 text-sm text-stone-500 dark:text-stone-400">当前充值比例：1 元 = 10 积分</div>
                </div>
            ),
        });
    }, [modal]);
}
