"use client";

import { useEffect, useState } from "react";
import {
  App,
  Button,
  Card,
  Form,
  Input,
  InputNumber,
  message,
  Modal,
  Popconfirm,
  Select,
  Space,
  Table,
  Tabs,
  Tag,
} from "antd";
import { Plus, Trash2 } from "lucide-react";
import {
  getApiConfig,
  saveApiConfig,
  type ApiConfigInfo,
} from "@/services/api/api-config";
import {
  listPricing,
  savePricing,
  deletePricing,
  rechargeCredits,
  listUsers,
  type PricingItem,
  type UserItem,
} from "@/services/api/pricing";
import { getBalance } from "@/services/api/credits";
import { useUserStore } from "@/stores/use-user-store";

export default function SettingsPage() {
  const user = useUserStore((s) => s.user);
  const isAdmin = user?.role === "tenant_admin" || user?.role === "super_admin";

  const tabItems = [
    { key: "api", label: "API 配置" },
    { key: "pricing", label: "积分定价" },
    { key: "recharge", label: "积分充值" },
  ];

  return (
    <main className="mx-auto max-w-4xl overflow-y-auto px-6 py-8">
      <h1 className="mb-6 text-2xl font-semibold text-stone-950 dark:text-stone-100">
        系统设置
      </h1>
      <Tabs
        items={tabItems.map((tab) => ({
          key: tab.key,
          label: tab.label,
          children: (
            <TabContent tabKey={tab.key} />
          ),
        }))}
      />
    </main>
  );
}

function TabContent({ tabKey }: { tabKey: string }) {
  const [apiConfig, setApiConfig] = useState<ApiConfigInfo | null>(null);
  const [apiLoading, setApiLoading] = useState(false);
  const [pricingItems, setPricingItems] = useState<PricingItem[]>([]);
  const [pricingLoading, setPricingLoading] = useState(false);
  const [pricingModalOpen, setPricingModalOpen] = useState(false);
  const [editingPricing, setEditingPricing] = useState<PricingItem | null>(null);
  const [pricingForm] = Form.useForm();
  const [rechargeForm] = Form.useForm();
  const [rechargeLoading, setRechargeLoading] = useState(false);
  const [users, setUsers] = useState<UserItem[]>([]);
  const [usersLoading, setUsersLoading] = useState(false);
  const { message } = App.useApp();

  useEffect(() => {
    if (tabKey === "api") fetchApiConfig();
    if (tabKey === "pricing") fetchPricing();
    if (tabKey === "recharge") fetchUsers();
  }, [tabKey]);

  const fetchApiConfig = async () => {
    setApiLoading(true);
    try {
      const config = await getApiConfig();
      setApiConfig(config);
    } catch {
      setApiConfig(null);
    } finally {
      setApiLoading(false);
    }
  };

  const handleApiSave = async (values: { base_url: string; api_key: string }) => {
    try {
      await saveApiConfig(values);
      message.success("API 配置已保存");
      fetchApiConfig();
    } catch (err: any) {
      message.error(err?.message || "保存失败");
    }
  };

  const fetchPricing = async () => {
    setPricingLoading(true);
    try {
      const items = await listPricing();
      setPricingItems(items || []);
    } catch {
      setPricingItems([]);
    } finally {
      setPricingLoading(false);
    }
  };

  const handlePricingSave = async (values: PricingItem) => {
    try {
      await savePricing(values);
      message.success("定价已保存");
      setPricingModalOpen(false);
      fetchPricing();
    } catch (err: any) {
      message.error(err?.message || "保存失败");
    }
  };

  const handlePricingDelete = async (id: number) => {
    try {
      await deletePricing(id);
      message.success("已删除");
      fetchPricing();
    } catch (err: any) {
      message.error(err?.message || "删除失败");
    }
  };

    const [transactions, setTransactions] = useState<any[]>([]);
  const [transactionsLoading, setTransactionsLoading] = useState(false);

  const fetchTransactions = async () => {
    setTransactionsLoading(true);
    try {
      const result = await getTransactions(1, 50);
      setTransactions(result.items || []);
    } catch {
      setTransactions([]);
    } finally {
      setTransactionsLoading(false);
    }
  };

  const fetchUsers = async () => {
    setUsersLoading(true);
    try {
      const result = await listUsers();
      setUsers(result.items || []);
    } catch {
      setUsers([]);
    } finally {
      setUsersLoading(false);
    }
  };

  const handleRecharge = async (values: { user_id: number; amount: number; note?: string }) => {
    setRechargeLoading(true);
    try {
      const result = await rechargeCredits(values);
      message.success(`充值成功！用户 ${result.user_id} 余额: ${result.balance}`);
      rechargeForm.resetFields();
    } catch (err: any) {
      message.error(err?.message || "充值失败");
    } finally {
      setRechargeLoading(false);
    }
  };

  if (tabKey === "api") {
    return (
      <Card loading={apiLoading}>
        <Form
          layout="vertical"
          onFinish={handleApiSave}
          initialValues={{
            base_url: apiConfig?.base_url || "",
            api_key: "",
          }}
          key={apiConfig?.base_url || "empty"}
        >
          <Form.Item
            name="base_url"
            label="上游 API 地址"
            rules={[{ required: true, message: "请输入 API 基础地址" }]}
            extra="例如: https://api.openai.com"
          >
            <Input placeholder="https://api.openai.com" />
          </Form.Item>
          <Form.Item
            name="api_key"
            label="API Key"
            rules={[{ required: true, message: "请输入 API Key" }]}
            extra={apiConfig?.has_key ? "已保存 API Key，重新输入将覆盖" : "首次配置需要输入 API Key"}
          >
            <Input.Password placeholder="sk-..." />
          </Form.Item>
          <Button type="primary" htmlType="submit">
            保存配置
          </Button>
        </Form>
        {apiConfig && (
          <div className="mt-4 text-sm text-stone-500">
            当前已配置 API 地址: {apiConfig.base_url}
            {apiConfig.has_key ? "（已设置 Key）" : "（未设置 Key）"}
          </div>
        )}
      </Card>
    );
  }

  if (tabKey === "pricing") {
    const columns = [
      { title: "模型名称", dataIndex: "model", key: "model" },
      {
        title: "积分/次",
        dataIndex: "credits_per_unit",
        key: "credits_per_unit",
      },
      {
        title: "计费单位",
        dataIndex: "unit_type",
        key: "unit_type",
        render: (v: string) => {
          const labels: Record<string, string> = {
            per_image: "每张图片",
            per_video: "每个视频",
            per_token: "每 Token",
          };
          return <Tag>{labels[v] || v}</Tag>;
        },
      },
      {
        title: "操作",
        key: "actions",
        render: (_: any, record: PricingItem) => (
          <Space>
            <Button
              size="small"
              type="link"
              onClick={() => {
                setEditingPricing(record);
                pricingForm.setFieldsValue(record);
                setPricingModalOpen(true);
              }}
            >
              编辑
            </Button>
            <Popconfirm
              title="确定删除？"
              onConfirm={() => handlePricingDelete(record.id!)}
            >
              <Button size="small" type="link" danger>
                删除
              </Button>
            </Popconfirm>
          </Space>
        ),
      },
    ];

    return (
      <Card
        title="模型定价"
        extra={
          <Button
            type="primary"
            icon={<Plus className="size-4" />}
            onClick={() => {
              setEditingPricing(null);
              pricingForm.resetFields();
              setPricingModalOpen(true);
            }}
          >
            新增定价
          </Button>
        }
        loading={pricingLoading}
      >
        <Table
          dataSource={pricingItems}
          columns={columns}
          rowKey="id"
          pagination={false}
          locale={{ emptyText: "暂无定价配置" }}
        />

        <Modal
          title={editingPricing ? "编辑定价" : "新增定价"}
          open={pricingModalOpen}
          onCancel={() => setPricingModalOpen(false)}
          onOk={() => pricingForm.submit()}
        >
          <Form form={pricingForm} layout="vertical" onFinish={handlePricingSave}>
            <Form.Item
              name="model"
              label="模型名称"
              rules={[{ required: true, message: "请输入模型名称" }]}
            >
              <Input placeholder="例如: gpt-image-2" />
            </Form.Item>
            <Form.Item
              name="credits_per_unit"
              label="每次消耗积分"
              rules={[{ required: true, message: "请输入积分" }]}
            >
              <InputNumber min={1} style={{ width: "100%" }} />
            </Form.Item>
            <Form.Item name="unit_type" label="计费单位" initialValue="per_image">
              <Select
                options={[
                  { label: "每张图片", value: "per_image" },
                  { label: "每个视频", value: "per_video" },
                  { label: "每 Token", value: "per_token" },
                ]}
              />
            </Form.Item>
          </Form>
        </Modal>
      </Card>
    );
  }

  if (tabKey === "recharge") {
    const userOptions = users.map((u) => ({
      label: `${u.display_name || u.username} (${u.username})`,
      value: u.id,
    }));

    return (
      <Card title="用户充值">
        <Form
          form={rechargeForm}
          layout="vertical"
          onFinish={handleRecharge}
          style={{ maxWidth: 400 }}
        >
          <Form.Item
            name="user_id"
            label="选择用户"
            rules={[{ required: true, message: "请选择用户" }]}
          >
            <Select
              showSearch
              placeholder="搜索用户"
              options={userOptions}
              filterOption={(input, option) =>
                (option?.label as string)?.toLowerCase().includes(input.toLowerCase())
              }
            />
          </Form.Item>
          <Form.Item
            name="amount"
            label="充值积分"
            rules={[{ required: true, message: "请输入积分数量" }]}
          >
            <InputNumber min={1} max={100000} style={{ width: "100%" }} placeholder="100" />
          </Form.Item>
          <Form.Item name="note" label="备注">
            <Input placeholder="可选，如：活动赠送" />
          </Form.Item>
          <Button
            type="primary"
            htmlType="submit"
            loading={rechargeLoading}
          >
            确认充值
          </Button>
        </Form>
      </Card>
    );
  }

  if (tabKey === "transactions") {
    const txColumns = [
      {
        title: "类型",
        dataIndex: "type",
        key: "type",
        render: (v: string) => {
          const labels: Record<string, { color: string; text: string }> = {
            earn: { color: "green", text: "收入" },
            spend: { color: "red", text: "消费" },
            refund: { color: "blue", text: "退款" },
            adjust: { color: "orange", text: "调整" },
          };
          const info = labels[v] || { color: "default", text: v };
          return <Tag color={info.color}>{info.text}</Tag>;
        },
      },
      { title: "数量", dataIndex: "amount", key: "amount" },
      {
        title: "余额",
        dataIndex: "balance_after",
        key: "balance_after",
      },
      { title: "说明", dataIndex: "note", key: "note" },
      {
        title: "时间",
        dataIndex: "created_at",
        key: "created_at",
        render: (v: string) => new Date(v).toLocaleString("zh-CN"),
      },
    ];

    return (
      <Card loading={transactionsLoading}>
        <Table
          dataSource={transactions}
          columns={txColumns}
          rowKey="id"
          pagination={false}
          locale={{ emptyText: "暂无积分记录" }}
        />
      </Card>
    );
  }

  return null;
}
