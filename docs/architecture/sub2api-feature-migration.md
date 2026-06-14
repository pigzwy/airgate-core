# sub2api vs airgate 功能与架构差异核对

## 状态

核对文档（基于真实代码）。目的：把 sub2api（单体）与 airgate（Core + 插件）逐项功能对照清楚，作为"哪些已有、哪些缺、缺的该放哪、怎么做"的事实依据。

## 核对口径

- **airgate Core**：本地代码核对，准确。
- **airgate 插件**：已全部 clone 到 `D:\java\github\` 核对，准确——`airgate-openai`、`airgate-epay`、`airgate-health`、`airgate-claude`、`airgate-kiro`。
- **sub2api**：本地源码核对，准确。
- epay / health 两节为**逐功能点精确核对**（见第四、五节）。claude/kiro 用户暂不关注，未深核。

图例：✅ 已有　🔶 部分有/需增强　❌ 没有

---

## 一、架构差异（根本不同）

| 维度 | sub2api | airgate |
|------|---------|---------|
| **整体架构** | 单体：所有功能编进一个二进制 | Core 内核 + 独立插件进程（gRPC / hashicorp go-plugin） |
| **网关实现** | 内置 OpenAI/Claude/Gemini/Antigravity/Bedrock | 由 gateway 插件提供，Core 不转发 AI 请求 |
| **支付实现** | 内置 5 渠道 | 由 payment 插件提供（airgate-epay） |
| **运维监控** | 内置 Ops（70+ 文件，转发热路径埋点） | 几乎无（仅 health 插件做主动探测） |
| **后端分层** | service 直连 ent | 严格四层：dto→handler→service→Repository(接口)→store→ent；service 禁止 import ent |
| **前端** | Vue 3（172 组件，60+ 页） | React 19 + TanStack Query（约 15 页） |
| **扩展方式** | 改源码 | 插件热插拔，Core 不重启 |
| **插件↔核心** | 无（单体） | 插件经 `Host.Invoke` 调 Core；Core 经 gRPC 调插件 |

**迁移含义**：sub2api 代码不能直接拷（直连 ent，违反 airgate 红线），必须按四层重写；且要先判断功能进 Core 还是做插件。

---

## 二、★最关键的架构发现：运维数据从哪来

这决定运维功能能不能做、怎么做，是整个迁移最核心的技术难题。

- **sub2api**：转发在主进程内，每个请求在 `gateway.Forward` 直接埋点 `RecordError/RecordSuccess` → 落库 `usage_logs/ops_error_logs` → 后台 `OpsMetricsCollector`（1分钟 cron + leader lock）聚合成 RPS/延迟分位/错误率 → WebSocket 推前端。**真实用户流量全程可见**。
- **airgate**：转发在**插件子进程**里。Core 默认看不到请求级细节（延迟、token、错误体、上游状态码）。
- **airgate-health**：是**主动心跳探测**（自己发测试请求采集可用性），**采集不到真实用户流量**。

**三种可选采集路径**（需拍板）：
1. **插件经 Host.Invoke 上报**：每次 Forward 后，插件把请求级指标回传 Core 落库。改动所有 gateway 插件，但数据最全最准。
2. **Core 在 ext-proxy/forwarder 层埋点**：Core 转发到插件的出入口处采样。改动集中在 Core，但拿不到插件内部细节（如真实上游延迟/错误体）。
3. **混合**：基础指标 Core 埋点（路径 2），详细错误/token 插件上报（路径 1）。

> 这是运维迁移的**前置设计决策**，建议优先定。已有线索：airgate 计费链路（billing/recorder）已经在记 UsageLog，说明插件已能经 Host 上报 token/cost——可在此基础上扩展为 Ops 采集。

---

## 三、功能总览：已有 / 缺失

### airgate 已有（含插件）✅

| 功能 | airgate 落位 |
|------|------------|
| 用户/登录/JWT/API Key/余额/分组/订阅/计费/调度/并发/代理/仪表盘/设置/向导/升级 | Core 内置 |
| OpenAI / Claude / Kiro 网关 | gateway 插件 |
| 支付（易支付虎皮椒+彩虹/支付宝/微信） | airgate-epay 插件 |
| 渠道可用性监控 + 公开状态页 | airgate-health 插件 |

### airgate 缺失或较弱（按用户优先级）

| 类别 | 缺什么 | 落位 |
|------|--------|------|
| **运维 Ops** ★最需要 | 实时大盘、告警、错误日志/请求详情、系统日志查询、健康分数 | Core 新增 `app/ops` |
| **风控/审核** ★需要 | 内容审核、违规封禁、错误透传规则、模型黑名单 | Core 新增 `app/moderation` |
| **支付细节** ★对齐 | 退款、订单状态机、审计日志、运营仪表板、Resume Token 等（详见第四节） | 增强 epay 插件 |
| **渠道监控细节** ★对齐 | challenge 校验、请求模板、小时级聚合（详见第五节） | 增强 health 插件 |
| 计费营销 | 卡密、优惠码、推广返利、订阅计划商品化 | Core 新增 |
| 认证增强 | 多 OAuth 登录、TOTP 2FA、邮箱绑定 | Core 增强 auth |
| 平台特性 | 公告、自定义页、用户属性、简易/后端模式、数据备份 | Core 新增 |
| 网关（不急） | Gemini、Antigravity、Bedrock | 新建插件（暂缓） |

---

## 四、附录：支付细节差距（epay 真实核对）

airgate-epay 基础功能完整，但比 sub2api 弱在以下点（用户要"细节一样"，这些是待补项）：

### 支付渠道
- epay：易支付（虎皮椒 + 彩虹两家）、支付宝官方、微信官方
- sub2api 多出：**Stripe（国际卡）**、**Airwallex（跨境）**、多币种（CNY/USD）

### 订单与退款（差距最大，优先级高）
| 功能 | epay | sub2api | 差距 |
|------|------|---------|------|
| 订单状态机 | 5 态 | 9+ 态（充值中→完成、退款申请→退款中→已退款） | 补全 |
| 退款 | 有表无逻辑 | 完整：用户申请→管理员审批→执行 | **补全业务逻辑** |
| 订单取消 | 无 | AdminCancelOrder | 补 |
| 订单商户快照 | 无 | provider_snapshot(JSONB) | 补 |
| 支付审计日志 | 无 | payment_audit_logs（全操作链） | 补 |

### 运营分析（管理后台仪表板，优先级高）
sub2api 有、epay 无：日营收趋势(daily_series)、支付方式分布、Top 用户排行、日订单数。

### 商品化与恢复
- **订阅计划商品化**：sub2api 把订阅计划做成可购买商品（充余额 + 买套餐两种模式），epay 仅充余额
- **Resume Token**：sub2api 支持无登录 JWT 链接恢复订单（微信 OAuth 场景），epay 无
- **充值倍数**：sub2api 支持充 100 得 150，epay 无

---

## 五、附录：运维细节差距（health 真实核对）

### 渠道监控（airgate 已有，但 sub2api 更丰富）
| 功能 | health | sub2api | 差距 |
|------|--------|---------|------|
| 定时探测可用性 | ✅ 分组级 5min | ✅ 渠道级 15s~1h 可配 | health 周期粗 |
| 风控 challenge 校验 | ❌ | ✅ 算术校验防假成功 | 补 |
| 自定义请求模板 | ❌ | ✅ headers/body merge/api_mode | 补 |
| 历史 + 聚合 | ✅ 日桶 | ✅ 明细→日聚合，小时级时序 | health 粒度粗 |
| 可用率/延迟分位 | ✅ p50/95/99 | ✅ 同 + 任意窗口 | 接近 |
| 公开状态页 | ✅ | ❌ | **health 反而更强** |

### Ops 运维大盘（health 几乎全无，sub2api 完整）★最需要
| 功能 | sub2api 实现 | 关键文件 |
|------|------------|---------|
| 实时大盘 RPS/错误率/延迟直方图/并发/吞吐 | OpsMetricsCollector 1min 聚合 | `ops_metrics_collector.go`/`ops_dashboard.go` |
| WebSocket 实时推送 | 1s/5s 推 QPS/TPS 快照 | `ops_ws_handler.go` |
| 告警（规则+评估器+事件+静音+邮件限流） | 1min cron 评估 | `ops_alert_evaluator_service.go` |
| 错误日志/请求详情（完整请求响应、上游分析、钻取） | RecordError 落 20+ 字段 | `ops_service.go`/`ops_request_details.go` |
| 系统日志（多维索引、运行时级别调整） | 异步 sink | `ops_system_log_service.go`/`ops_log_runtime.go` |
| 健康分数（0-100，业务+基础设施） | gopsutil 采集 | `ops_health_score.go` |
| OpenAI Token 统计 | 按 model/user/key | `ops_openai_token_stats.go` |
| 数据清理/聚合/定时报告 | cron | `ops_cleanup/aggregation/scheduled_report` |

### 风控/内容审核（airgate 全无）★需要
| 功能 | sub2api | 关键文件 |
|------|---------|---------|
| 内容审核（敏感词 + OpenAI Moderation API） | ✅ | `content_moderation.go` |
| 输入审核（observe/pre_block 模式）+ 脱敏 | ✅ | `content_moderation_input/redact.go` |
| 违规封禁（N 次/M 小时）+ 邮件通知 | ✅ | `content_moderation.go`/`_email.go` |
| 错误透传规则（按错误码自定义策略） | ✅ | `error_passthrough_service.go` |
| 模型黑名单（all/include/exclude） | ✅ | `content_moderation.go` |

---

## 六、下一步建议（按优先级）

聚焦用户诉求：**运维 + 风控（新建）** 和 **支付/渠道监控细节对齐（增强插件）**。

1. **先定运维数据采集路径**（第二节的 3 选 1）——这是 Ops 一切的前提，不定无法动工。
2. **运维 MVP**：Core 新增 `app/ops`，先做「错误日志 + 实时基础指标」最小闭环（数据采集 → 落库 → 一个 React 大盘页），验证采集路径可行。
3. **告警系统**：在 Ops 数据之上加规则评估 + 邮件（复用 settings 的 SMTP）。
4. **风控**：Core 新增 `app/moderation` + 错误透传规则。
5. **支付细节对齐**：按第四节，优先补退款 + 订单状态机 + 运营仪表板（增强 epay 插件）。
6. **渠道监控对齐**：按第五节，补 challenge 校验 + 请求模板（增强 health 插件）。

> 每个 Core 新 domain 参照 `account` 全链路；每个 Ent schema 变更要 `make ent` 提交生成代码。

---

## 七、运维 Ops MVP 设计（已定方案）

**决策**：采集路径 = 插件上报；第一个动手 = Ops MVP。

### 7.1 地基确认（已核实代码）

airgate 的 `Host.Invoke` 是「通用平台原语」（ADR-0001）：新增能力**只需在 `host_service.go` 的 `invoke()` 加一个 method case，无需改 proto / SDK 协议**。现有 `gateway.forward`、`scheduler.report_account_result` 等都是这么挂的。
→ Ops 上报可干净接入：插件调 `Host.Invoke("ops.report_request", payload)`，Core 加 case 落库。

参考点：`host_service.go:169-188`（method 常量）、`:198`（switch 分发）、计费上报链路 `billing/recorder.go`（插件已能经 Host 上报 token/cost，Ops 在此基础上扩展）。

### 7.2 MVP 范围（最小闭环，验证采集路径）

只做「错误日志 + 实时基础指标」，跑通端到端：

```
插件 Forward 后
  → Host.Invoke("ops.report_request", {request_id, platform, model, user_id,
       api_key_id, account_id, duration_ms, status_code, success,
       error_kind, error_msg, input_tokens, output_tokens, ts})
  → Core host_service 新增 case → ops service 落库 ops_request_log 表
  → 后台聚合器(1min) → 算 RPS/错误率/p50/p95 → 落 ops_window_stat 表
  → React 管理页轮询(MVP 先不上 WebSocket) → 展示实时大盘 + 错误列表
```

### 7.3 改动清单（按 airgate 四层）

**SDK 侧**（airgate-sdk，可能需要）
- [ ] 确认 SDK 是否已有 Host 上报 helper；没有则加一个 `host.ReportRequest(...)` 便捷方法（可选，也可插件直接 Invoke）

**插件侧**（先改 airgate-openai 一个验证，再铺开 claude/kiro）
- [ ] 在 `gateway.go` Forward 收尾处，组装指标并 `Host.Invoke("ops.report_request", ...)`
- [ ] manifest 声明所需 capability（如 `host.ops`）

**Core 侧**（主战场，按四层）
- [ ] `ent/schema/opsrequestlog.go` + `opswindowstat.go` → `make ent`
- [ ] `app/ops/{service,types,errors}.go`：定义 `ReportInput`、`Repository` 接口、查询用 `Filter/Result`
- [ ] `infra/store/ops_store.go`：Repository 的 ent 实现
- [ ] `plugin/host_service.go`：加 `hostMethodOpsReport = "ops.report_request"` 常量 + case + capability 校验
- [ ] `app/ops` 后台聚合器：1min cron 算 window_stats（参考 health 插件的 aggregator 思路）
- [ ] `server/dto/ops.go` + `server/handler/ops_handler{,_routes,_mapper}.go`：管理员查询 API（实时指标、错误列表、错误详情）
- [ ] `bootstrap/http_handlers.go` + `server/router.go`：接线（store→service→handler，挂 adminGroup）

**前端侧**（React，MVP 简化版）
- [ ] `web/src/pages/admin/OpsPage.tsx`：实时大盘（指标卡 + 趋势图，轮询）+ 错误日志表 + 错误详情弹窗
- [ ] `web/src/shared/api/ops.ts` + queryKeys
- [ ] AppShell 加菜单入口

### 7.4 MVP 后的扩展（不在首期）

WebSocket 实时推送、告警系统、系统日志查询、健康分数、OpenAI Token 统计、数据清理/定时报告——验证 MVP 采集路径可行后再逐个加。

### 7.5 风险点

- **采集对热路径的影响**：上报必须异步、失败不阻断转发（参考 sub2api 的异步 sink）。
- **数据量与清理**：ops_request_log 增长快，MVP 就要带保留期清理，否则撑爆库。
- **capability 时序**：插件 Init 阶段不能调 Host RPC（capability 未绑），上报要在 Forward 运行期，没问题。
- **多插件改造成本**：MVP 先只改 openai 验证，通了再铺 claude/kiro。

---

## 八、Ops 前端差距清单（精确核对，2026-06-14）

> 基于 sub2api `frontend/src/views/admin/ops/`（19 个 .vue 组件 + `api/admin/ops.ts` 40+ 接口）与 airgate 当前 OpsPage（4 指标卡 + RPS/错误率折线 + 错误日志表 + 15s 轮询）逐项对比。当前覆盖度约 **15-20%**。

### 顶部工具栏
| 功能 | sub2api | airgate |
|------|---------|---------|
| 全局平台筛选（联动所有图） | ✅ | 🔶 仅错误表有 |
| 分组 group 筛选 | ✅ | ❌ |
| 时间范围切换 5m/30m/1h/6h/24h + 自定义区间 | ✅ | ❌ 固定 1min/1h |
| 查询模式 auto/raw/preagg | ✅ | ❌ |
| 手动刷新 / 全屏大屏 / URL 深链 | ✅ | ❌ |

### 健康与实时
| 功能 | sub2api | airgate |
|------|---------|---------|
| 健康分数环（0-100 三色）+ Hover 诊断报告（自动列 critical/warning 项+处置建议） | ✅ | ❌ |
| 实时 QPS/TPS 大数字（当前/峰值/平均）+ 心跳动画 | ✅ | 🔶 仅 RPS 单值 |
| WebSocket 实时推送 QPS | ✅ | ❌（15s 轮询） |

### 业务指标卡（sub2api 6 张）
| 卡 | airgate |
|----|---------|
| 请求卡（总数/token/均QPS/均TPS + 详情下钻） | 🔶 仅窗口请求数 |
| SLA 卡（进度条+异常数） | ❌ |
| 请求错误率（%+错误数+业务限流数+染色） | 🔶 仅 % |
| 请求耗时 P99+P95/P90/P50/Avg/Max 五件套 | 🔶 仅 P95 |
| TTFT 首 token 延迟（全套分位） | ❌ |
| 上游错误卡（上游错误率+排除429/529+429/529数） | ❌ |

### 系统资源卡（sub2api 6 张）
CPU/内存/数据库连接/Redis连接/Goroutines/后台Jobs心跳 —— airgate **全部 ❌**

### 并发/可用性卡
平台/分组/账号/用户四维并发统计 + 账号可用性（限流/过载倒计时）+ 进度条 —— airgate **全部 ❌**

### 趋势/分布图
| 图 | airgate |
|----|---------|
| 吞吐趋势（QPS+TPS 双轴 + 平台/分组下钻 + 缩放） | 🔶 仅 RPS+错误率 |
| 切换率趋势（账号轮换负载） | ❌ |
| 延迟直方图（bucket 分布柱状） | ❌ |
| 错误分布饼图（按错误码分类） | ❌ |
| 错误趋势（SLA/上游/限流三线） | 🔶 仅错误率单线 |

### OpenAI Token 统计（按 model 表 + TopN + 时间范围）—— ❌ 全缺

### 错误日志（airgate 有基础，但缺很多）
| 功能 | airgate |
|------|---------|
| 请求错误 vs 上游错误两个入口 | ❌ 混合表 |
| 搜索框 / 状态码筛选 / phase 筛选 / owner 筛选 / view 模式 | ❌ |
| 表列：endpoint / 模型映射 requested→upstream / group / user_email / severity / request_type 徽章 | ❌ |
| 详情：client_request_id / 模型映射 / 入站端点 / 上游端点 / request_type / 分段延迟(auth/routing/upstream/response) / 关联上游错误 trace | ❌ |
| 请求明细（成功/慢请求列表，不只错误） | ❌ |

### 告警模块
规则管理（13种指标+运算符+阈值+窗口+持续+严重度+冷却+邮件+filters）/ 事件流（状态徽章+severity+游标分页）/ 静默（按规则+平台+group+region）/ 手动解决 —— airgate **全部 ❌**

### 设置中心（8 大块）
运行时（评估间隔+分布式锁+全局静默）/ 指标阈值染色 / 告警邮件 / 报告邮件(每日每周摘要) / 数据保留 / 预聚合开关 / OpenAI 配额自动暂停 / 错误过滤开关(5类) / 自动刷新可调 —— airgate **全部 ❌**

### 系统日志模块
日志表（11维筛选）/ 日志清理 / Sink 健康指标 / 运行时日志级别调整(不重启) —— airgate **全部 ❌**

### 补齐优先级
- **P0（架构性/最易感知）**：健康分数+诊断、WebSocket 实时、时间范围+维度下钻、请求详情钻取(成功也看)、告警闭环
- **P1（运维必备）**：并发四维卡、系统资源6卡、TTFT+全分位、延迟直方图、错误分布饼图、错误日志增强+trace、系统日志+运行时级别
- **P2（增强）**：切换率、吞吐缩放、URL深链、全屏、OpenAI Token 统计、设置中心

### 关键文件参考
- sub2api 主页：`frontend/src/views/admin/ops/OpsDashboard.vue`
- API：`frontend/src/api/admin/ops.ts`（40+ 接口）
- 组件：`frontend/src/views/admin/ops/components/`（19 个，最大 OpsDashboardHeader.vue 70KB）
- 后端：`backend/internal/service/ops_*.go`（70+ 文件）
