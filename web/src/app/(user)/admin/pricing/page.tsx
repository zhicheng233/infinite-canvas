"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { Spin } from "antd";

export default function AdminPricingPage() {
    const router = useRouter();

    useEffect(() => {
        router.replace("/admin/api-config");
    }, [router]);

    return (
        <div className="flex min-h-[240px] items-center justify-center">
            <Spin tip="正在跳转到统一配置页..." />
        </div>
    );
}
