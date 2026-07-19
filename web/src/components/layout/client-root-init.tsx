"use client";

import type { ReactNode } from "react";
import { useEffect, useRef } from "react";
import { getChannels, getChannelModels } from "@/services/api/channel";
import { AUTH_TOKEN_CHANGE_EVENT, getStoredToken } from "@/services/api/client";
import { getMetrics } from "@/services/api/metrics";
import { listPricing } from "@/services/api/pricing";
import { useConfigStore } from "@/stores/use-config-store";

export function ClientRootInit({ children }: { children: ReactNode }) {
    const handledConfigParams = useRef(false);
    const applyServerChannelCatalog = useConfigStore((state) => state.applyServerChannelCatalog);
    const applyServerOptionMetadata = useConfigStore((state) => state.applyServerOptionMetadata);
    const setServerCatalogError = useConfigStore((state) => state.setServerCatalogError);
    const setServerCatalogLoading = useConfigStore((state) => state.setServerCatalogLoading);

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
            if (!getStoredToken()) return;
            setServerCatalogLoading(true);
            setServerCatalogError(null);
            try {
                const channels = await getChannels();
                const entries = await Promise.all(channels.map(async (channel) => [channel.id, await getChannelModels(channel.id)] as const));
                if (cancelled) return;
                applyServerChannelCatalog(channels, Object.fromEntries(entries));
                const [pricing, metrics] = await Promise.all([listPricing(), getMetrics(24).catch(() => null)]);
                if (!cancelled) applyServerOptionMetadata(pricing, metrics);
            } catch (reason) {
                if (!cancelled) setServerCatalogError(reason instanceof Error ? reason.message : "加载模型列表失败");
            } finally {
                if (!cancelled) setServerCatalogLoading(false);
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
    }, [applyServerChannelCatalog, applyServerOptionMetadata, setServerCatalogError, setServerCatalogLoading]);

    return <>{children}</>;
}
