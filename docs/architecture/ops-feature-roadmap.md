# Ops 运维监控补齐路线（对齐 sub2api）

## 状态

✅ **全部 16 个模块（M1–M16）已实现**（2026-06）。OpsPage 已覆盖 sub2api 运维能力的主体。

配套背景见 `sub2api-feature-migration.md` 第八节（精确差距清单）。本文是**可执行的任务清单**，每个模块独立交付、做完打勾。

### 实现要点与偏差说明

- **M5 用 SSE 而非 WebSocket**：零新增依赖、走现有 header JWT 鉴权、反代友好。端点 `GET /ops/stream`，前端 `useOpsStream` 用 fetch 流式读取（非原生 EventSource，以便带 Authorization 头），2s 推一帧，断线自动重连。
- **M7 并发四维做了三维**（账号/平台/分组 + 账号可用性汇总）；用户维度需 scheduler 暴露用户槽位批量计数接口，暂缺。
- **M12 分段延迟未做**：airgate-sdk 的 `ForwardOutcome` 未暴露分阶段耗时，做不了 auth/routing/upstream/response 拆分（需改 SDK，越界）。已实现的部分：模型映射（requested→upstream）、`client_request_id` trace 关联（跨账号重试链路）、错误体钻取。
- **保留策略/邮件日报/阈值染色**（M15）经 settings(group=ops) 实时生效；阈值用于前端卡片染色，保留期与日报由后台循环读取。

## 推进原则

1. **小步交付**：一个模块一个 PR/commit，做完能独立验证，不憋大招。
2. **先 Core 自采、后插件配合**：能 Core 自己搞的（系统资源、并发、告警）先做，卡在插件上报字段的（分段延迟、trace）后排。
3. **纵切交付**：每个模块都是「后端能力 + API + 前端页」完整一条，不横着铺半层。
4. **复用 airgate 四层**：每个新 domain 参照 `account` 全链路；schema 改了要 `make ent`。

## 关键认知：采集能力边界

sub2api 是单体，转发在进程内，所有请求数据天然可得。
airgate 转发在**插件子进程**，Core 只拿得到插件主动 `Host.Invoke("ops.report_request")` 上报的字段。

所以补齐前要分清每个功能的**数据来源**：

| 数据来源 | 说明 | 典型功能 |
|---------|------|---------|
| **Core 自采** | Core 进程内直接采集，零插件依赖 | 系统资源(CPU/内存/DB/Redis/Goroutines)、并发统计(scheduler 已有)、账号可用性 |
| **插件上报已有** | 当前 ops.report_request 已带上报 | 延迟/状态码/错误/token/platform/model（MVP 已通） |
| **需扩插件上报** | 当前上报字段不够，要改插件 schema | 分段延迟、模型映射 requested→upstream、trace 链路、入站/上游端点 |
| **Core 聚合** | 基于已上报数据二次计算 | 延迟直方图 bucket、错误分布饼图、切换率 |

> 规则：能走「Core 自采」或「Core 聚合」的先做（不卡插件），「需扩插件上报」的单独排期。

---

## 任务清单（按优先级，逐个打勾推进）

### P0 — 架构性 / 最易感知"功能少"

- [x] **M1 系统资源监控（6 卡）** Goroutines/堆内存/GC/DB 连接池/Redis/后台任务 + 任务明细
- [x] **M2 时间范围切换 + 平台维度下钻** URL 持久化（validateSearch）
- [x] **M3 告警闭环（规则+事件+静音+邮件）** 3 张表 + 评估器（cron+Redis 锁）+ 自动恢复
- [x] **M4 健康分数 + 诊断报告** 业务/基础设施加权 + 诊断建议
- [x] **M5 实时推送** SSE（见上「实现要点」）
- [x] **M6 请求详情钻取** success 模式 + 排序 + 慢请求

### P1 — 运维必备

- [x] **M7 并发统计卡** 账号/平台/分组三维 + 账号可用性（用户维度待 scheduler API）
- [x] **M8 TTFT + 延迟全分位（P50/P90/P95/P99/Max）** 实时从原始日志计算
- [x] **M9 延迟直方图（bucket 分布）**
- [x] **M10 错误分布饼图（上游5xx/客户端4xx/网关5xx/其他）**
- [x] **M11 系统日志表 + 运行时日志级别调整** slog sink（异步+背压）
- [x] **M12 错误日志增强 + trace** 模型映射 + client_request_id 链路（分段延迟待 SDK）

### P2 — 增强

- [x] **M13 切换率趋势** sticky 重路由计数 → 分钟桶
- [x] **M14 OpenAI Token 统计（按 model）**
- [x] **M15 设置中心（阈值染色/保留策略/邮件报告）** 经 settings(group=ops)
- [x] **M16 URL 深链 / 全屏大屏 / 自动刷新可调**

---

## 各模块详情（每节含：目标 / 采集方案 / 后端 / 前端 / 依赖 / 工作量）

> 工作量是粗估（人天），供排期参考。S=0.5天内，M=1-2天，L=3-5天。

### M1 系统资源监控（6 卡）⭐ P0 / 推荐先做

**目标**：大盘顶部展示 CPU / 内存 / 数据库连接 / Redis连接 / Goroutines / 后台Jobs 六张卡，含阈值染色（warn/crit）。

**采集方案**：**Core 自采，零插件依赖**。
- CPU/内存/Goroutines：用 `github.com/shirou/gopsutil`（sub2api 同款）按需采样
- DB 连接：`ent.Client` 底层 `*sql.DB.Stats()`（active/idle/waiting）
- Redis 连接：`redis.Client.PoolStats()`
- 后台 Jobs：Core 各后台循环（聚合器/清理/recorder/配额刷新等）上报心跳到一个注册表

**后端**：
- [ ] `app/ops/` 新增 `SystemMetrics()` 方法，聚合上述指标
- [ ] 新增 `app/ops/` Job 心跳注册表（各后台 loop 启动时注册 + 定期 heartbeat）
- [ ] 新增 API：`GET /api/v1/admin/ops/system-metrics`

**前端**：
- [ ] OpsPage 顶部加 6 张系统资源卡（Card + 阈值染色 + 进度条）

**依赖**：无（纯 Core）。需引入 gopsutil 依赖。
**工作量**：M（1-2天）

---

### M2 时间范围切换 + 维度下钻 ⭐ P0

**目标**：顶部加时间范围选择器（5m/30m/1h/6h/24h/自定义）+ 平台/分组筛选，所有图和卡跟着切换。

**采集方案**：**Core 聚合**，当前 ops_window_stat 已有数据，扩展查询参数即可。

**后端**：
- [ ] `Overview` / 趋势查询支持 time_range / platform / group_id 参数
- [ ] 按时间范围聚合（当前固定查最近窗口，改成查 [now-range, now]）

**前端**：
- [ ] 顶部工具栏：时间范围 Select + 平台 Select + 分组 Select
- [ ] 指标卡、趋势图、错误表全部联动查询参数
- [ ] URL query 持久化（刷新不丢筛选）

**依赖**：无。
**工作量**：M（1-2天）

---

### M3 告警闭环 ⭐ P0

**目标**：告警规则（13种指标）→ 定时评估 → 事件流 → 邮件通知 → 静音/手动解决。

**采集方案**：**Core 自有定时评估**。规则评估读 ops_window_stat + M1 的系统指标，不依赖插件。

**后端**：
- [ ] schema：`opsalertrule`（指标/运算符/阈值/窗口/持续/严重度/冷却/邮件/filters）+ `opsalertevent` + `opsalertsilence`
- [ ] `app/ops/alert`：规则 CRUD + 评估器（1min cron + 分布式锁）+ 邮件通知（复用 settings SMTP）+ 静音
- [ ] API：规则 CRUD / 事件列表（游标分页）/ 静音 CRUD / 手动解决

**前端**：
- [ ] 告警规则卡（列表+新建表单+启停）
- [ ] 告警事件流卡（状态徽章+severity+筛选+手动解决）
- [ ] 静音管理

**依赖**：M1（系统资源指标作为告警源）。邮件复用现有 SMTP。
**工作量**：L（3-5天）

---

### M4 健康分数 + 诊断报告

**目标**：0-100 健康分环 + Hover 诊断报告（自动列 critical/warning 项 + 处置建议）。

**采集方案**：基于 M1 系统资源 + 现有错误率/TTFT 加权计算。

**后端**：
- [ ] `app/ops/healthscore.go`：0-100 计算（业务层 70% = 错误率+TTFT；基础设施层 30% = CPU/内存/DB/Redis）
- [ ] 诊断规则引擎（各项阈值 + 影响描述 + 处置建议）
- [ ] API：`GET /api/v1/admin/ops/health` 返回分数 + 诊断项

**前端**：
- [ ] 健康分环组件（三色 + 超大数字）
- [ ] Hover 诊断弹层（critical/warning/info 分组列表）

**依赖**：M1（系统资源）+ M8（TTFT）。错误率 MVP 已有。
**工作量**：M（1-2天）

---

### M5 WebSocket 实时推送

**目标**：把大盘 15s 轮询升级为秒级 WebSocket 推送（QPS/TPS 快照）。

**采集方案**：Core 推送聚合后的 window stat，不依赖插件。

**后端**：
- [ ] 引入 WebSocket（gin + gorilla/websocket 或 nhooyr）
- [ ] 端点 `GET /api/v1/admin/ops/ws/qps`，token 走 query/子协议
- [ ] 聚合器每周期广播最新快照给订阅者
- [ ] 连接管理（心跳/断线重连/超时）

**前端**：
- [ ] 替换 Overview 的轮询为 WS 订阅（保留轮询作降级）
- [ ] 连接状态指示 + 断线重连

**依赖**：无（但建议 M2 之后做，推送的是维度数据）。
**风险**：airgate 当前无 WS 基础设施，是架构新增。Caddy/nginx 反代要加 WS 支持。
**工作量**：L（3-5天，含基础设施）

---

### M6 请求详情钻取（成功/慢请求）

**目标**：不只看错误，能按 success/error/all + 排序(时间/耗时) 查所有请求，慢请求 Top。

**采集方案**：当前 ops_request_log 已存所有请求（success 字段区分），**数据已有**，只缺查询。

**后端**：
- [ ] `RequestLogFilter` 扩展：success 模式(all/success/error) + 排序字段 + 耗时区间
- [ ] API：`GET /api/v1/admin/ops/request-logs`（复用 error-logs，加参数）

**前端**：
- [ ] 错误日志区改成「请求明细」，加 success 模式切换 + 排序 + 耗时筛选
- [ ] 卡片"详情"按钮带 preset 跳转

**依赖**：无（数据已在）。
**工作量**：S-M（0.5-1.5天）

---

### M7 并发统计卡（四维）

**目标**：按 平台/分组/账号/用户 展示并发使用率 + 账号可用性（限流/过载倒计时）。

**采集方案**：**Core 自采**。`scheduler.ConcurrencyManager` 已有三级并发数据；账号状态机已有可用性。

**后端**：
- [ ] `app/ops/` 新增并发聚合（按 platform/group/account/user 维度 + current/max + 等待队列）
- [ ] 账号可用性统计（active/rate_limited/degraded/disabled 计数 + 倒计时）
- [ ] API：`GET /api/v1/admin/ops/concurrency`

**前端**：
- [ ] 并发卡（四维 Tab 切换 + 进度条 + 状态徽章）

**依赖**：无（scheduler 数据现成）。
**工作量**：M（1-2天）

---

### M8 TTFT + 延迟全分位

**目标**：P50/P90/P95/P99/Max 五件套 + TTFT 首 token 延迟全套。

**采集方案**：**Core 聚合**。first_token_ms 插件已上报（ops_report.go 有）；聚合器当前只算 p50/p95/p99，扩 p90/max。

**后端**：
- [ ] 聚合器加 p90/max + TTFT 分位
- [ ] ops_window_stat 加字段 或 新表
- [ ] API 扩展返回

**前端**：
- [ ] 耗时卡改 5 件套；新增 TTFT 卡

**依赖**：无。
**工作量**：S（0.5-1天）

---

### M9 延迟直方图（bucket 分布）

**目标**：延迟按 bucket（0-50/50-100/100-500/500+ms）展示分布柱状图。

**采集方案**：**Core 聚合**。聚合时按 duration_ms 分桶计数。

**后端**：
- [ ] 聚合器输出 bucket 分布
- [ ] API：`GET /api/v1/admin/ops/latency-histogram`

**前端**：
- [ ] 延迟直方图（Bar 图）

**依赖**：无。
**工作量**：S（0.5-1天）

---

### M10 错误分布饼图

**目标**：按错误码/错误类型分类的环形图（上游5xx/客户端4xx/系统500/其他）。

**采集方案**：**Core 聚合**。按 status_code/error_kind 分组计数。

**后端**：
- [ ] 错误分布聚合
- [ ] API：`GET /api/v1/admin/ops/error-distribution`

**前端**：
- [ ] 错误分布 Doughnut 图 + 占比

**依赖**：无。
**工作量**：S（0.5-1天）

---

### M11 系统日志表 + 运行时级别调整

**目标**：网关自身日志查询（多维筛选）+ 不重启改日志级别。

**采集方案**：**Core 自采**。给 slog 加一个 Sink，异步写库。

**后端**：
- [ ] schema：`opssystemlog`（level/component/message/request_id/client_request_id/...）
- [ ] slog sink（异步、带背压保护）
- [ ] 运行时级别控制（slog.SetDefault 动态换 level）
- [ ] API：日志列表（11维筛选）+ 清理 + 级别 get/update/reset

**前端**：
- [ ] 系统日志表（筛选 + 分页）
- [ ] 运行时级别调整面板

**依赖**：无。
**工作量**：M（1-2天）

---

### M12 错误日志增强 + trace ⚠️ 需扩插件上报

**目标**：请求/上游错误拆分、模型映射、分段延迟、入站/上游端点、关联上游错误 trace。

**采集方案**：⚠️ **需扩插件上报 schema**。分段延迟（auth/routing/upstream/response）、入站端点、trace 链路都得插件在 Forward 各阶段埋点上报。

**后端**：
- [ ] 扩 `ops.report_request` payload（分段延迟、endpoint、upstream_model 映射）
- [ ] trace 关联表（一次请求 → 多次上游尝试）
- [ ] 查询 API 拆分请求错误/上游错误

**前端**：
- [ ] 错误表增强列 + 双入口 + 详情字段 + trace 钻取

**依赖**：⚠️ **改所有 gateway 插件上报**（openai/claude/kiro 都要加字段）。工作量最大的一块。
**工作量**：L（3-5天，含多插件改造）

---

### M13-M16（P2）

- **M13 切换率趋势**：Core 聚合 scheduler 的账号切换事件。S。
- **M14 OpenAI Token 统计**：Core 聚合 ops_request_log 按 model。S。
- **M15 设置中心**：阈值染色配置/保留策略/邮件报告/错误过滤开关。M。
- **M16 增强交互**：吞吐图缩放/URL深链/全屏/自动刷新可调。S。

---

## 建议执行顺序（考虑依赖和价值）

```
M1 系统资源(6卡)   ──┐
M2 时间范围+下钻   ──┼─→ 立即可见"运维监控"丰满感
M6 请求详情钻取    ──┘   (都独立、不卡插件、S-M 工作量)

M8 全分位+TTFT ──┐
M9 延迟直方图   ──┼─→ 指标体系完整
M10 错误分布饼图 ─┘

M7 并发四维卡 ──→ 调度透明度

M3 告警闭环(依赖M1) ──→ 主动监控
M4 健康分数(依赖M1+M8) ──→ 一眼看全局

M5 WebSocket ──→ 实时性升级(架构改造)
M11 系统日志 ──→ 排障能力

M12 错误增强+trace ──→ 最难(需改插件)，最后做
```

**第一批推荐**：M1 + M2 + M6（都是独立、S-M、纯 Core，做完 OpsPage 立刻丰满）。
