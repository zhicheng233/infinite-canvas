const fallbackBuildVersion = "dev-000000000000Z";

export const buildVersion = process.env.NEXT_PUBLIC_BUILD_VERSION?.trim() || fallbackBuildVersion;
export const buildVersionLabel = `版本 ${buildVersion}`;
