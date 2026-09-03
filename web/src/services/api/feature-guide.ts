import apiClient from "./client";

export type FeatureGuideSurface = "canvas" | "image" | "video";

export type FeatureGuide = {
    surface: FeatureGuideSurface;
    enabled: boolean;
    title: string;
    pages: string[];
    version: number;
};

export type SaveFeatureGuideInput = Pick<FeatureGuide, "enabled" | "title" | "pages">;

export async function getFeatureGuide(surface: FeatureGuideSurface): Promise<FeatureGuide | null> {
    const res = await apiClient.get(`/feature-guides/${surface}`);
    return res.data.data as FeatureGuide | null;
}

export async function getAdminFeatureGuides(): Promise<FeatureGuide[]> {
    const res = await apiClient.get("/admin/feature-guides");
    return res.data.data as FeatureGuide[];
}

export async function saveAdminFeatureGuide(surface: FeatureGuideSurface, input: SaveFeatureGuideInput): Promise<FeatureGuide> {
    const res = await apiClient.put(`/admin/feature-guides/${surface}`, input);
    return res.data.data as FeatureGuide;
}
