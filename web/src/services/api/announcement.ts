import apiClient from "./client";

export type SiteAnnouncement = {
    enabled: boolean;
    title: string;
    content: string;
    version: number;
};

export async function getAnnouncement() {
    const res = await apiClient.get("/announcement");
    return res.data.data as SiteAnnouncement;
}

export async function getAdminAnnouncement() {
    const res = await apiClient.get("/admin/announcement");
    return res.data.data as SiteAnnouncement;
}

export async function saveAdminAnnouncement(input: SiteAnnouncement) {
    const res = await apiClient.post("/admin/announcement", input);
    return res.data.data as SiteAnnouncement;
}
