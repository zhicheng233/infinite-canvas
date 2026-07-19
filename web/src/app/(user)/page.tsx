"use client";

import { ArrowRight, CheckCircle2, Coins, ImageIcon, Layers, LogIn, Music2, ShieldCheck, Sparkles, UserPlus, Video, Workflow } from "lucide-react";
import { type ReactNode, useEffect, useState } from "react";
import { App, Button, Image, Tag } from "antd";
import { useRouter } from "next/navigation";

import { fetchPrompts, type Prompt } from "@/services/api/prompts";
import { navigationTools } from "@/constant/navigation-tools";
import { getStoredToken } from "@/services/api/client";
import { cn } from "@/lib/utils";
import { useUserStore } from "@/stores/use-user-store";
import { PromptCover } from "@/components/prompts/prompt-cover";

function Highlighter({ action, color, children }: { action: "highlight" | "underline"; color: string; children: ReactNode }) {
    return (
        <span className="relative inline-block px-1">
            {action === "highlight" ? (
                <span className="absolute inset-x-0 bottom-0 top-1 rounded-sm opacity-45" style={{ backgroundColor: color }} />
            ) : (
                <span className="absolute inset-x-0 bottom-0 h-1 rounded-full opacity-80" style={{ backgroundColor: color }} />
            )}
            <span className="relative font-medium text-stone-800 dark:text-stone-200">{children}</span>
        </span>
    );
}

const features = [
    { icon: ImageIcon, title: "AI 图片生成", desc: "文生图、图生图、多角度生成，支持多种主流模型" },
    { icon: Video, title: "视频创作", desc: "AI 视频生成，将静态创意转化为动态作品" },
    { icon: Music2, title: "音频生成", desc: "AI 配音与音效，为创作增添声音维度" },
    { icon: Layers, title: "无限画布", desc: "自由连接节点，构建复杂创作工作流" },
];

const scenes = ["电商主图与详情页", "短视频分镜与素材", "品牌视觉探索", "广告创意批量变体"];
const steps = ["选择模型与比例", "输入提示词或参考图", "生成并在画布继续迭代"];

function PublicHome() {
    const router = useRouter();

    return (
        <main className="relative h-full overflow-y-auto bg-background bg-[radial-gradient(#e5e7eb_1px,transparent_1px)] [background-size:16px_16px] text-stone-950 dark:bg-[radial-gradient(rgba(245,245,244,.18)_1px,transparent_1px)] dark:text-stone-100">
            <section className="relative mx-auto min-h-[calc(100vh-4rem)] max-w-7xl overflow-hidden px-6 pb-20">
                <div className="pointer-events-none absolute left-[15%] top-24 size-20 rounded-full border border-dashed border-stone-200 dark:border-stone-800" />
                <div className="pointer-events-none absolute right-[23%] top-[48%] size-20 rounded-full border border-dashed border-stone-200 dark:border-stone-800" />

                <div className="relative grid min-h-[620px] items-center gap-10 pt-10 lg:grid-cols-[1.05fr_.95fr]">
                    <div className="text-center lg:text-left">
                        <div className="mb-5 inline-flex items-center gap-2 rounded-full border border-stone-200 bg-white/70 px-3 py-1 text-sm text-stone-600 shadow-sm backdrop-blur dark:border-stone-800 dark:bg-stone-950/60 dark:text-stone-300">
                            <Sparkles className="size-4 text-amber-500" />
                            面向商业创作的 AI 内容工作台
                        </div>
                        <h1 className="ai-title-aurora max-w-5xl text-balance text-5xl font-semibold tracking-normal sm:text-7xl lg:text-8xl">一站式 AI 创作画布</h1>
                        <p className="mt-8 max-w-3xl text-balance text-lg leading-8 text-stone-500 dark:text-stone-400">
                            把
                            <Highlighter action="highlight" color="#87CEFA">
                                图片、视频、音频
                            </Highlighter>
                            生成和
                            <Highlighter action="underline" color="#FF9800">
                                无限画布工作流
                            </Highlighter>
                            放在一起，从灵感、参考图到批量变体，都能在一个空间里持续迭代。
                        </p>
                        <div className="mt-6 flex flex-wrap justify-center gap-2 lg:justify-start">
                            {scenes.map((item) => (
                                <Tag key={item} className="m-0 rounded-full px-3 py-1 text-sm">
                                    {item}
                                </Tag>
                            ))}
                        </div>
                        <div className="mt-10 flex flex-wrap items-center justify-center gap-3 lg:justify-start">
                            <Button type="primary" size="large" onClick={() => router.push("/auth/register")} icon={<UserPlus className="size-4" />} iconPlacement="end">
                                免费注册
                            </Button>
                            <Button size="large" onClick={() => router.push("/auth/login")} icon={<LogIn className="size-4" />} iconPlacement="end">
                                登录
                            </Button>
                        </div>
                    </div>
                    <div className="rounded-[2rem] border border-stone-200 bg-white/80 p-4 shadow-2xl shadow-stone-200/60 backdrop-blur dark:border-stone-800 dark:bg-stone-950/70 dark:shadow-black/30">
                        <div className="rounded-[1.5rem] bg-stone-950 p-4 text-white">
                            <div className="mb-4 flex items-center justify-between">
                                <div className="text-sm text-white/60">Creative Canvas</div>
                                <div className="rounded-full bg-emerald-400/15 px-2 py-1 text-xs text-emerald-200">运行中</div>
                            </div>
                            <div className="grid gap-3 sm:grid-cols-2">
                                <PreviewCard icon={<ImageIcon className="size-5" />} title="生图" desc="多模型统一调用" />
                                <PreviewCard icon={<Video className="size-5" />} title="视频" desc="参考图转动态镜头" />
                                <PreviewCard icon={<Workflow className="size-5" />} title="画布" desc="节点连接上下文" />
                                <PreviewCard icon={<Coins className="size-5" />} title="积分" desc="生成前透明预估" />
                            </div>
                            <div className="mt-4 rounded-2xl border border-white/10 bg-white/5 p-4">
                                <div className="mb-3 text-sm font-medium">从提示词到商业素材</div>
                                <div className="space-y-2">
                                    {steps.map((item, index) => (
                                        <div key={item} className="flex items-center gap-2 text-sm text-white/75">
                                            <span className="inline-flex size-5 items-center justify-center rounded-full bg-white/10 text-xs">{index + 1}</span>
                                            {item}
                                        </div>
                                    ))}
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                <section className="relative mx-auto mb-16 max-w-6xl border-t border-stone-200 pt-12 dark:border-stone-800">
                    <div className="mb-10 text-center">
                        <h2 className="text-3xl font-semibold text-stone-950 dark:text-stone-100">为商业创作准备的核心能力</h2>
                        <p className="mt-3 text-base leading-7 text-stone-500 dark:text-stone-400">统一模型配置、积分计费和创作资产沉淀，让团队不用反复切换工具。</p>
                    </div>
                    <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-4">
                        {features.map((feature) => {
                            const Icon = feature.icon;
                            return (
                                <div key={feature.title} className="group rounded-2xl border border-stone-200 bg-stone-50/80 p-6 transition hover:border-stone-300 hover:shadow-sm dark:border-stone-800 dark:bg-stone-900/50 dark:hover:border-stone-700">
                                    <div className="mb-4 inline-flex size-12 items-center justify-center rounded-xl bg-stone-200/80 dark:bg-stone-800">
                                        <Icon className="size-6 text-stone-600 dark:text-stone-300" />
                                    </div>
                                    <h3 className="mb-2 text-lg font-semibold text-stone-950 dark:text-stone-100">{feature.title}</h3>
                                    <p className="text-sm leading-6 text-stone-500 dark:text-stone-400">{feature.desc}</p>
                                </div>
                            );
                        })}
                    </div>
                </section>

                <section className="mx-auto grid max-w-6xl gap-6 md:grid-cols-3">
                    <InfoCard icon={<Coins className="size-5" />} title="积分制透明扣费" desc="后台统一设置模型价格，生成前展示预计积分，用户余额和扣费记录清晰可查。" />
                    <InfoCard icon={<ShieldCheck className="size-5" />} title="统一 API 管理" desc="用户侧不接触 API Key，模型路由、计费和上游地址都由管理员控制。" />
                    <InfoCard icon={<CheckCircle2 className="size-5" />} title="结果可沉淀复用" desc="提示词、素材、画布项目和生成记录围绕账号沉淀，方便继续迭代。" />
                </section>

                <section className="mx-auto mt-16 max-w-4xl rounded-[2rem] border border-stone-200 bg-stone-950 px-6 py-10 text-center text-white shadow-xl dark:border-stone-800">
                    <h2 className="text-3xl font-semibold">开始搭建你的 AI 创作工作流</h2>
                    <p className="mx-auto mt-3 max-w-2xl text-sm leading-7 text-white/65">注册后即可进入图片、视频和无限画布工作台。管理员可在后台配置模型、价格和积分。</p>
                    <div className="mt-10 flex flex-wrap items-center justify-center gap-3">
                        <Button type="primary" size="large" onClick={() => router.push("/auth/register")} icon={<UserPlus className="size-4" />} iconPlacement="end">
                            免费注册
                        </Button>
                        <Button size="large" onClick={() => router.push("/auth/login")} icon={<LogIn className="size-4" />} iconPlacement="end">
                            登录
                        </Button>
                    </div>
                </section>
            </section>
        </main>
    );
}

function PreviewCard({ icon, title, desc }: { icon: ReactNode; title: string; desc: string }) {
    return (
        <div className="rounded-2xl border border-white/10 bg-white/5 p-4">
            <div className="mb-3 inline-flex size-10 items-center justify-center rounded-xl bg-white/10 text-sky-200">{icon}</div>
            <div className="font-medium">{title}</div>
            <div className="mt-1 text-xs text-white/55">{desc}</div>
        </div>
    );
}

function InfoCard({ icon, title, desc }: { icon: ReactNode; title: string; desc: string }) {
    return (
        <div className="rounded-2xl border border-stone-200 bg-white/70 p-6 shadow-sm backdrop-blur dark:border-stone-800 dark:bg-stone-950/60">
            <div className="mb-4 inline-flex size-11 items-center justify-center rounded-xl bg-stone-100 text-stone-700 dark:bg-stone-900 dark:text-stone-200">{icon}</div>
            <h3 className="text-lg font-semibold">{title}</h3>
            <p className="mt-2 text-sm leading-6 text-stone-500 dark:text-stone-400">{desc}</p>
        </div>
    );
}

export default function IndexPage() {
    const { message } = App.useApp();
    const user = useUserStore((state) => state.user);
    const isAdmin = user?.role === "super_admin" || user?.role === "tenant_admin";
    const visibleTools = navigationTools.filter((tool) => !tool.adminOnly || isAdmin);
    const [primaryTool] = visibleTools;
    const [promptShowcase, setPromptShowcase] = useState<Prompt[]>([]);
    const [previewIndex, setPreviewIndex] = useState(0);
    const [previewOpen, setPreviewOpen] = useState(false);
    const [isLoggedIn, setIsLoggedIn] = useState(false);
    const [mounted, setMounted] = useState(false);

    useEffect(() => {
        setIsLoggedIn(!!getStoredToken());
        setMounted(true);
    }, []);

    useEffect(() => {
        if (!isLoggedIn) return;
        void fetchPrompts({ pageSize: 12 })
            .then((data) => setPromptShowcase(data.items))
            .catch((error) => message.error(error instanceof Error ? error.message : "获取提示词失败"));
    }, [isLoggedIn, message]);

    if (!mounted) return null;

    if (!isLoggedIn) return <PublicHome />;

    return (
        <main className="relative h-full overflow-y-auto bg-background bg-[radial-gradient(#e5e7eb_1px,transparent_1px)] [background-size:16px_16px] text-stone-950 dark:bg-[radial-gradient(rgba(245,245,244,.18)_1px,transparent_1px)] dark:text-stone-100">
            <section className="relative mx-auto min-h-[calc(100vh-4rem)] max-w-7xl overflow-hidden px-6">
                <div className="pointer-events-none absolute left-[15%] top-24 size-20 rounded-full border border-dashed border-stone-200 dark:border-stone-800" />
                <div className="pointer-events-none absolute right-[23%] top-[48%] size-20 rounded-full border border-dashed border-stone-200 dark:border-stone-800" />

                <div className="relative flex min-h-[620px] flex-col items-center justify-center pt-10 text-center">
                    <h1 className="ai-title-aurora max-w-5xl text-balance text-5xl font-semibold tracking-normal sm:text-7xl lg:text-8xl">无限画布</h1>
                    <p className="mt-8 max-w-3xl text-balance text-lg leading-8 text-stone-500 dark:text-stone-400">
                        在
                        <Highlighter action="underline" color="#FF9800">
                            无限画布
                        </Highlighter>
                        中生成、连接和重组
                        <Highlighter action="highlight" color="#87CEFA">
                            图片、文字与图形
                        </Highlighter>
                        ，让创作从单次生成变成连续推演。
                    </p>
                    <div className="mt-10 flex flex-wrap items-center justify-center gap-3">
                        <Button type="primary" size="large" href={"/" + primaryTool.slug} icon={<ArrowRight className="size-4" />} iconPlacement="end">
                            开始使用
                        </Button>
                        <Button size="large" href="/canvas">
                            打开画布
                        </Button>
                    </div>
                </div>

                <section className="relative mx-auto mb-20 max-w-6xl border-t border-stone-200 pt-12 dark:border-stone-800">
                    <div className="mb-8 grid gap-4 md:grid-cols-[1fr_auto_1fr] md:items-start">
                        <div />
                        <div className="max-w-2xl text-center">
                            <h2 className="text-3xl font-semibold text-stone-950 dark:text-stone-100">沉淀每一次好结果</h2>
                            <p className="mt-3 text-base leading-7 text-stone-500 dark:text-stone-400">收藏稳定出图的提示词、参考风格和结果图片，让下一次创作从已有经验开始。</p>
                        </div>
                        <Button type="link" href="/prompts" className="justify-self-center md:justify-self-end" icon={<ArrowRight className="size-4" />} iconPlacement="end">
                            查看提示词库
                        </Button>
                    </div>
                    <div className="grid auto-rows-[210px] gap-4 md:grid-cols-4">
                        {promptShowcase.map((item, index) => (
                            <button
                                key={item.id}
                                type="button"
                                onClick={() => {
                                    setPreviewIndex(index);
                                    setPreviewOpen(true);
                                }}
                                className={cn(
                                    "group relative cursor-pointer overflow-hidden border border-stone-200 bg-stone-100 text-left dark:border-stone-800 dark:bg-stone-900",
                                    index === 0 && "md:col-span-2 md:row-span-2",
                                    index === 3 && "md:col-span-2",
                                )}
                            >
                                <PromptCover src={item.coverUrl} title={item.title} className="h-full w-full object-cover transition duration-500 group-hover:scale-[1.03]" />
                                <div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/70 via-black/35 to-transparent p-4 text-white">
                                    <div className="mb-2 flex flex-wrap gap-1.5">
                                        {item.tags.slice(0, 2).map((tag) => (
                                            <Tag key={tag} variant="filled" className="m-0 bg-white/15 text-[11px] text-white backdrop-blur">
                                                {tag}
                                            </Tag>
                                        ))}
                                    </div>
                                    <h3 className="text-sm font-medium">{item.title}</h3>
                                    <p className="mt-1 line-clamp-2 text-xs leading-5 text-white/75">{item.prompt}</p>
                                </div>
                            </button>
                        ))}
                    </div>
                </section>
            </section>
            <Image.PreviewGroup
                preview={{
                    open: previewOpen,
                    current: previewIndex,
                    onOpenChange: setPreviewOpen,
                    onChange: setPreviewIndex,
                }}
            >
                <div className="hidden">
                    {promptShowcase.map((item) => (
                        <Image key={item.id} src={item.coverUrl} alt={item.title} />
                    ))}
                </div>
            </Image.PreviewGroup>
        </main>
    );
}
