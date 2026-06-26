<p align="center">
  <img src="web/public/logo.svg" width="96" alt="logo">
</p>

<h1 align="center">无限画布</h1>

无限画布是一套面向商用部署改造中的 AI 创作工作台，包含无限画布、图片/视频/音频生成、账号体系、积分计费、管理后台与生成记录沉淀能力。

## 当前版本特性

- 用户注册、登录、个人中心、密码修改
- 管理后台统一配置上游 API、模型目录与模型计费
- 前台用户统一走后端代理，不再自行配置 API Key
- 只有已配置计费的模型才可在前台使用
- 画布、生成记录、积分流水按账号隔离
- 用户可查看自己的积分明细、充值记录

## 快速部署

```bash
git clone https://github.com/GeQainZz/infinite-canvas.git
cd infinite-canvas
chmod +x scripts/init-env.sh
./scripts/init-env.sh

docker compose up -d --build
```

默认端口：

- 前端：`3001`
- 后端：`18080`

详细说明见：

- `docs/content/docs/overview/docker.mdx`
- `docs/content/docs/backend/local-development.mdx`
- `docs/content/docs/backend/backend-database.mdx`
- `docs/content/docs/progress/pending-test.mdx`

## 说明

- 当前仓库已偏向单租户商用部署，不再以浏览器本地直连 AI 为主。
- 生产环境请务必自行设置 `.env` 中的数据库密码、JWT 密钥和 API Key 加密密钥。
- 不建议将 MySQL 暴露到公网。
- 当数据库为空且 `.env` 已配置 `INIT_ADMIN_*` 时，后端首次启动会自动创建初始 `super_admin`。
- 推荐正式环境使用同域名反向代理，例如前端 `https://hmgai.cc/`，后端 `https://hmgai.cc/api`。
- 如果你使用 Caddy，可直接参考 `deploy/Caddyfile.example`。
- 前端会在构建阶段读取 `NEXT_PUBLIC_API_URL`，如果你修改了这个值，需要重新构建 `app` 容器。

## 更新命令

```bash
git pull
docker compose up -d --build
```

常见更新场景：

- 只更新后端：`docker compose up -d --build backend`
- 只更新前端：`docker compose up -d --build app`
- 修改了 `NEXT_PUBLIC_API_URL`：必须执行 `docker compose up -d --build app`
