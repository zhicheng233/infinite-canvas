"use client";

import type { ReactNode } from "react";
import { useEffect, useRef } from "react";
import { getAutoChannelModels, getChannels, getChannelModels } from "@/services/api/channel";
import { AUTH_TOKEN_CHANGE_EVENT, getStoredToken } from "@/services/api/client";
import { getMetrics } from "@/services/api/metrics";
import { listPricing } from "@/services/api/pricing";
import { useConfigStore } from "@/stores/use-config-store";

export function ClientRootInit({ children }: { children: ReactNode }) {
    const handledConfigParams = useRef(false);
    const beginServerCatalogRefresh = useConfigStore((state) => state.beginServerCatalogRefresh);
    const invalidateServerCatalogRefresh = useConfigStore((state) => state.invalidateServerCatalogRefresh);
    const applyServerCatalogSnapshot = useConfigStore((state) => state.applyServerCatalogSnapshot);
    const failServerCatalogRefresh = useConfigStore((state) => state.failServerCatalogRefresh);

    // URL-based config import removed - API config is now admin-only via server backend
    useEffect(() => {
        if (handledConfigParams.current) return;
        handledConfigParams.current = true;
        const searchParams = new URLSearchParams(window.location.search);
        const hasConfig = searchParams.get("baseUrl") || searchParams.get("baseurl") || searchParams.get("apiKey") || searchParams.get("apikey");
        if (hasConfig) {
            searchParams.delete("baseUrl");
            searchParams.delete("baseurl");
            searchParams.delete("apiKey");
            searchParams.delete("apikey");
            window.history.replaceState(null, "", window.location.pathname + (searchParams.size ? "?" + searchParams : "") + window.location.hash);
        }
    }, []);

    useEffect(() => {
        let cancelled = false;
        const syncCatalog = async () => {
            const token = getStoredToken();
            if (!token) {
                invalidateServerCatalogRefresh();
                return;
            }
            const requestId = beginServerCatalogRefresh();
            try {
                const [channels, autoChannelModels, pricing, metrics] = await Promise.all([getChannels(), getAutoChannelModels(), listPricing(), getMetrics(24).catch(() => null)]);
                const entries = await Promise.all(channels.map(async (channel) => [channel.id, await getChannelModels(channel.id)] as const));
                if (!cancelled && token === getStoredToken()) {
                    applyServerCatalogSnapshot(requestId, { channels, channelModels: Object.fromEntries(entries), autoChannelModels, pricing, metrics });
                }
            } catch (reason) {
                if (!cancelled && token === getStoredToken()) failServerCatalogRefresh(requestId, reason instanceof Error ? reason.message : "加载模型列表失败");
            }
        };

        void syncCatalog();

        const handleTokenChange = () => {
            void syncCatalog();
        };
        const handleStorage = (event: StorageEvent) => {
            if (event.key === "infinite-canvas:auth_token") void syncCatalog();
        };

        window.addEventListener(AUTH_TOKEN_CHANGE_EVENT, handleTokenChange);
        window.addEventListener("storage", handleStorage);

        return () => {
            cancelled = true;
            window.removeEventListener(AUTH_TOKEN_CHANGE_EVENT, handleTokenChange);
            window.removeEventListener("storage", handleStorage);
        };
    }, [applyServerCatalogSnapshot, beginServerCatalogRefresh, failServerCatalogRefresh, invalidateServerCatalogRefresh]);

    return <>{children}</>;
}
