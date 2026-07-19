import apiClient from "./client";

export type ApiUser = {
    id: number;
    username: string;
    display_name: string;
    avatar_url: string;
    tenant_id: number;
    role: string;
};

type AuthResponse = { token: string; user: ApiUser };

type CaptchaData = {
    captcha_id: string;
    svg: string;
};

export async function fetchCaptcha(): Promise<CaptchaData> {
    const res = await apiClient.get("/auth/captcha");
    return res.data.data as CaptchaData;
}

export async function register(input: { tenant_name?: string; username: string; password: string; captcha_id?: string; captcha_answer?: string }) {
    const res = await apiClient.post("/auth/register", input);
    return res.data.data as AuthResponse;
}

export async function login(input: { username: string; password: string }) {
    const res = await apiClient.post("/auth/login", input);
    return res.data.data as AuthResponse;
}

export async function getMe() {
    const res = await apiClient.get("/auth/me");
    return res.data.data as ApiUser;
}

export async function changePassword(input: { old_password: string; new_password: string }) {
    const res = await apiClient.put("/auth/password", input);
    return res.data;
}

export async function updateProfile(input: { display_name?: string; avatar_url?: string }) {
    const res = await apiClient.put("/auth/profile", input);
    return res.data.data as ApiUser;
}
