import { FileText, ImagePlus, Images, Maximize2, Shield, Video, Zap } from "lucide-react";

const ENABLE_VIDEO = process.env.NEXT_PUBLIC_ENABLE_VIDEO !== "false";

export const navigationTools = [
  { slug: "canvas" as const, label: "我的画布", icon: Maximize2 },
  { slug: "image" as const, label: "生图工作台", icon: ImagePlus },
  ...(ENABLE_VIDEO ? [{ slug: "video" as const, label: "视频创作台", icon: Video }] : []),
  { slug: "prompts" as const, label: "提示词库", icon: FileText },
  { slug: "assets" as const, label: "我的素材", icon: Images },
  { slug: "recharge" as const, label: "积分充值", icon: Zap },
  { slug: "admin" as const, label: "管理后台", icon: Shield, adminOnly: true },
] as const;

export type NavigationToolSlug = (typeof navigationTools)[number]["slug"];
