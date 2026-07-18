"use client";

import type { ReactNode } from "react";
import { useEffect, useRef } from "react";
import { getApiModelCatalog } from "@/services/api/api-config";
import { AUTH_TOKEN_CHANGE_EVENT, getStoredToken } from "@/services/api/client";
import { useConfigStore } from "@/stores/use-config-store";

export function ClientRootInit({ children }: { children: ReactNode }) {
    const handledConfigParams = useRef(false);
    const applyServerModelCatalog = useConfigStore((state) => state.applyServerModelCatalog);

    // URL-based config import removed - API config is now admin-only via server backend
    useEffect(() => {
        if (handledConfigParams.current) return;
        handledConfigParams.current = true;
        const searchParams = new URLSearchParams(window.location.search);
        const hasConfig = searchParams.get("baseUrl") || searchParams.get("baseurl")
            || searchParams.get("apiKey") || searchParams.get("apikey");
        if (hasConfig) {
            searchParams.delete("baseUrl");
            searchParams.delete("baseurl");
            searchParams.delete("apiKey");
            searchParams.delete("apikey");
            window.history.replaceState(
                null,
                "",
                window.location.pathname + (searchParams.size ? "?" + searchParams : "") + window.location.hash,
            );
        }
    }, []);

    useEffect(() => {
        let cancelled = false;
        const syncCatalog = async () => {
            if (!getStoredToken()) return;
            try {
                const catalog = await getApiModelCatalog();
                if (cancelled) return;
                applyServerModelCatalog({
                    models: catalog.models,
                    imageModels: catalog.image_models,
                    videoModels: catalog.video_models,
                    textModels: catalog.text_models,
                    audioModels: catalog.audio_models,
                    modelRoutes: catalog.model_routes,
                    modelVideoDurations: catalog.model_video_durations,
                    modelVideoCustomizable: catalog.model_video_customizable,
                });
            } catch {
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
    }, [applyServerModelCatalog]);

    return <>{children}</>;
}
