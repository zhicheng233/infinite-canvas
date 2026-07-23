"use client";

import { useEffect, useMemo, useRef, useState, type ComponentProps } from "react";
import { Zap } from "lucide-react";
import { Button } from "antd";

import { estimateCost, getBalance } from "@/services/api/credits";
import { getStoredToken } from "@/services/api/client";
import { decodeChannelModel, modelOptionName, parseMergeModelValue } from "@/stores/use-config-store";

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

export type CreditEstimate = { status: "idle" | "loading" | "ready" | "missing" | "error"; credits: number };

type CreditEstimateRequest = ReturnType<typeof buildCreditEstimateRequest>;

const inFlightCreditEstimates = new Map<string, ReturnType<typeof estimateCost>>();

export function buildCreditEstimateRequest(model: string, count?: string | number, options?: { type?: string; seconds?: string | number; resolution?: string; size?: string }) {
    const decoded = decodeChannelModel(model || "");
    const merge = parseMergeModelValue(model || "");
    const rawModel = merge?.groupName || modelOptionName(model || "");
    const normalizedCount = Math.max(1, Math.floor(Math.abs(Number(count)) || 1));
    const params: Record<string, string | number> = { count: normalizedCount };
    if (options?.type) params.type = options.type;
    if (options?.seconds) params.seconds = options.seconds;
    if (options?.resolution) params.resolution = options.resolution;
    if (options?.size) params.size = options.size;
    if (merge) {
        params.channel_id = merge.channelId;
        params.fuzzy_group_name = merge.groupName;
    } else if (decoded) {
        const channelId = Number(decoded.channelId);
        if (Number.isInteger(channelId) && channelId >= 0) params.channel_id = channelId;
        const channelModelId = decoded.channelModelId === null ? null : Number(decoded.channelModelId);
        if (channelModelId && channelModelId > 0) params.channel_model_id = channelModelId;
    }
    return { model: rawModel, params };
}

export function creditEstimateRequestKey(request: CreditEstimateRequest) {
    const params = request.params;
    return JSON.stringify([request.model, params.channel_id ?? null, params.channel_model_id ?? null, params.fuzzy_group_name ?? null, params.count ?? null, params.seconds ?? null, params.resolution ?? null, params.size ?? null, params.type ?? null]);
}

export function requestCreditEstimate(request: CreditEstimateRequest) {
    const key = creditEstimateRequestKey(request);
    const existing = inFlightCreditEstimates.get(key);
    if (existing) return existing;

    const pending = estimateCost(request.model, request.params).finally(() => {
        if (inFlightCreditEstimates.get(key) === pending) inFlightCreditEstimates.delete(key);
    });
    inFlightCreditEstimates.set(key, pending);
    return pending;
}

export function resolveCreditEstimate(data: { total_cost?: number; credits_per_unit?: number; unit_type?: string }, count: number): CreditEstimate {
    const totalCost = Number(data?.total_cost) || 0;
    if (totalCost > 0) return { status: "ready", credits: totalCost };
    const unitCost = Number(data?.credits_per_unit) || 0;
    const multiplier = data?.unit_type === "per_image" ? count : 1;
    return unitCost > 0 ? { status: "ready", credits: unitCost * multiplier } : { status: "missing", credits: 0 };
}

export function creditEstimateButtonText(estimate: CreditEstimate) {
    if (estimate.status === "idle" || estimate.status === "loading") return "正在预估计费";
    if (estimate.status === "error") return "计费预估失败";
    if (estimate.status === "missing") return "未配置计费";
    return `${estimate.credits} 积分`;
}

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
    const decoded = useMemo(() => decodeChannelModel(model || ""), [model]);
    const normalizedModel = useMemo(() => modelOptionName(model || ""), [model]);
    const channelId = decoded?.channelId;
    const channelModelId = decoded?.channelModelId;
    const normalizedCount = Math.max(1, Math.floor(Math.abs(Number(count)) || 1));
    const requestType = options?.type || "";
    const requestSeconds = options?.seconds || "";
    const requestResolution = options?.resolution || "";
    const requestSize = options?.size || "";
    const [estimate, setEstimate] = useState<CreditEstimate>({ status: "idle", credits: 0 });
    const requestSequence = useRef(0);

    useEffect(() => {
        const sequence = ++requestSequence.current;
        const token = typeof window !== "undefined" ? window.localStorage.getItem("infinite-canvas:auth_token") : null;
        if (!token || !normalizedModel) {
            setEstimate({ status: "idle", credits: 0 });
            return;
        }

        let cancelled = false;
        setEstimate({ status: "loading", credits: 0 });
        const request = buildCreditEstimateRequest(model, normalizedCount, {
            type: requestType,
            seconds: requestSeconds,
            resolution: requestResolution,
            size: requestSize,
        });
        requestCreditEstimate(request)
            .then((data) => {
                if (cancelled || sequence !== requestSequence.current) return;
                setEstimate(resolveCreditEstimate(data, normalizedCount));
            })
            .catch((error: unknown) => {
                if (!cancelled && sequence === requestSequence.current) setEstimate({ status: error instanceof Error && error.message.includes("未配置计费") ? "missing" : "error", credits: 0 });
            });

        return () => {
            cancelled = true;
        };
    }, [normalizedCount, normalizedModel, requestResolution, requestSeconds, requestSize, requestType, channelId, channelModelId, model]);

    return estimate;
}

export function CreditCostHint({ credits, estimate, balance, compact = false }: { credits?: number; estimate?: CreditEstimate; balance: number | null; compact?: boolean }) {
    const current = estimate || { status: "ready", credits: credits || 0 };
    const currentCredits = current.credits;
    const failed = current.status === "error";
    const hasCost = currentCredits > 0;
    const postBalance = balance === null ? null : balance - currentCredits;
    const insufficient = hasCost && balance !== null && balance < currentCredits;
    if (compact) {
        return (
            <span className={`inline-flex items-center gap-1 text-xs ${insufficient ? "text-red-500" : "text-stone-500 dark:text-stone-400"}`}>
                <CreditSymbol className={insufficient ? "text-red-500" : "text-amber-500"} />
                {failed ? "计费预估失败" : current.status === "idle" || current.status === "loading" ? "正在预估计费" : hasCost ? `预计 ${currentCredits} 积分` : "未配置计费"}
            </span>
        );
    }
    return (
        <div className="mt-2 flex flex-wrap items-center justify-between gap-2 text-xs text-stone-500 dark:text-stone-400">
            <span className={`inline-flex items-center gap-1 ${insufficient ? "text-red-500" : ""}`}>
                <CreditSymbol className={insufficient ? "text-red-500" : "text-amber-500"} />
                {failed ? "暂时无法读取计费预估" : current.status === "idle" || current.status === "loading" ? "正在预估本次扣费" : balance === null ? "正在读取当前积分" : `当前余额 ${balance}，预计生成后剩余 ${Math.max(postBalance || 0, 0)}`}
            </span>
            <span className={insufficient ? "text-red-500" : ""}>
                {failed ? "计费预估失败" : current.status === "idle" || current.status === "loading" ? "正在读取计费配置" : hasCost ? `本次预计扣除 ${currentCredits} 积分${insufficient ? "，余额不足" : ""}` : "当前模型未配置扣费"}
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
