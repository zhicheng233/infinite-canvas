"use client";

import { useEffect, useState, useCallback } from "react";
import { Table, Button, Modal, Form, Input, InputNumber, Select, Space, Typography, Tag, Popconfirm, App } from "antd";
import { Plus, Trash2 } from "lucide-react";
import type { ColumnsType } from "antd/es/table";
import { listPricing, savePricing, deletePricing, type PricingItem } from "@/services/api/pricing";

const { Title } = Typography;

const unitLabels: Record<string, string> = {
  per_image: "按图片",
  per_video: "按视频",
  per_token: "按 Token",
};

export default function AdminPricingPage() {
  const { message } = App.useApp();
  const [items, setItems] = useState<PricingItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<PricingItem | null>(null);
  const [saving, setSaving] = useState(false);
  const [form] = Form.useForm();

  const fetch = useCallback(async () => {
    setLoading(true);
    try {
      const data = await listPricing();
      setItems(data);
    } catch (err: any) {
      message.error(err?.message || "获取定价列表失败");
    } finally {
      setLoading(false);
    }
  }, [message]);

  useEffect(() => { fetch(); }, [fetch]);

  const openCreate = () => {
    setEditing(null);
    form.resetFields();
    setModalOpen(true);
  };

  const openEdit = (record: PricingItem) => {
    setEditing(record);
    form.setFieldsValue(record);
    setModalOpen(true);
  };

  const handleSave = async () => {
    try {
      const values = await form.validateFields();
      setSaving(true);
      await savePricing({ ...editing, ...values });
      message.success(editing?.id ? "定价已更新" : "定价已创建");
      setModalOpen(false);
      fetch();
    } catch (err: any) {
      if (err?.message) message.error(err.message);
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (id: number) => {
    try {
      await deletePricing(id);
      message.success("已删除");
      fetch();
    } catch (err: any) {
      message.error(err?.message || "删除失败");
    }
  };

  const columns: ColumnsType<PricingItem> = [
    { title: "ID", dataIndex: "id", key: "id", width: 80 },
    { title: "模型", dataIndex: "model", key: "model" },
    { title: "每次消耗积分", dataIndex: "credits_per_unit", key: "credits_per_unit" },
    {
      title: "计费单位",
      dataIndex: "unit_type",
      key: "unit_type",
      render: (v: string) => <Tag>{unitLabels[v] || v}</Tag>,
    },
    {
      title: "操作",
      key: "actions",
      width: 160,
      render: (_, record) => (
        <Space>
          <Button type="link" size="small" onClick={() => openEdit(record)}>编辑</Button>
          <Popconfirm title="确认删除？" onConfirm={() => handleDelete(record.id!)}>
            <Button type="link" danger size="small" icon={<Trash2 className="size-3" />}>删除</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <Title level={4} className="!mb-0">计费定价</Title>
        <Button type="primary" icon={<Plus className="size-4" />} onClick={openCreate}>新增定价</Button>
      </div>
      <Table rowKey="id" columns={columns} dataSource={items} loading={loading} pagination={false} />

      <Modal
        title={editing?.id ? "编辑定价" : "新增定价"}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={handleSave}
        confirmLoading={saving}
        destroyOnClose
      >
        <Form form={form} layout="vertical" className="mt-4">
          <Form.Item name="model" label="模型名称" rules={[{ required: true, message: "请输入模型名称" }]}>
            <Input placeholder="例如: gpt-4o, dall-e-3" />
          </Form.Item>
          <Form.Item name="credits_per_unit" label="每次消耗积分" rules={[{ required: true, message: "请输入积分" }]}>
            <InputNumber min={1} className="w-full" placeholder="例如: 10" />
          </Form.Item>
          <Form.Item name="unit_type" label="计费单位" initialValue="per_image">
            <Select
              options={[
                { label: "按图片 (per_image)", value: "per_image" },
                { label: "按视频 (per_video)", value: "per_video" },
                { label: "按 Token (per_token)", value: "per_token" },
              ]}
            />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
