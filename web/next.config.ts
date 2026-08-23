import type { NextConfig } from "next";
import { PHASE_DEVELOPMENT_SERVER } from "next/constants";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";
import { parseChangelog } from "@/lib/release";

const webDir = dirname(fileURLToPath(import.meta.url));
const localReleaseVersion = readFileSync(resolve(webDir, "../VERSION"), "utf8").trim() || "dev";
const buildVersion = process.env.BUILD_VERSION?.trim() || process.env.NEXT_PUBLIC_BUILD_VERSION?.trim() || "dev-000000000000Z";
const localChangelog = readFileSync(resolve(webDir, "../CHANGELOG.md"), "utf8");
const defaultBackendApiUrl = "http://127.0.0.1:18080/backend-api";

function resolveBackendApiUrl(): string {
    const configured = process.env.BACKEND_API_URL?.trim() || defaultBackendApiUrl;
    return configured.replace(/\/+$/, "").replace(/\/backend-api$/, "");
}

export default function nextConfig(phase: string): NextConfig {
    const isDev = phase === PHASE_DEVELOPMENT_SERVER;
    const releases = parseChangelog(localChangelog);

    return {
        output: "standalone",
        allowedDevOrigins: isDev ? ["*.*.*.*"] : [],
        ...(isDev
            ? {
                  rewrites: async () => [
                      {
                          source: "/backend-api/:path*",
                          destination: `${resolveBackendApiUrl()}/backend-api/:path*`,
                      },
                  ],
              }
            : {}),
        typescript: {
            ignoreBuildErrors: true,
        },
        env: {
            NEXT_PUBLIC_APP_VERSION: localReleaseVersion,
            NEXT_PUBLIC_BUILD_VERSION: buildVersion,
            NEXT_PUBLIC_APP_RELEASES: JSON.stringify(releases),
        },
    };
}
