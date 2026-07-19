"use client";

import { useEffect, useState, useCallback } from "react";
import { Card, Statistic, Row, Col, Typography, App, Spin } from "antd";
import { Users, TrendingUp, TrendingDown, CreditCard } from "lucide-react";
import { getAdminStats, type AdminStats } from "@/services/api/admin";

const { Title } = Typography;

export default function AdminDashboardPage() {
    const { message } = App.useApp();
    const [stats, setStats] = useState<AdminStats | null>(null);
    const [loading, setLoading] = useState(true);

    const fetchStats = useCallback(async () => {
        setLoading(true);
        try {
            const data = await getAdminStats();
            setStats(data);
        } catch (err: any) {
            message.error(err?.message || "获取统计数据失败");
        } finally {
            setLoading(false);
        }
    }, [message]);

    useEffect(() => {
        fetchStats();
    }, [fetchStats]);

    if (loading) {
        return (
            <div className="flex items-center justify-center h-64">
                <Spin size="large" />
            </div>
        );
    }

    return (
        <div>
            <Title level={4} className="!mb-6">
                管理概览
            </Title>
            <Row gutter={[16, 16]}>
                <Col xs={24} sm={12} lg={6}>
                    <Card>
                        <Statistic title="用户总数" value={stats?.total_users ?? 0} prefix={<Users className="size-5 text-blue-500" />} />
                    </Card>
                </Col>
                <Col xs={24} sm={12} lg={6}>
                    <Card>
                        <Statistic title="累计发放积分" value={stats?.total_credits_earned ?? 0} prefix={<TrendingUp className="size-5 text-green-500" />} />
                    </Card>
                </Col>
                <Col xs={24} sm={12} lg={6}>
                    <Card>
                        <Statistic title="累计消耗积分" value={stats?.total_credits_spent ?? 0} prefix={<TrendingDown className="size-5 text-orange-500" />} />
                    </Card>
                </Col>
                <Col xs={24} sm={12} lg={6}>
                    <Card>
                        <Statistic title="充值总额" value={stats?.total_recharged ?? 0} prefix={<CreditCard className="size-5 text-purple-500" />} suffix="分" />
                    </Card>
                </Col>
            </Row>
        </div>
    );
}
