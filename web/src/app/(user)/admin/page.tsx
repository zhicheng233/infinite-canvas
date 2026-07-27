"use client";

import { useEffect, useState, useCallback } from "react";
import { Button, Card, Statistic, Row, Col, Typography, App, Spin, Switch, Input } from "antd";
import { Users, TrendingUp, TrendingDown, CreditCard, Megaphone } from "lucide-react";
import { getAdminStats, type AdminStats } from "@/services/api/admin";
import { getAdminAnnouncement, saveAdminAnnouncement, type SiteAnnouncement } from "@/services/api/announcement";
import { useUserStore } from "@/stores/use-user-store";

const { Title } = Typography;
const { TextArea } = Input;

export default function AdminDashboardPage() {
    const { message } = App.useApp();
    const user = useUserStore((state) => state.user);
    const isSuperAdmin = user?.role === "super_admin";
    const [stats, setStats] = useState<AdminStats | null>(null);
    const [announcement, setAnnouncement] = useState<SiteAnnouncement>({ enabled: false, title: "公告", content: "", version: 1 });
    const [loading, setLoading] = useState(true);
    const [savingAnnouncement, setSavingAnnouncement] = useState(false);

    const fetchStats = useCallback(async () => {
        setLoading(true);
        try {
            const [statsData, announcementData] = await Promise.all([getAdminStats(), isSuperAdmin ? getAdminAnnouncement() : Promise.resolve(null)]);
            setStats(statsData);
            if (announcementData) {
                setAnnouncement({ enabled: Boolean(announcementData.enabled), title: announcementData.title || "公告", content: announcementData.content || "", version: announcementData.version || 1 });
            }
        } catch (err: any) {
            message.error(err?.message || "获取统计数据失败");
        } finally {
            setLoading(false);
        }
    }, [isSuperAdmin, message]);

    const saveAnnouncement = async () => {
        setSavingAnnouncement(true);
        try {
            const next = await saveAdminAnnouncement(announcement);
            setAnnouncement({ enabled: Boolean(next.enabled), title: next.title || "公告", content: next.content || "", version: next.version || 1 });
            message.success("公告已保存");
        } catch (err: any) {
            message.error(err?.message || "保存公告失败");
        } finally {
            setSavingAnnouncement(false);
        }
    };

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
            {isSuperAdmin && (
                <Card
                    className="mt-4"
                    title={
                        <span className="inline-flex items-center gap-2">
                            <Megaphone className="size-4 text-blue-500" />
                            全局公告
                        </span>
                    }
                    extra={<span className="text-xs text-stone-400">版本 {announcement.version || 1}</span>}
                >
                    <div className="grid gap-4">
                        <div className="flex items-center justify-between gap-3">
                            <div>
                                <div className="text-sm font-medium">访问时弹出公告</div>
                                <div className="text-xs text-stone-500">用户勾选“本次更新前不再弹出”后，公告更新前不会再次弹出。</div>
                            </div>
                            <Switch checked={announcement.enabled} onChange={(enabled) => setAnnouncement((value) => ({ ...value, enabled }))} />
                        </div>
                        <Input value={announcement.title} maxLength={100} placeholder="公告标题" onChange={(event) => setAnnouncement((value) => ({ ...value, title: event.target.value }))} />
                        <TextArea value={announcement.content} maxLength={2000} rows={6} showCount placeholder="请输入公告内容" onChange={(event) => setAnnouncement((value) => ({ ...value, content: event.target.value }))} />
                        <div className="flex justify-end">
                            <Button type="primary" loading={savingAnnouncement} onClick={saveAnnouncement}>
                                保存公告
                            </Button>
                        </div>
                    </div>
                </Card>
            )}
        </div>
    );
}
