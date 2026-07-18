# VEO JSON 视频路由设计

## 目标
为需要 `POST /v1/videos` + `application/json` 协议的视频模型新增独立路由，不影响现有 multipart `/v1/videos`、`/v1/videos/generations`、`/v1/video/generations` 与 Seedance 分支。

## 方案
- 后台视频路由新增 `veo_json` 选项，展示为 `/v1/videos（JSON / veo）`。
- 当前端模型路由显式选择 `veo_json` 时：
  - 请求路径使用 `/videos`
  - 请求头使用 `application/json`
  - 请求体字段使用 `model`、`prompt`、`duration`、`aspect_ratio`、`Ingredients_images`
- `Ingredients_images` 严格按文档原样发送，不做字段名兼容。
- 轮询先复用现有 `/videos/{id}` 查询逻辑。

## 字段映射
- `duration`：沿用当前视频时长配置与固定时长约束。
- `aspect_ratio`：优先输出当前尺寸对应的比例值，例如 `16:9`。
- `Ingredients_images`：参考图片 URL 列表；保留当前参考图读取逻辑。

## 影响范围
- `web/src/app/(user)/admin/api-config/page.tsx`
- `web/src/services/api/video.ts`

## 风险控制
- 仅显式选择 `veo_json` 的模型使用新协议。
- 不改现有自动判断与其他路由分支。
