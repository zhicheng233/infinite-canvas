# 图片文生图/图生图路由拆分 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将图片模型的文生图与图生图路由拆分配置，避免同一模型两类请求互相冲突。

**Architecture:** 在前端 store 与后台管理页中引入两个独立图片路由字段，并保留对旧 `image:<model>` 配置的图生图兼容读取。生成与编辑请求分别按各自路由决定调用 `/images/generations`、`/images/edits` 或 `/chat/completions`。

**Tech Stack:** Next.js App Router、TypeScript、Zustand、Ant Design、Axios

## Global Constraints

- 页面文案保持中文。
- 最小改动完成，不顺手重构无关代码。
- 兼容当前线上 `model_routes` 老数据，不要求用户手工改库。
- 业务请求统一沿用 `web/src/services/api/` 现有封装。

---

### Task 1: 拆分图片路由读取能力

**Files:**
- Modify: `web/src/stores/use-config-store.ts`

**Interfaces:**
- Consumes: `config.modelRoutes: Record<string, string>`
- Produces: `imageGenerateRouteForModel(config, value)`, `imageEditRouteForModel(config, value)`

- [ ] 新增文生图与图生图路由键读取函数。
- [ ] 保留旧 `image:<model>` 仅作为图生图兼容回退。
- [ ] 保持现有视频路由逻辑不变。

### Task 2: 修正文生图与图生图请求分流

**Files:**
- Modify: `web/src/services/api/image.ts`

**Interfaces:**
- Consumes: `imageGenerateRouteForModel()`, `imageEditRouteForModel()`
- Produces: `requestGeneration()` 与 `requestEdit()` 的正确路由行为

- [ ] 文生图按“文生图路由”决定走 `generations` 或 `chat`。
- [ ] 图生图按“图生图路由”决定走 `edits` 或 `chat`。
- [ ] 若图生图被配置成 `generations`，返回明确中文错误提示。

### Task 3: 调整后台 API 配置页

**Files:**
- Modify: `web/src/app/(user)/admin/api-config/page.tsx`

**Interfaces:**
- Consumes: `model_routes`
- Produces: `image_generate_route`, `image_edit_route` 两项 UI 与保存逻辑

- [ ] 将行数据结构拆成“文生图接口”和“图生图接口”。
- [ ] 保存时写入 `image_generate:<model>` 与 `image_edit:<model>`。
- [ ] 初始化时兼容旧 `image:<model>` 到图生图字段。
- [ ] 调整高级配置文案，避免“图片接口”歧义。

### Task 4: 更新文档与待测项

**Files:**
- Modify: `docs/content/docs/progress/pending-test.mdx`
- Modify: `docs/content/docs/overview/features.mdx`

**Interfaces:**
- Consumes: 最新后台配置行为
- Produces: 用户可测试说明

- [ ] 将“图片模型单独指定接口路由”描述改为区分文生图与图生图。
- [ ] 补充兼容旧配置与验证点说明。

### Task 5: 构建与线上验证

**Files:**
- Modify: 无

**Interfaces:**
- Consumes: 前端构建、线上部署
- Produces: 可验证的运行结果

- [ ] 运行前端构建，确认 TypeScript 与 Next 构建通过。
- [ ] 部署到 `23.106.44.56:/data/infinite-canvas`。
- [ ] 验证后台配置页与目标图生图链路恢复正常。
