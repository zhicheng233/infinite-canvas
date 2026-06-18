"use client";

import { useEffect, useState } from "react";
import { App, Button, Card, Form, Input } from "antd";
import { Settings } from "lucide-react";
import { getApiConfig, saveApiConfig, type ApiConfigInfo } from "@/services/api/api-config";

export default function AdminApiConfigPage() {
  const { message } = App.useApp();
  const [apiConfig, setApiConfig] = useState<ApiConfigInfo | null>(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [form] = Form.useForm();

  const fetchApiConfig = async () => {
    setLoading(true);
    try {
      const config = await getApiConfig();
      setApiConfig(config);
      form.setFieldsValue({ base_url: config?.base_url || "", api_key: "" });
    } catch {
      setApiConfig(null);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchApiConfig();
  }, []);

  const handleSave = async (values: { base_url: string; api_key: string }) => {
    setSaving(true);
    try {
      await saveApiConfig(values);
      message.success("API 配置已保存");
      fetchApiConfig();
    } catch (err: any) {
      message.error(err?.message || "保存失败");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div>
      <h2 className="mb-6 text-xl font-semibold text-stone-950 dark:text-stone-100">
        <Settings className="size-5 inline mr-2" />
        API 配置
      </h2>
      <Card loading={loading}>
        <Form
          layout="vertical"
          onFinish={handleSave}
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
            extra="请输入 OpenAI 兼容 API 根地址，例如: https://api.openai.com 或 http://8.219.243.189:3000；系统会自动拼接 /v1"
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
          <Button type="primary" htmlType="submit" loading={saving}>
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
    </div>
  );
}
