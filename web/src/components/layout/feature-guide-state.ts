import type { FeatureGuide, FeatureGuideSurface } from "@/services/api/feature-guide";

const SURFACES = new Set<FeatureGuideSurface>(["canvas", "image", "video"]);

export function getFeatureGuideSurface(pathname: string): FeatureGuideSurface | null {
    if (/^\/canvas\/[^/]+$/.test(pathname)) return "canvas";
    const surface = pathname.slice(1) as FeatureGuideSurface;
    return pathname === `/${surface}` && SURFACES.has(surface) && surface !== "canvas" ? surface : null;
}

export function getFeatureGuideCompletionKey(userId: string, surface: FeatureGuideSurface) {
    return `infinite-canvas:feature-guide:${userId}:${surface}:completed-version`;
}

export function shouldPresentFeatureGuide(guide: FeatureGuide | null, completedVersion: string | null) {
    return Boolean(guide?.enabled && guide.pages.some((page) => page.trim()) && completedVersion !== String(guide.version));
}
