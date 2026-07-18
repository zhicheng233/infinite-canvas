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
  model_video_durations?: Record<string, number[]>;
  model_video_customizable?: Record<string, boolean>;
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
  model_video_durations?: Record<string, number[]>;
  model_video_customizable?: Record<string, boolean>;
};

export type ApiModelTestInput = {
  model: string;
  generation: string;
  route?: string;
  prompt?: string;
};

export type ApiModelTestResult = {
  success: boolean;
  model: string;
  generation: string;
  route: string;
  method: string;
  path: string;
  status_code: number;
  response_time_ms: number;
  error_message?: string;
  response_body?: string;
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
  model_video_durations?: Record<string, number[]>;
  model_video_customizable?: Record<string, boolean>;
}) {
  const res = await apiClient.post("/api-config", input);
  return res.data.data as { saved: boolean };
}

export async function testApiModel(input: ApiModelTestInput) {
  const res = await apiClient.post("/api-config/test-model", input);
  return res.data.data as ApiModelTestResult;
}
