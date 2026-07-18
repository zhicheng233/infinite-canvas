# 图片文生图/图生图路由拆分设计

## 背景

当前后台对图片模型只有一个“图片接口”配置，但前端实际存在两条不同链路：

- 文生图：`/v1/images/generations`
- 图生图：`/v1/images/edits` 或 `/v1/chat/completions`

这会导致同一个模型无法同时表达“文生图走 A、图生图走 B”。线上 `nano-banana-pro` 被配置为 `chat` 后，图生图固定落到 `/chat/completions`，而文生图仍走 `/images/generations`，配置语义与实际行为不一致。

## 目标

- 后台将图片模型路由拆分为“文生图路由”和“图生图路由”。
- 前端文生图与图生图分别读取各自配置，不再共用同一字段。
- 兼容现有线上 `model_routes` 老数据，避免手工改库后才能恢复使用。

## 方案

### 1. 路由配置拆分

`model_routes` 中新增两类图片键：

- `image_generate:<model>`
- `image_edit:<model>`

保留旧键：

- `image:<model>`

兼容规则：

- 读取文生图路由时，优先 `image_generate:<model>`，否则回退到 `auto`。
- 读取图生图路由时，优先 `image_edit:<model>`，其次回退 `image:<model>`，最后 `auto`。

这样可保证线上已有 `image:nano-banana-pro = chat` 会继续作用于图生图，不影响文生图。

### 2. 前端调用行为

- `requestGeneration()` 按“文生图路由”决定请求：
  - `generations` 或 `auto`：走 `/images/generations`
  - `chat`：走 `/chat/completions`
- `requestEdit()` 按“图生图路由”决定请求：
  - `edits` 或 `auto`：走 `/images/edits`
  - `chat`：走 `/chat/completions`
  - `generations`：明确报错，提示该路由不支持参考图编辑

### 3. 后台配置页

管理员高级配置中的“图片接口”拆成两项：

- 文生图接口
- 图生图接口

保存时分别写入：

- `image_generate:<model>`
- `image_edit:<model>`

页面初始值兼容旧数据：

- 文生图默认取 `image_generate:<model>`，没有则 `auto`
- 图生图默认取 `image_edit:<model>`，没有则回退旧的 `image:<model>`

## 验证点

- `nano-banana-pro` 可配置为：
  - 文生图：`/v1/images/generations`
  - 图生图：`/v1/images/edits`
- 旧库里仅有 `image:nano-banana-pro = chat` 时：
  - 文生图仍走 `/images/generations`
  - 图生图继续走 `/chat/completions`
- 管理后台保存新配置后，前端刷新可立即读取新字段。
