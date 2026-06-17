import apiClient from "./client";

export type ApiConfigInfo = {
  base_url: string;
  has_key: boolean;
};

export async function getApiConfig() {
  const res = await apiClient.get("/api-config");
  return res.data.data as ApiConfigInfo;
}

export async function saveApiConfig(input: { base_url: string; api_key: string }) {
  const res = await apiClient.post("/api-config", input);
  return res.data.data as { saved: boolean };
}
