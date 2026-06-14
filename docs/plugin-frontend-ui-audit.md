# 插件前端 UI 一致性审查

> 审查日期：2026-06-14
> 范围：airgate-core 主应用 + 各网关/扩展插件的前端组件
> 目的：确认插件前端与 core 设计语言（navy 结构 / gold 单一强调 / green 语义 / token 化）的一致性

---

## 1. 背景：插件前端是怎么回事

插件前端**不是独立页面**，而是一组**嵌入式 React 组件**，通过 `PluginFrontendModule` 接口注入到 core 的 admin 界面里。

以 `airgate-openai` 为例，`web/src/index.ts` 导出 8 个组件：

| 组件 | 注入位置（core admin 里） |
|------|--------------------------|
| `AccountForm` | 账号创建/编辑 Modal 的表单字段 |
| `AccountIdentity` | 账号列表的身份显示 |
| `UsageWindow` | 账号用量窗口 |
| `UsageModelMeta` / `UsageMetricDetail` / `UsageCostDetail` | 用量记录详情 |
| `platformIcon` | 平台图标 |

**例外**：`airgate-health` 还有一个 **standalone 状态页**（`web/src/status/`），由 core 反代 `/status/*` 对外公开，**不嵌入 admin**，有独立视觉语境。

### 样式机制

插件组件通过两种方式取色：
1. `var(--ag-*)` CSS 变量 / `cssVar()` helper —— 走 core 提供的品牌 token
2. 硬编码 hex / rgb —— 不走 token（一致性风险点）

插件依赖的 `--ag-*` 变量**由 core 的 `theme.css` 提供**。core 在 `5c3d423` 提交里删掉了外部 SDK 主题包、改为自建 `theme.css`，因此 core 必须保证插件用到的所有 `--ag-*` 变量都有定义。

---

## 2. Token 兼容性（已修复）

### 问题
插件组件使用旧 SDK `airgate-theme` 的变量命名，core 重建 theme 后有 **4 个变量取不到值**，会导致插件组件样式崩坏（文字/背景/反色失效）。

### 修复（core `web/src/styles/theme.css` 第 02b 段）
补 4 个兼容别名，指向 core 现有 token：

```css
:root, .light, .default, .dark,
[data-theme="light"], [data-theme="default"], [data-theme="dark"] {
  --ag-bg-surface: var(--ag-surface);
  --ag-text-secondary: var(--ag-muted);
  --ag-text-tertiary: var(--ag-muted);
  --ag-text-inverse: var(--ag-primary-foreground);
}
```

### 状态：✅ 已修
插件用到的 15 个 `--ag-*` 变量现已全部在 core 定义。**此别名一次性覆盖所有插件**（不止 openai）的 token 兼容性。

> 旧 SDK 命名 → core 现命名对照：
> | 插件用（旧） | core 现有 | 别名指向 |
> |---|---|---|
> | `--ag-bg-surface` | `--ag-surface` | ✅ |
> | `--ag-text-secondary` | `--ag-muted` | ✅ |
> | `--ag-text-tertiary` | （无，回退 muted） | ✅ |
> | `--ag-text-inverse` | `--ag-primary-foreground` | ✅ |

### 给插件开发者的约定
新增插件请优先用 core 现命名（`--ag-surface` / `--ag-muted` / `--ag-primary-foreground`），上述别名仅为向后兼容保留。

---

## 3. 各插件硬编码颜色审查

扫描结果（本地存在的 5 个插件；playground / studio 未克隆，未扫描）：

| 插件 | 硬编码 hex 种类 | rgba()/rgb() | ag token 引用 | token 化程度 |
|------|----------------|--------------|--------------|--------------|
| airgate-openai | 8（已处理） | — | 多 | 良好 |
| airgate-claude | 1 | 6 | 95 | 优秀 |
| airgate-kiro | 0（rgb 4） | 5 | 51 | 良好 |
| airgate-epay | 1 | 5 | 239 | 优秀 |
| airgate-health | 4 | 1 | 35 | 良好（standalone） |

### 逐项判定

#### airgate-openai —— ✅ 已修复（方案 A）
**位置**：`AccountForm.tsx` 套餐徽章 `planDisplayMap`（Free/Plus/Pro/Team）
**原问题**：硬编码浅底（`#f3f4f6` / `#d1fae5` / `#ede9fe` / `#dbeafe`），**深色模式刺眼**
**修复**：底色改为文字色的 14% 透明度，深浅主题都柔和，保留套餐辨识度
```js
free: { color: '#6b7280', bg: 'rgba(107,114,128,0.14)' }
plus: { color: '#059669', bg: 'rgba(5,150,105,0.14)' }
pro:  { color: '#7c3aed', bg: 'rgba(124,58,237,0.14)' }
team: { color: '#2563eb', bg: 'rgba(37,99,235,0.14)' }
```

#### airgate-claude —— 🟢 合理，无需改
**唯一硬编码**：`ClaudeIcon.tsx` 的 `#D97757`（Claude 品牌官方锈红色）
**判定**：品牌图标专用色，保留正确。token 引用 95 处，是 5 个插件里 token 化最规范的之一。

#### airgate-kiro —— 🟡 类别色，建议保留
**位置**：`UsageModelMeta.tsx` 推理强度等级色（`EFFORT_COLORS`）
```js
low: 'rgb(34,197,94)'     // 绿
medium: 'rgb(59,130,246)' // 蓝
high: 'rgb(249,115,22)'   // 橙
xhigh: 'rgb(239,68,68)'   // 红
```
**判定**：low/medium/high/xhigh 是 4 个并列等级，用颜色编码强度是合理的"类别色"（同 Dashboard 图表多色）。绿→红的渐进语义清晰。
**可选改进**：若要严格品牌化，可改成单色明度梯度，但会降低强度辨识度。**建议不动。**

#### airgate-epay —— 🟢 基本合理
**唯一硬编码**：`OrdersPage.tsx:295` 的 `color: '#fff'`（某彩色按钮上的白字）
**判定**：token 引用 239 处，token 化最彻底。`#fff` 是按钮文字，若按钮底是品牌色则合理。
**可选改进**：核对该按钮底色，必要时把 `#fff` 换 `var(--ag-primary-foreground)`。低优先级。

#### airgate-health —— 🟡 状态语义色，合理
**位置**：`status/StatusPage.tsx` uptime 健康度柱状色
```js
uptime >= 99.5 → '#22c55e'  // 绿（健康）
uptime >= 95   → '#eab308'  // 黄（警告）
uptime < 95    → '#ef4444'  // 红（故障）
total === 0    → '#ffffff'  // 白（无数据）
```
**判定**：①这是**数据语义色**（健康度红黄绿是行业通用约定）②health 状态页是 **standalone 公开页**，不嵌入 admin，有独立视觉语境。**保留正确。**
**可选改进**：绿/黄/红可统一到 core 的 `--ag-success`/`--ag-warning`/`--ag-danger` token，让状态页和 admin 语义色一致。中优先级（仅当希望状态页也走 core token 时）。

---

## 4. 结论与建议

### 总体评价
**插件前端整体 token 化程度高**（claude 95 / epay 239 处 token 引用），硬编码颜色集中在**类别色 / 语义色 / 品牌图标色**这三类**合理场景**，不是随意散落的破绽。

### 已完成
1. ✅ core 补 4 个兼容别名 → 所有插件 token 不再崩
2. ✅ openai 套餐徽章深色刺眼修复（方案 A）

### 建议保留（合理的颜色编码，不算不一致）
- claude 品牌图标 `#D97757`
- kiro 推理强度 4 色（类别色）
- health uptime 红黄绿（状态语义色）

### 可选改进（按需，非必须）
| 项 | 优先级 | 说明 |
|----|--------|------|
| health 状态色 → core 语义 token | 中 | 仅当希望 standalone 状态页与 admin 语义色统一 |
| epay `#fff` → `--ag-primary-foreground` | 低 | 核对按钮底色后决定 |
| kiro/openai 类别色 → 单色明度梯度 | 低 | 会降低辨识度，不推荐 |

### 未覆盖
- **playground / studio**：本地未克隆，未扫描。建议克隆后用本文档的方法（扫 `#hex` / `rgba(` + 核对 token 引用）补审。

---

## 5. 复用此审查的命令

```bash
# 扫某插件的硬编码色与 token 使用
cd <plugin>/web
grep -rhoE "#[0-9a-fA-F]{3,6}\b" src/ --include="*.tsx" | sort -u         # 硬编码 hex
grep -rhoE "rgba?\([0-9]" src/ --include="*.tsx"                          # rgba/rgb
grep -rhoE "var\(--ag-[a-z0-9-]+\)|cssVar\(" src/ --include="*.tsx" | wc -l # token 引用数

# 核对插件用的 --ag-* 变量是否都在 core 定义
cd airgate-core/web/src
grep -oE "^\s*--ag-[a-z0-9-]+:" styles/theme.css | sed 's/[: ]//g' | sort -u > /tmp/core_vars.txt
cd <plugin>/web
grep -rhoE "var\(--ag-[a-z0-9-]+\)" src/ | sed 's/var(//;s/)//' | sort -u > /tmp/plugin_vars.txt
comm -23 /tmp/plugin_vars.txt /tmp/core_vars.txt   # 输出 = core 缺失的变量

# 构建并同步插件到 core（dev）
cd <plugin>/web && pnpm build
cp dist/index.js ../../airgate-core/backend/data/plugins/<plugin-id>/assets/index.js
# 重启 core 生效
```
