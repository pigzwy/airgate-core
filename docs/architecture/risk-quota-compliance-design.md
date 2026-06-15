# 风控 / 配额 / 合规门 迁移设计（sub2api → airgate）

## 状态

落地设计文档（基于双仓库真实代码）。覆盖 `sub2api-feature-migration.md` 中三项缺失功能的 airgate 实现方案：

1. **内容审核 moderation**（三节「风控/审核」★需要）
2. **用户×平台 USD 配额**（三节「用量限额」）
3. **管理员合规确认门**（三节「平台特性」）

每节给出：架构决策 → 四层文件清单 → 拦截/挂载点（带 airgate 真实文件:行号）→ 数据表 → 配置 → API → 前端 → 风险。配套需求规格见 `sub2api-feature-migration.md` 四/五/八节。

---

## 〇、关键架构结论（决定一切）

**与 Ops 不同，风控/配额不存在「数据在插件子进程里看不到」的难题。**

airgate Core 在转发给插件**之前**就已完整读取并解析请求体：

| 事实 | 依据 |
|------|------|
| Core 转发前 `io.ReadAll` 整个请求体 | `internal/plugin/request.go:36-45` |
| `parseBody` 解析出 model/stream/sessionID（可扩展提取 message 文本） | `internal/plugin/request.go:48-76` |
| 转发链有清晰的「预检 → 选号 → 调插件」阶段 | `internal/plugin/forwarder.go:81-337` |
| 余额预检 `checkBalance` 是现成的「转发前拒绝」范式 | `internal/plugin/forwarder.go:86`、`internal/plugin/quota.go:22` |

**推论**：
- **moderation 与 quota 都做成纯 Core**，拦截点紧挨 `checkBalance`，无需改任何 gateway 插件（与 Ops「必须插件上报」根本不同）。
- **compliance gate 更简单**：纯 HTTP 中间件 + 复用 settings 键值表，**连 ent schema 都不用动**。

**统一的转发前拦截顺序**（建议，自上而下）：
```
forwarder.Forward()
  parseRequest()         // 已读 body
  checkBalance()         // 现有：余额不足拒绝
  checkModeration()      // 【新增①】pre_block 命中 → 403 拒绝；observe → 异步记录后放行
  checkPlatformQuota()   // 【新增②】日/周/月 USD 超限 → 429 拒绝
  acquireClientQuota() … // 现有：并发闸门 → 选号 → 调插件
```
> moderation/quota 都放在「选号之前」，超限/命中可零成本拒绝，不占账号并发槽。

---

## 一、内容审核 moderation（Core 新增 `app/moderation`）

### 1.1 架构决策

- **纯 Core 域**，挂在 forwarder 预检阶段（`forwarder.go:86` checkBalance 之后）。
- 需新增一个**请求文本提取器**（对标 sub2api `content_moderation_input.go`）：从已解析的 body 里抽「最后一条 user message」文本，支持 Anthropic Messages / OpenAI Chat 两种格式（MVP 先这两种）。
- 检查 = 本地敏感词 + OpenAI Moderation API，二选一或串联。

### 1.2 四层文件清单（参照 `account` 全链路）

| 层 | 新增文件 | 职责 |
|---|---|---|
| Ent | `ent/schema/moderationlog.go` | 审核日志表（命中明细，对标 sub2api `content_moderation_logs`） |
| Service | `app/moderation/{service,types,errors}.go` | 审核编排：抽文本→敏感词→API→判定→落库→（异步）封禁/邮件 |
| Service | `app/moderation/input.go` | 请求体 → 待审文本提取 + 脱敏（URL/key/JWT → `[已脱敏]`） |
| Service | `app/moderation/openai.go` | OpenAI Moderation API 客户端（多 key 轮询 + 超时 + 重试） |
| Store | `infra/store/moderation_store.go` | `Repository` 的 ent 实现（落日志、按窗口计数违规） |
| 拦截 | `internal/plugin/moderation_gate.go`（或 forwarder 内方法） | `checkModeration(c, state)` 调 service，返回 allow/deny |
| DTO | `server/dto/moderation.go` | 配置/日志查询 DTO |
| Handler | `server/handler/moderation_handler{,_routes,_mapper}.go` | 管理员配置读写 + 日志查询 + 解禁 |

### 1.3 配置（复用 settings 表，`group="moderation"`）

settings 存取入口：`app/settings/service.go` + `infra/store/settings_store.go`（key/value/group，`ent/schema/setting.go`）。

一个 JSON 配置项 `moderation_config`（对标 sub2api `ContentModerationConfig`，MVP 精简）：
```jsonc
{
  "enabled": false,
  "mode": "off|observe|pre_block",        // 默认 off
  "keyword_mode": "keyword_only|keyword_and_api|api_only",
  "blocked_keywords": ["…"],               // 本地词库，子串不区分大小写
  "api_base_url": "https://api.openai.com",
  "api_model": "omni-moderation-latest",
  "api_keys": ["sk-…"],                    // 加密存储（见下）
  "timeout_ms": 3000, "retry_count": 2,
  "thresholds": { "harassment": 0.9, "sexual": 0.65, … },
  "block_status": 403, "block_message": "内容命中风险规则",
  "email_on_hit": true,
  "auto_ban_enabled": true, "ban_threshold": 10, "violation_window_hours": 720,
  "model_filter": { "type": "all|include|exclude", "models": [] },
  "hit_retention_days": 180, "non_hit_retention_days": 3
}
```
> ⚠️ `api_keys` 是上游凭证，**必须经 `internal/auth/crypto.go` 的 `EncryptAPIKey` 加密落库**，不明文、不写日志（项目红线）。

### 1.4 模式行为

| mode | 行为 | 返回 |
|---|---|---|
| off | 不审核 | 放行 |
| observe | 异步后台审核（worker 池），立即放行 | 200 放行，命中只记录/邮件 |
| pre_block | 同步审核，命中即拒 | `block_status`（默认 403）+ `content_policy_violation` |

### 1.5 违规封禁（需要 User 加字段——见风险）

- 计数：`moderation_store` 按 `user_id` 查 `violation_window_hours` 内 `flagged=true` 条数。
- 达 `ban_threshold` → 封禁该用户 + 失效其鉴权缓存 + 邮件通知（复用 settings 的 SMTP / 现有邮件能力）。
- 解禁：管理员端点 `POST /admin/moderation/unban/:user_id`。

### 1.6 管理 API（adminGroup）

```
GET    /admin/moderation/config              # 读配置
PUT    /admin/moderation/config              # 写配置
POST   /admin/moderation/test-keys           # 测 OpenAI key 可用性
GET    /admin/moderation/logs                # 审核日志分页（按 user/group/时间/flagged）
POST   /admin/moderation/unban/:user_id      # 解禁
```

### 1.7 前端（`web/`，零冲突——全新页面）

- `pages/admin/ModerationPage.tsx`：配置表单 + 审核日志表 + 解禁。
- `shared/api/moderation.ts` + queryKeys + types；AppShell 菜单入口。

### 1.8 风险 / 待决

- **User 无 `status/banned` 字段**：airgate `ent/schema/user.go` 只有 role(admin/user)，无封禁态。auto-ban 需**给 User 加 `banned bool` 或 `status enum`**（→ `make ent`），或建独立封禁表。**建议加 `User.banned`**，并在 API Key 鉴权处（`middleware/auth.go` 的 `ValidateAPIKey`）拒绝已封禁用户。
- **文本提取覆盖面**：MVP 只覆盖 Anthropic/OpenAI Chat；Gemini/Responses/Images 后排。
- **observe 异步队列**：MVP 可先用简单 goroutine + 带界 channel，别一上来上复杂 worker 池。

---

## 二、用户×平台 USD 配额（Core 新增 `app/quota` 或并入 `app/billing`）

### 2.1 架构决策

- **纯 Core**，执行点在 forwarder 预检（`checkModeration` 之后），紧挨现有 `checkBalance`。
- **不实时聚合 `usage_log`**（每请求扫表太重）。对标 sub2api：建**专表 `user_platform_quota`**，存「限额 + 当前窗口已用 + 窗口起点」，请求时只读一行做比较；用量在**计费落库时顺带累加**。
- 平台白名单单一权威源：新增 `app/quota` 内 `AllowedPlatforms`（对标 sub2api `domain_constants.go:46` `AllowedQuotaPlatforms`），ent schema 的 Validate 与之手工同步。

### 2.2 数据表 `ent/schema/userplatformquota.go`

对标 sub2api `user_platform_quota`：

| 字段 | 类型 | 说明 |
|---|---|---|
| user_id | int64 | 用户 |
| platform | string(≤32) | 白名单 anthropic/openai/gemini/antigravity |
| daily_limit_usd / weekly_/ monthly_ | *float (nillable) | **nil=不限、0=禁用、>0=上限** |
| daily_usage_usd / weekly_/ monthly_ | float default 0 | 当前窗口已用 |
| daily_window_start / weekly_/ monthly_ | *time | 窗口起点（nil=未初始化，代码兜底） |
| (mixin) created_at/updated_at | time | timeNow（参 `ent/schema/mixin.go`） |

唯一约束：`(user_id, platform)`（airgate 无 SoftDelete mixin，可不带 deleted_at 条件；如需软删再议）。

### 2.3 执行点与用量累计

- **检查**：`internal/plugin/quota_platform.go` 加 `checkPlatformQuota(c, state)`：
  - 取 `state` 的 user_id + 账号 platform → 读该行 → 窗口过期则就地清零并更新 window_start → 任一窗口 `limit!=nil && usage>=limit` 则拒。
  - 超限返回 **429** + 错误码 `USER_PLATFORM_{DAILY|WEEKLY|MONTHLY}_QUOTA_EXHAUSTED` + `window_resets_at`。
- **累计**：在计费写库处 `internal/billing/recorder.go`（`applyUsageCharges` 一带，:518-572）按 `user_id+platform` 给三个 usage 字段加 `actual_cost`。**与余额扣减同事务**，保证一致。
- **缓存**：MVP 直接读写 DB 一行（开销可接受）；后续再按 sub2api 加 Redis entry + singleflight（不阻塞首期）。

### 2.4 管理 / 用户 API

```
GET  /admin/users/:id/platform-quotas          # 查配额+已用
PUT  /admin/users/:id/platform-quotas          # 全量替换（[]{platform,daily,weekly,monthly}）
POST /admin/users/:id/platform-quotas/reset    # 重置某窗口 {platform, window}
GET  /me/platform-quotas                        # 用户侧查自己配额与已用（userGroup）
```

### 2.5 前端（零冲突）

- admin 用户详情加「平台配额」编辑弹窗（对标 `UserPlatformQuotaModal.vue`）。
- 用户面板加配额单元 `usage/limit`（对标 `UserPlatformQuotaCell.vue`）。

### 2.6 风险 / 待决

- **`usage_log` 是否带 platform**：累计需 user_id+platform 维度。account 有 platform，usage_log 关联 account_id——确认 recorder 能拿到 platform（应可由 account 快照得到）。
- **窗口语义**：日/周/月按自然窗口还是滚动窗口？sub2api 用窗口起点+过期重置（自然窗口）。沿用。
- **订阅豁免**：sub2api 仅 standard（余额）模式查配额，订阅模式豁免。airgate 若有订阅模式需对齐。

---

## 三、管理员合规确认门（Core 新增中间件，最简单）

> **✅ 已实现（2026-06-15，前后端完整）**：
> - **后端**：`app/compliance/{types,errors,service,service_test}.go` + `legal/admin-compliance.{zh,en}.md`（go:embed）、`server/middleware/admin_compliance.go`、`server/dto/compliance.go`、`server/handler/compliance_handler.go`、`bootstrap/http_handlers.go` + `server/router.go` 接线。端点：`GET /admin/compliance`、`POST /admin/compliance/accept`、`PUT /admin/compliance/enable`。单元测试 5/5 通过。
> - **前端**：`shared/api/compliance.ts`、`shared/components/AdminComplianceGate.tsx`（client.ts 全局 423 钩子 → 弹框，react-markdown 渲染文档 + 逐字短语，挂在 AppShell）、`pages/admin/ComplianceToggleCard.tsx`（设置→安全 开关）。type-check + eslint 通过。
> - **默认关闭**（`compliance_gate_enabled` 未开启时中间件 no-op），管理员在「设置→安全」开启后才生效——开启后该管理员下次请求即触发 423 弹框确认。
> 注：`go build ./...` 全量编译当前被监控窗口在建的 `app/ops` 包阻断（与本功能无关）；合规门相关包独立编译 + 测试均通过。

### 3.1 架构决策

- **纯 Core，最易落地**：HTTP 中间件 + **复用 settings 键值表存确认状态**，**无新 ent schema、无 make ent**。
- per-admin 版本化：确认记录按 admin user_id 存；版本号变更则需重新确认。

### 3.2 文件清单

| 层 | 新增/改动 | 职责 |
|---|---|---|
| 中间件 | `server/middleware/admin_compliance.go` | 拦截 adminGroup 非 bypass 路由，未确认返 **423** |
| Service | `app/compliance/{service,types,errors}.go` | 状态查询/确认/短语校验/版本判定 |
| Handler | `server/handler/compliance_handler.go` | `GET /admin/compliance`、`POST /admin/compliance/accept` |
| 文档 | `docs/legal/admin-compliance.zh.md` / `.en.md` | 合规声明正文（对标 sub2api） |
| 前端 | `components/admin/AdminComplianceDialog.tsx` | 拦截到 423 弹确认框，逐字短语 + 语言 |

### 3.3 存储（settings 表，key=`admin_compliance_ack:{user_id}`）

value = JSON：`{version, admin_user_id, ip, user_agent, accepted_at, language}`。
版本常量 `const ComplianceVersion = "v2026.06.10"`（与文档同步，升级即触发全员重确认）。

### 3.4 中间件逻辑

- bypass：`/admin/compliance*`（取状态/提交确认本身不能被拦）。
- 取 ctx 中 admin user_id（JWTAuth 已注入）→ service 查是否已确认当前版本 → 否则 **423 Locked** + body：
  ```json
  { "code": "ADMIN_COMPLIANCE_ACK_REQUIRED",
    "metadata": { "version": "v2026.06.10", "document_url_zh": "…", "document_url_en": "…" } }
  ```
- 挂载点：`server/router.go` adminGroup 在 `JWTAuth` + `AdminOnly` 之后追加 `middleware.AdminComplianceGate(complianceSvc)`（router.go:101-102 一带）。

### 3.5 确认逻辑

- `POST /admin/compliance/accept {phrase, language}`：phrase 须**精确匹配**该语言的确认短语，否则 400；匹配则记录（含 IP/UA/时间）。
- ⚠️ 用 **423**（Locked）不要用 401——airgate 前端对 401 有全局登出拦截（项目红线）。

### 3.6 风险

- 几乎无。唯一注意：中间件须在 JWTAuth 之后（要 user_id）、且 bypass 自身端点，否则死锁。

---

## 四、落地顺序 & 与监控窗口的冲突边界

### 4.1 共享文件冲突点（必须与监控窗口协调）

scaffold 这三个功能会碰到监控窗口可能也在改的**共享文件/操作**：

| 共享点 | 谁会碰 | 冲突规避 |
|---|---|---|
| `make ent`（全量重生成 `ent/`） | moderation、quota 加 schema；监控可能加 alert/syslog schema | **串行做 ent 变更**：一个窗口 schema 改完 `make ent` 提交，另一个再开始。切勿并行 |
| `bootstrap/http_handlers.go` | 两边都要装配新 handler | 各加各的字段，行不同，手动合并即可（低风险） |
| `server/router.go` `registerRoutes()` | 两边都注册路由 | 同上，不同分组/不同行（低风险） |

**compliance gate 不碰 `make ent`**（用 settings 表），是三者中冲突最小的——可最先落。

### 4.2 推荐顺序（按「易 + 价值 + 低冲突」）

1. **合规门**（最易、零 ent、价值高/合规刚需）——先落，验证「纯 Core + settings + 中间件」链路。
2. **moderation**（★需要，纯 Core，但要加 `User.banned` + moderation 表 → 一次 `make ent`）。
3. **配额**（纯 Core，加 `user_platform_quota` 表 → 与 moderation 合并到同一次 `make ent` 批次，减少重生成次数）。

> 建议把 moderation + quota 的 schema **凑一批 `make ent`**，与监控窗口约定一个「ent 变更窗口」串行执行。

### 4.3 代码脚手架的启动方式（二选一）

- **A. 等监控窗口收手**：它停止改 `http_handlers.go`/`router.go`/ent 后，我按本文顺序落代码。
- **B. 用 git worktree 隔离**：我在独立 worktree 写新域代码（service/store/dto/handler/前端等**全新文件**先就绪），ent 变更与共享文件接线留到最后合并时统一做。

> 推荐 **A**（合并简单）；若想并行赶进度选 **B**。

---

> 每个 Core 新域参照 `account` 全链路；schema 改动 `make ent` 并提交生成代码（红线）；service 不碰 gin/不 import ent；handler 不写业务。
