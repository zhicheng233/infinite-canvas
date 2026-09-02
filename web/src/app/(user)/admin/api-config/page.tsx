"use client";

import { Suspense, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { Tabs, Typography } from "antd";
import { Boxes, Cable, Coins, Route, SlidersHorizontal } from "lucide-react";

import { AutoRoutingPools } from "./components/auto-routing-pools";
import { ModelServiceChannels } from "./components/model-service-channels";
import { ModelServiceModels } from "./components/model-service-models";
import { ModelServicePresets } from "./components/model-service-presets";
import { ModelServicePricing } from "./components/model-service-pricing";

const sections = ["channels", "models", "pricing", "routing", "presets"] as const;
type Section = (typeof sections)[number];

export default function AdminModelServicePage() {
    return (
        <Suspense fallback={null}>
            <AdminModelServiceContent />
        </Suspense>
    );
}

function AdminModelServiceContent() {
    const router = useRouter();
    const pathname = usePathname();
    const searchParams = useSearchParams();
    const section = sections.includes(searchParams.get("section") as Section) ? (searchParams.get("section") as Section) : "channels";
    const view = searchParams.get("view") === "model" ? "model" : "channel";
    const [refreshToken, setRefreshToken] = useState(0);
    const changed = () => setRefreshToken((value) => value + 1);

    const updateQuery = (nextSection: Section, nextView = view) => {
        const query = new URLSearchParams(searchParams.toString());
        query.set("section", nextSection);
        if (nextSection === "models") query.set("view", nextView);
        router.replace(`${pathname}?${query.toString()}`);
    };

    return (
        <div className="mx-auto max-w-[1500px]">
            <div className="mb-5">
                <Typography.Title level={4} className="!mb-1">模型服务</Typography.Title>
                <Typography.Text type="secondary">统一维护渠道接入、公开模型、操作协议、定价和智能路由。</Typography.Text>
            </div>
            <Tabs
                activeKey={section}
                onChange={(key) => updateQuery(key as Section)}
                items={[
                    { key: "channels", label: <span className="inline-flex items-center gap-2"><Cable className="size-4" />渠道</span>, children: <ModelServiceChannels refreshToken={refreshToken} onChanged={changed} /> },
                    { key: "models", label: <span className="inline-flex items-center gap-2"><Boxes className="size-4" />模型目录</span>, children: <ModelServiceModels view={view} onViewChange={(nextView) => updateQuery("models", nextView)} refreshToken={refreshToken} onChanged={changed} /> },
                    { key: "pricing", label: <span className="inline-flex items-center gap-2"><Coins className="size-4" />定价</span>, children: <ModelServicePricing refreshToken={refreshToken} onChanged={changed} /> },
                    { key: "routing", label: <span className="inline-flex items-center gap-2"><Route className="size-4" />智能路由</span>, children: <AutoRoutingPools /> },
                    { key: "presets", label: <span className="inline-flex items-center gap-2"><SlidersHorizontal className="size-4" />视频预设</span>, children: <ModelServicePresets refreshToken={refreshToken} onChanged={changed} /> },
                ]}
            />
        </div>
    );
}
