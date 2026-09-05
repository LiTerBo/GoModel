# GoModel Dashboard 多主题切换机制分析

> **分析日期：** 2026-09-05
> **涉及范围：** `web/dashboard/` 前端
> **核心文件：** `themes.css`, `ui.svelte.js` (ThemeStore), `ThemeToggle.svelte`, `ChartCanvas.svelte`, `chartTheme.js`

---

## 概览

GoModel Dashboard 的多主题切换机制采用**四层架构**：CSS 变量主题定义 → 状态管理驱动 → 切换控件 → 组件消费。支持**浅色 / 系统跟随 / 深色**三态切换，且通过 `tick` 版本号机制确保 Chart.js 图表颜色与 CSS 变量同步更新。

---

## 一、CSS 变量层：三层级定义

**入口：** `main.js` → `dashboard.css` → 首行 `@import "./themes.css"`。

**`themes.css`** 定义了三个层级的 CSS 变量，使用 `@import` 顺序确保全局生效：

| 层级 | 选择器 | 场景 |
|------|--------|------|
| **Dark (默认)** | `:root` | 无 `data-theme` 属性时生效，设置 `color-scheme: dark` |
| **Light (显式)** | `[data-theme="light"]` | 用户明确选择浅色时，通过 `data-theme` 属性覆盖 |
| **Light (系统)** | `@media (prefers-color-scheme: light)` 内的 `:root:not([data-theme="dark"])` | 系统浅色模式 + 未锁定主题时 |

### 关键设计决策

Light 主题的 CSS 变量**重复写了两次**（`[data-theme="light"]` 和系统媒体查询块），原因：

1. `themeStore.apply()` 在 `system` 模式下会**移除** `data-theme` 属性
2. 此时显式 `[data-theme="light"]` 选择器不匹配，全靠系统媒体查询兜底
3. **不用** `light-dark()` CSS 函数：`chartTheme.js` 通过 `getComputedStyle` 读取变量，`light-dark()` 的 token 流会原样传递给 Chart.js 导致解析失败

### 共同变量体系（~20 个语义变量）

```
--bg, --bg-surface, --bg-surface-hover         背景
--border, --text, --text-muted                 边框/文字
--accent, --accent-hover                       强调色
--success, --info, --warning, --danger         语义色
--chart-grid, --chart-text, --chart-day-marker 图表轴
--chart-tooltip-bg, --chart-tooltip-border, --chart-tooltip-text 图表提示
--cal-level-0 ～ --cal-level-10               日历热力图 11 级色阶
--token-input, --token-output, --token-prompt, --token-local  Token 吞吐量
--cache-meter-uncached, --cache-meter-local, --cache-meter-prompt  缓存仪表
--sidebar-width, --radius                      布局
```

---

## 二、状态管理层：`ThemeStore`（`ui.svelte.js`）

```js
class ThemeStore {
  theme = $state("system");   // "light" | "system" | "dark"
  tick = $state(0);           // 版本号，驱动图表重建
}
```

### 核心方法

| 方法 | 行为 |
|------|------|
| **`init()`** | 从 `localStorage` 读取 `gomodel_theme`（默认 `"system"`）；调用 `apply()` 设置 DOM；监听 `prefers-color-scheme: dark` 媒体查询变化 → 仅在 `system` 模式递增 `tick` |
| **`set(t)`** | 写入 `localStorage` + `apply()` + `tick++` |
| **`toggle()`** | 循环切换 `light → system → dark` |
| **`apply()`** | `system` → 移除 `data-theme`；`light`/`dark` → `setAttribute("data-theme", t)` |

### 持久化

通过 `storage.js` 的 `readStored`/`writeStored` 操作 `localStorage`，key 为 `gomodel_theme`。`storage.js` 对 `window` 不存在或 `localStorage` 被禁用做了防御性处理，返回 `null` 而非抛出异常。

---

## 三、交互控制层：`ThemeToggle`（`organisms/ThemeToggle.svelte`）

### 两种形态，根据可用空间自适应

| 形态 | 触发条件 | 交互方式 |
|------|---------|---------|
| **三按钮 Pill**（宽） | 侧边栏展开 + 视口 > 768px | 三个带图标的按钮（☀️ / 🖥 / 🌙），直接选择 |
| **单按钮循环**（窄） | `compact=true` 或视口 ≤ 768px | 一个按钮，点击按 `light → system → dark` 循环 |

### 实现细节

- 使用 `lucide` 图标（`Sun`, `Monitor`, `Moon`）
- 按钮通过 `aria-pressed` 标识当前选中态
- 窄模式通过 `themeStore.toggle()` 循环，按钮 `title` 动态显示当前操作（如 "切换到浅色主题"）
- 放在 `Sidebar.svelte` 的底部 `sidebar-footer`，传入 `compact={sidebar.collapsed}`

---

## 四、组件消费层

### 4.1 全局样式文件

所有 `*.css` 模块（`layout.css`, `cards-charts.css`, `tables.css`, `forms.css`, `buttons.css` 等 12 个模块）通过 `var(--bg)`、`var(--text)`、`var(--border)` 等引用主题变量，无需额外操作即可切换。

### 4.2 Chart.js 图表（`ChartCanvas.svelte` + `chartTheme.js`）

这是主题切换中最复杂的联动环节：

**`ChartCanvas.svelte`** 在 `@attach` 回调中读取 `themeStore.tick`，当 `tick` 变化时销毁旧 Chart 实例并重建：

```svelte
{#function chart(canvas)}
  void themeStore.tick;  // 追踪主题变化
  const config = build();
  const instance = new Chart(canvas.getContext("2d"), config);
  return () => instance.destroy();
{/function}
```

**`chartTheme.js`** 提供颜色读取工具：

| 函数 | 作用 |
|------|------|
| `cssVar(name)` | 通过 `getComputedStyle(document.documentElement)` 实时读取 CSS 变量 |
| `chartColors()` | 聚合 `--chart-grid`, `--chart-text`, `--chart-tooltip-*` |
| `resolveCssColor(expr)` | 通过临时 DOM 元素解析 `color-mix()` 等复杂 CSS 表达式为 `rgb()` 字符串 |
| `labelColor(label)` | 使用 djb2 哈希确定性分配调色板颜色，跨图表一致 |

### 4.3 日历热力图

使用 `--cal-level-0` 到 `--cal-level-10` 共 11 级变量：
- **暗色主题：** 从深蓝（`#161b22`）递增到亮蓝（`--info` 掺白），10 个可区分梯度
- **浅色主题：** 从浅灰（`#ebedf0`）递增到深蓝（`--info` 掺黑）

---

## 五、数据流图

```
用户点击 ThemeToggle
    │
    ▼
themeStore.set(t) / themeStore.toggle()
    │
    ├──→ localStorage.setItem("gomodel_theme", t)    ← 持久化
    │
    ├──→ document.documentElement.data-theme = t/移除  ← CSS 级联生效
    │         │
    │         └──→ :root / [data-theme="light"] / @media   ← 所有 CSS 变量更新
    │
    └──→ themeStore.tick++
              │
              └──→ ChartCanvas.svelte 检测变化
                        │
                        └──→ destroy old Chart → new Chart → cssVar() 读新颜色
```

---

## 六、设计特点总结

| 特点 | 说明 |
|------|------|
| **三态切换** | 浅色 / 系统跟随 / 深色，满足不同用户习惯 |
| **持久化** | `localStorage` 的 `gomodel_theme` key，跨会话保留 |
| **零 JS 闪烁** | CSS 变量直接驱动全部样式，无 JS 计算颜色开销 |
| **图表同步** | `tick` 版本号驱动 Chart.js 重建，确保 canvas 颜色与 CSS 一致 |
| **SSR 安全** | `storage.js` 和 `chartTheme.js` 对 `window`/`document` 做防御性检查 |
| **响应式** | 768px 断点 + 侧边栏折叠状态驱动两种切换控件形态 |
| **无外部依赖** | 纯 CSS 自定义属性 + Svelte 响应式，无 Tailwind/ThemeProvider |
| **注释即文档** | `themes.css` 注释详细解释了 light-dark() 不可用的原因、日历色阶的设计意图、token 颜色的语义对应关系 |

---

## 七、关键文件清单

| 文件 | 用途 |
|------|------|
| `web/dashboard/src/styles/themes.css` | 三层 CSS 变量定义（dark/light 显式/light 系统） |
| `web/dashboard/src/lib/stores/ui.svelte.js` | `ThemeStore` 主题状态管理 |
| `web/dashboard/src/lib/components/organisms/ThemeToggle.svelte` | 主题切换 UI 控件 |
| `web/dashboard/src/lib/components/molecules/ChartCanvas.svelte` | Chart.js 容器，响应 `tick` 重建 |
| `web/dashboard/src/lib/utils/chartTheme.js` | 图表颜色读取与解析工具 |
| `web/dashboard/src/lib/utils/storage.js` | 安全的 localStorage 封装 |
| `web/dashboard/src/styles/dashboard.css` | 入口样式表，`@import` 顺序驱动 |
| `web/dashboard/src/main.js` | 入口 JS，导入 `dashboard.css` |
| `web/dashboard/src/App.svelte` | 根组件，调用 `themeStore.init()` |