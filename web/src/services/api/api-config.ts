import apiClient from "./client";
import type { PricingItem } from "./pricing";

export type ApiConfigInfo = {
  base_url: string;
  has_key: boolean;
  models?: string[];
  image_models?: string[];
  video_models?: string[];
  text_models?: string[];
  audio_models?: string[];
  model_routes?: Record<string, string>;
};

export type ApiModelCatalog = {
  models?: string[];
  image_models?: string[];
  video_models?: string[];
  text_models?: string[];
  audio_models?: string[];
  priced_models?: string[];
  disabled_models?: string[];
  enabled_count?: number;
  total_models?: number;
  pricing_map?: Record<string, PricingItem>;
  model_routes?: Record<string, string>;
};

export async function getApiConfig() {
  const res = await apiClient.get("/api-config");
  return res.data.data as ApiConfigInfo;
}

export async function getApiModelCatalog() {
  const res = await apiClient.get("/api-config/catalog");
  return res.data.data as ApiModelCatalog;
}

export async function saveApiConfig(input: {
  base_url: string;
  api_key: string;
  models?: string[];
  image_models?: string[];
  video_models?: string[];
  text_models?: string[];
  audio_models?: string[];
  model_routes?: Record<string, string>;
}) {
  const res = await apiClient.post("/api-config", input);
  return res.data.data as { saved: boolean };
}
