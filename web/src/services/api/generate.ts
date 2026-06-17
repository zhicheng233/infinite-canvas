import apiClient from "./client";

export type GenerateResult = {
  data: any;
  cost: number;
  balance: number;
};

export async function generateImage(body: Record<string, unknown>) {
  const res = await apiClient.post("/generate/image", body);
  return res.data.data as GenerateResult;
}

export async function generateText(body: Record<string, unknown>) {
  const res = await apiClient.post("/generate/text", body);
  return res.data.data as GenerateResult;
}

export async function generateVideo(body: Record<string, unknown>) {
  const res = await apiClient.post("/generate/video", body);
  return res.data.data as GenerateResult;
}

export async function generateAudio(body: Record<string, unknown>) {
  const res = await apiClient.post("/generate/audio", body);
  return res.data.data as GenerateResult;
}
