"use client";

import type { ReactNode } from "react";
import { useEffect, useRef } from "react";

export function ClientRootInit({ children }: { children: ReactNode }) {
    const handledConfigParams = useRef(false);

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

    return <>{children}</>;
}
