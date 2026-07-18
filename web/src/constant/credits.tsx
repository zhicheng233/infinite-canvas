"use client";

import { useEffect, useMemo, useState, type ComponentProps } from "react";
import { Zap } from "lucide-react";
import { Button } from "antd";

import { estimateCost, getBalance } from "@/services/api/credits";
import { getStoredToken } from "@/services/api/client";
import { modelOptionName } from "@/stores/use-config-store";

export function CreditSymbol({ className, ...props }: ComponentProps<"span">) {
    return (
        <span {...props} className={`inline-flex items-center justify-center ${className || ""}`}>
            <Zap className="size-[1em] fill-current" strokeWidth={2.4} />
        </span>
    );
}

export type ModelCreditCost = {
    model: string;
    credits: number;
};

function modelCreditCost(modelCosts: ModelCreditCost[] | undefined, model: string) {
    return modelCosts?.find((item) => item.model === model)?.credits || 0;
}

export function requestCreditCost(options: { channelMode: string; modelCosts?: ModelCreditCost[]; model: string; count?: string | number }) {
    if (options.channelMode !== "remote") return 0;
    const count = Math.max(1, Math.floor(Math.abs(Number(options.count)) || 1));
    return modelCreditCost(options.modelCosts, options.model) * count;
}

const CREDIT_BALANCE_EVENT = "infinite-canvas:credits-updated";

export function notifyCreditBalanceChanged() {
    if (typeof window === "undefined") return;
    window.dispatchEvent(new CustomEvent(CREDIT_BALANCE_EVENT));
}

export function useCreditBalanceRefreshSignal() {
    const [signal, setSignal] = useState(0);

    useEffect(() => {
        if (typeof window === "undefined") return;
        const handleRefresh = () => setSignal((value) => value + 1);
        window.addEventListener(CREDIT_BALANCE_EVENT, handleRefresh);
        return () => window.removeEventListener(CREDIT_BALANCE_EVENT, handleRefresh);
    }, []);

    return signal;
}

export function useUserCreditBalance() {
    const refreshSignal = useCreditBalanceRefreshSignal();
    const token = getStoredToken();
    const [balance, setBalance] = useState<number | null>(null);

    useEffect(() => {
        if (!token) {
            setBalance(null);
            return;
        }
        getBalance()
            .then((data) => setBalance(data.balance))
            .catch(() => setBalance(null));
    }, [refreshSignal, token]);

    return balance;
}

export function useEstimatedCreditCost(model: string, count?: string | number, options?: { type?: string; seconds?: string | number; resolution?: string; size?: string }) {
    const normalizedModel = useMemo(() => modelOptionName(model || ""), [model]);
    const normalizedCount = Math.max(1, Math.floor(Math.abs(Number(count)) || 1));
    const requestType = options?.type || "";
    const requestSeconds = options?.seconds || "";
    const requestResolution = options?.resolution || "";
    const requestSize = options?.size || "";
    const [credits, setCredits] = useState(0);

    useEffect(() => {
        const token = typeof window !== "undefined" ? window.localStorage.getItem("infinite-canvas:auth_token") : null;
        if (!token || !normalizedModel) {
            setCredits(0);
            return;
        }

        let cancelled = false;
        estimateCost(normalizedModel, {
            type: requestType || undefined,
            count: normalizedCount,
            seconds: requestSeconds || undefined,
            resolution: requestResolution || undefined,
            size: requestSize || undefined,
        })
            .then((data) => {
                if (cancelled) return;
                const totalCost = Number(data?.total_cost) || 0;
                if (totalCost > 0) {
                    setCredits(totalCost);
                    return;
                }
                const unitCost = Number(data?.credits_per_unit) || 0;
                const unitType = String(data?.unit_type || "");
                const multiplier = unitType === "per_image" ? normalizedCount : 1;
                setCredits(unitCost * multiplier);
            })
            .catch(() => {
                if (!cancelled) setCredits(0);
            });

        return () => {
            cancelled = true;
        };
    }, [normalizedCount, normalizedModel, requestResolution, requestSeconds, requestSize, requestType]);

    return credits;
}

export function CreditCostHint({ credits, balance, compact = false }: { credits: number; balance: number | null; compact?: boolean }) {
    const hasCost = credits > 0;
    const postBalance = balance === null ? null : balance - credits;
    const insufficient = hasCost && balance !== null && balance < credits;
    if (compact) {
        return (
            <span className={`inline-flex items-center gap-1 text-xs ${insufficient ? "text-red-500" : "text-stone-500 dark:text-stone-400"}`}>
                <CreditSymbol className={insufficient ? "text-red-500" : "text-amber-500"} />
                {hasCost ? `预计 ${credits} 积分` : "未配置计费"}
            </span>
        );
    }
    return (
        <div className="mt-2 flex flex-wrap items-center justify-between gap-2 text-xs text-stone-500 dark:text-stone-400">
            <span className={`inline-flex items-center gap-1 ${insufficient ? "text-red-500" : ""}`}>
                <CreditSymbol className={insufficient ? "text-red-500" : "text-amber-500"} />
                {balance === null ? "正在读取当前积分" : `当前余额 ${balance}，预计生成后剩余 ${Math.max(postBalance || 0, 0)}`}
            </span>
            <span className={insufficient ? "text-red-500" : ""}>
                {hasCost ? `本次预计扣除 ${credits} 积分${insufficient ? "，余额不足" : ""}` : "当前模型未配置扣费"}
            </span>
            {insufficient ? (
                <Button size="small" type="link" href="/recharge" className="!h-auto !p-0">
                    去充值
                </Button>
            ) : null}
        </div>
    );
}

export function isInsufficientCreditError(message: string) {
    return message.includes("积分不足");
}

export function CreditHelpActions() {
    return (
        <div className="flex flex-wrap justify-center gap-2">
            <Button size="small" href="/credits">
                积分明细
            </Button>
            <Button size="small" type="primary" ghost href="/recharge">
                去充值
            </Button>
        </div>
    );
}
