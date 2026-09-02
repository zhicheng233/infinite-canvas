# 项目稳定化治理测试需求

## 1. 测试目标

验证本轮稳定化治理没有破坏现有功能，并重点确认以下系统边界可靠：

- 后端、前端、测试代码和生产构建具有可信质量门禁。
- 图片、视频协议由模型配置决定，管理后台测试与真实生成行为一致。
- 生成请求先原子扣费，失败后幂等退款，并始终绑定创建任务时的物理渠道。
- 模型目录以渠道模型配置为唯一事实来源，不依赖旧 `TenantApiConfig`。
- 画布文档迁移、云端修订冲突及账号切换不会造成数据覆盖或串号。
- 旧画布、画布导出 v3、模型 API 配置包 v1 和既有供应商协议仍可使用。
- 空库和已有旧表都能执行 Goose 数据库迁移。

## 2. 执行规则

1. 测试 Agent 默认只执行测试和记录结果，不修改业务代码、配置文件或测试断言。
2. 不清理、不回滚当前工作区改动，不执行 `git reset`、`git checkout --` 等命令。
3. 每个失败必须保留：命令或操作步骤、预期结果、实际结果、完整错误摘要、相关文件或接口、是否稳定复现。
4. 涉及真实渠道时使用测试账号和低成本请求，不在报告中暴露 API Key、JWT、完整上游响应或用户隐私。
5. 先执行自动化门禁，再进行数据库集成和浏览器测试；基础门禁失败时仍应继续收集其他独立结果。

## 3. 环境准备

- Windows PowerShell 或 Linux shell。
- Go 1.23.x。
- Bun 1.3.13。
- 可选 MySQL 8 测试实例。测试账号需要创建、删除临时数据库的权限。
- 浏览器测试需要可登录的普通用户、SuperAdmin 和至少一个可用图片/视频测试渠道。
- 不得将生产数据库 DSN 设置为 `CREDIT_TEST_DSN`。

记录以下环境信息：

```text
OS:
Git commit / working tree state:
Go version:
Bun version:
Node version:
MySQL version:
Browser version:
Backend URL:
Frontend URL:
```

## 4. 自动化质量门禁

在仓库根目录分别执行，禁止用前一个命令的成功掩盖后一个命令的失败。

### 后端

```bash
cd backend
go test ./...
go vet ./...
```

验收标准：全部退出码为 0，不存在编译错误、Vet 错误或失败测试。

### 前端

```bash
cd web
bun install --frozen-lockfile
bun test
bun run typecheck
bun run typecheck:test
bun run build
```

验收标准：

- Bun 测试全部通过，当前基线应不少于 318 项。
- 应用 TypeScript 和测试 TypeScript 均无错误。
- Next.js 生产构建成功，不允许依靠 `ignoreBuildErrors` 跳过错误。

### 文档

```bash
cd docs
bun install --frozen-lockfile
bun run build
```

验收标准：MDX、Shiki 代码块和静态页面全部构建成功。

### Git 差异

```bash
git diff --check
git status --short
```

验收标准：无空白错误；`.next`、`node_modules`、构建目录和测试临时数据库文件不得进入 Git 状态。

## 5. MySQL 集成测试

设置专用测试 DSN 后重新运行后端测试：

```bash
cd backend
CREDIT_TEST_DSN='user:password@tcp(127.0.0.1:3306)/mysql?charset=utf8mb4&parseTime=True&loc=Local' go test ./... -count=1
```

PowerShell 可使用：

```powershell
$env:CREDIT_TEST_DSN = 'user:password@tcp(127.0.0.1:3306)/mysql?charset=utf8mb4&parseTime=True&loc=Local'
go test ./... -count=1
```

必须验证：

1. 迁移可从旧 `canvas_projects` 全局 `project_id` 唯一索引升级。
2. 迁移新增 `schema_version`、`revision` 和 `generation_jobs`。
3. 迁移改为 `tenant_id + user_id + project_id` 复合唯一约束。
4. 同一迁移重复执行不会报错或重复修改。
5. 并发扣费时余额不为负，成功预留数量与初始余额一致。
6. 同一生成请求或异步任务重复退款只产生一笔退款流水。
7. 画布旧 revision 保存返回冲突，并携带数据库中最新项目。

## 6. 后端协议与目录测试

### 图片协议矩阵

对同一个支持参考图的测试模型分别配置并验证：

| 配置 | 参考图 | 预期请求 |
|---|---:|---|
| `image_edit_route=generations` | 1 张 | `POST /v1/images/generations`，JSON `image` 为字符串 |
| `image_edit_route=generations` | 多张 | 同路径，JSON `image` 为字符串数组 |
| `image_edit_route=edits` | 1 张或多张 | `POST /v1/images/edits`，multipart |
| `image_edit_route=auto` | 有参考图 | 保持既有 edits 兼容行为 |
| `image_edit_route=chat` | 有参考图 | `/v1/chat/completions` JSON |
| `image_edit_route=banana` | 有参考图 | `/v1/chat/completions` 且包含 Banana 配置 |

同一用例分别通过管理后台“测试模型”和普通用户真实生成触发。验收标准是 method、path、Content-Type 和图片载荷结构一致。

特别验证 `gpt-image-2`：图片节点连接另一个图片节点后应成功发起图生图；失败时界面显示第一条真实错误，而不是只有“全部图片生成失败”。

### 模型目录与选择

1. 删除或不创建 `TenantApiConfig`，仅配置渠道、渠道模型和定价。
2. 普通用户仍能读取图片、视频、文本和音频模型目录。
3. v0.5 旧 `POST /api-config` 返回成功和弃用提示，并原子双写旧记录与规范化模型目录；任一步失败时两边都不留部分数据。
4. 物理模型按 `channel_model_id` 选路。
5. 智能路由按管理员确认的 `routing_pool_id` 选路，并返回实际物理渠道 ID；不再按同名模型隐式聚合。
6. 合并组按 `channel_id + group_name` 选路。
7. `/chat/completions` 图片模型携带 `routing_capability=image` 时按图片能力路由和计费。
8. 未知图片或视频路由在保存、导入和模型测试前被拒绝。

### API 配置导入

1. 空站点导入多个不同名称但相同 Base URL 的渠道，全部应显示新增。
2. 再次导入同一文件，渠道应显示更新，不新增重复记录。
3. 同 URL 不同名称允许创建。
4. 同名不同 URL、重复 `channel_ref`、重复渠道名称和完全重复渠道仍应冲突。
5. 共享 URL 渠道的模型、渠道定价和合并组必须分别关联正确渠道。
6. API 配置包加密 envelope 保持 `version: 1`，新导出 payload 使用 `schema_version: 2`，并继续接受 payload v1；错误密码或篡改文件必须拒绝。

## 7. 计费生命周期测试

对同步图片生成和异步视频生成分别验证：

1. 余额不足时不调用上游，不创建成功任务，不产生扣款流水。
2. 请求上游前已经完成积分预留，并返回 `X-Generation-Request-ID`。
3. 上游网络错误或 HTTP 4xx/5xx 后自动退款，接口报告扣费为 0。
4. 上游同步成功后任务状态为 `succeeded`。
5. 异步创建成功后任务状态为 `pending`，保存上游任务 ID。
6. 异步成功轮询将原任务标记为 `succeeded`。
7. 异步失败轮询按原请求退款；重复轮询不重复退款。
8. Auto 或合并组创建任务后，更改成功率或禁用其他渠道，轮询、下载和退款仍使用创建时物理渠道。
9. 余额、`total_spent`、支出流水、退款流水和 `generation_jobs` 最终一致。

## 8. 认证与大小限制测试

1. 正常用户 JWT 可访问受保护接口。
2. 签发 JWT 后禁用用户，旧 JWT 立即失效。
3. 签发 JWT 后删除用户，旧 JWT 立即失效。
4. 普通用户不能访问 SuperAdmin 接口。
5. 生成、`/proxy` 和临时上传超出配置限制时返回 413 或明确错误。
6. 超大上游响应被中止，不得无限读取内存。
7. 临时媒体文件名包含 `../`、绝对路径或编码后的路径穿越时必须拒绝。

## 9. 画布兼容与同步测试

### 文档迁移

1. 加载无 `schemaVersion` 画布，按版本 1 升级到当前版本 2。
2. 加载显式版本 1 画布，`targetImageRole: "images"` 规范化为无角色旧连线并去重。
3. 风格图、元素图等显式角色保持不变。
4. 未来版本文档不得被当前客户端静默降级保存。
5. 导出 ZIP 仍为 envelope `version: 3`，导入后节点、连线和素材可用。

### 云端 revision

1. 客户端 A、B 同时打开同一画布。
2. A 保存成功，revision 递增。
3. B 使用旧 revision 保存，服务端返回 HTTP 409 和最新云端项目。
4. B 的本地内容保存为“本地冲突副本”，A 的云端内容不被覆盖。
5. 冲突副本可以继续保存为独立项目。

### 账号切换

1. A 用户修改画布后，在 2 秒防抖保存触发前快速退出并登录 B。
2. A 的旧定时器和请求必须取消或保持在 A 作用域。
3. B 账号中不得出现 A 的项目或保存结果。
4. 再登录 A 后，本地缓存和云端项目仍属于 A。

## 10. 画布模型与视频设置浏览器验收

1. 自定义视频模型启用 `images` 时，入口显示“图片参考”，新连线不保存 `targetImageRole`。
2. 仅启用 `input_reference` 时，旧图片连线自动显示并映射为“首帧参考图”。
3. 未启用 `images` 且启用多个图片角色时，旧连线不可用并提示重新连接。
4. “自定义首帧模型 → 标准模型 → 自定义首帧模型”切换后，入口依次显示“首帧参考图 → 图片参考 → 首帧参考图”。
5. 显式风格图或元素图连接在不支持模型下暂时失效，切回原模型后恢复。
6. 自定义尺寸选择 `1280x720`、`720x1280` 后概览立即显示对应尺寸。
7. 宽高比选择 `16:9`、`9:16` 后概览立即更新。
8. 自定义时长变化后概览显示当前运行时秒数，不显示旧 `6s`。
9. 自定义配置未启用尺寸或秒数时，不伪造 `720p · 方形 · 6s`。
10. 切换标准和自定义模型时，标签、锚点、容量、概览和最终请求参数同步变化，连线数据不被模型切换改写。

## 11. 结果报告格式

测试结束后提交以下报告：

```markdown
# 稳定化治理测试报告

## 结论
- 结果：通过 / 有条件通过 / 不通过
- 阻塞问题数量：
- 非阻塞问题数量：

## 环境
- OS：
- Commit / 工作区：
- Go / Bun / MySQL / Browser：

## 自动化结果
| 命令 | 结果 | 耗时 | 备注 |
|---|---|---:|---|

## 场景结果
| 编号 | 场景 | 结果 | 证据 |
|---|---|---|---|

## 失败详情
### [P0/P1/P2/P3] 标题
- 复现步骤：
- 预期：
- 实际：
- 错误日志：
- 相关接口/文件：
- 是否稳定复现：
- 是否可能造成数据丢失、错误扣费或跨账号污染：

## 未执行项目
- 项目：
- 原因：
- 所需环境或权限：
```

## 12. 最终验收标准

- 所有基础质量门禁通过。
- P0/P1 问题为 0。
- 不发生错误扣费、重复退款、余额为负或异步任务跨渠道。
- 不发生画布静默覆盖、跨账号保存或旧文档不可恢复。
- 管理后台测试与真实生成的协议载荷一致。
- 空站点模型目录和 API 配置导入可用。
- 未执行的 MySQL、真实渠道或浏览器项目必须明确列出，不得用自动化测试通过替代人工验收结论。
