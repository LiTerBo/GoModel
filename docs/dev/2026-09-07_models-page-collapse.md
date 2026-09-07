# 「模型」页面分组折叠 —— 需求 + 设计 + 任务清单

> 日期：2026-09-07
> 类型：前端功能增强（纯 `web/dashboard`，后端 0 改动、无新 API）
> 状态：待用户审核（决策 Q1–Q4 已裁决）

## 1. 背景与目标

**痛点**：`admin/dashboard/models` 目前已是「按供应商 + 虚拟模型」分组渲染，但所有组**恒展开**，无法折叠。真实模型 40 个 + 虚拟模型 6 个 = 46 行，列表过长。

**目标**：复用审计日志「按会话分组」的折叠/展开范式，给模型页的组头加折叠能力，默认折叠供应商组以压缩首屏高度，并支持一键「展开/折叠全部」。

**范围边界**：只做折叠/展开交互，不改分组算法、不改行内容、不加「分组/平铺」切换开关、不做持久化。

## 2. 现状（as-is）

| 层 | 位置 | 现状 |
|---|---|---|
| 分组数据 | `web/dashboard/src/pages/models/displayRows.js` → `groupDisplayModels()` (L293) | 产出置顶 `virtual-model-group`（别名类虚拟模型，`is_virtual_models:true`）+ 每供应商一组 `provider-group:<name>` |
| 组结构 | 同文件 | 每组含 `key` / `display_name` / `type_label` / `item_count_label` / `access` / `access_summary` / `rows` |
| 渲染 | `web/dashboard/src/pages/models/ModelTable.svelte` (L55-113) | `{#each groups}` → `<tbody>` 头行 `provider-group-row`（`<td colspan>` + 组级操作按钮）+ `{#each group.rows}` → `<ModelRow>` |
| 桥接 | `ModelsPage.svelte` (L57) | `modelGroups = $derived(virtualModels.filteredDisplayModelGroups)` → `<ModelTable groups={modelGroups}>`（Svelte 5 跨模块响应式桥接，勿破坏） |
| 折叠能力 | — | **无**，所有组恒展开 |
| 持久化 | — | 无 |

## 3. 功能需求（FR）

- **FR-1**：供应商组、虚拟模型组各自可独立折叠/展开，点击组头切换。
- **FR-2**：折叠时组头仍保留组名、类型标签、计数与**组级操作按钮**（访问开关 / 供应商定价 / 限流表 / 覆盖编辑），仅隐藏其子行 `rows`。
- **FR-3**：默认状态——「虚拟模型」组展开，所有「供应商」组折叠。
- **FR-4**：提供一键「展开全部 / 折叠全部」按钮。
- **FR-5**：折叠状态不持久化（刷新 / SPA 导航后回到 FR-3 默认态）。
- **FR-6**：折叠/展开带 `transition:slide` 过渡；展开器可访问（`aria-expanded` / `aria-label`，键盘可操作）。

## 4. 非功能需求（NFR）

- **NFR-1**：不改变 `groupDisplayModels` 的分组算法与行内容，不影响组级操作与行级操作。
- **NFR-2**：遵循 Svelte 5 响应式规范——折叠状态用 `$state`，模板读取经组件级 `$derived`/prop 桥接，避免重蹈「跨模块单例 store 模板非响应式」冻结。
- **NFR-3**：i18n 完整（`messages/en.json` + `messages/zh.json` 同步，命名沿用 `models_*`）。
- **NFR-4**：纯逻辑（折叠 Set 运算、默认态判定、全展开判定）可被 `node:test` 直接单测。

## 5. 决策记录（D）

| 决策点 | 裁决 | 说明 |
|---|---|---|
| D-1 分组含义 | 现状不变 | 只给现有「虚拟模型组 + 供应商组」加折叠，不按虚拟模型单独成组、不做反向映射 |
| D-2 分组/平铺开关 | 不加 | 恒分组，只加折叠 |
| D-3 默认折叠策略 | 虚拟模型组展开、供应商组折叠 + 一键全展开/折叠 | 首屏只显示组头，解决列表过长 |
| D-4 持久化 | 不做 | 折叠状态仅内存，刷新回默认 |

## 6. 设计

### 6.1 折叠状态管理

- **位置**：`virtualModels.svelte.js`（`VirtualModelsStore`，它已持有 `displayModelGroups`，天然归属）。
- **状态**：`expandedGroups = $state(new Set())`，存「展开的组 key」。
- **默认态**：`isGroupExpanded(key)` 在未显式切换时返回「是否虚拟模型组」——即 `key === "virtual-model-group"`（或 `group.is_virtual_models === true`）为展开，其余折叠。不依赖预填充 Set，避免组动态增减时状态漂移。
- **切换 API**：`toggleGroup(key)`、`expandAll(groups)`、`collapseAll()`、`areAllExpanded(groups)`。

### 6.2 纯函数（`displayRows.js`）

参照 `audit-logic.js` 的 `toggleExpandedThread` / `pruneThreadMap`，新增无 i18n 依赖的纯函数：

```
defaultGroupExpanded(group)      // group.is_virtual_models === true
toggleGroupExpanded(set, key)    // 返回新 Set（不可变）
areAllGroupsExpanded(set, groups)
toggleAllGroups(set, groups)     // 全展开→全折叠，否则→全展开
```

### 6.3 组件改动

- **`ModelTable.svelte`**：
  - 组头 `provider-group-row` 加展开器（chevron 图标，`aria-expanded` + `aria-label`，`onclick` 调 `virtualModels.toggleGroup(group.key)`）。
  - 折叠时跳过 `{#each group.rows}`；展开时子行包 `transition:slide`。
  - 组头现有的 `AccessToggle` / 定价 / 限流 / 覆盖编辑按钮**不随折叠隐藏**（FR-2）。
- **`ModelsPage.svelte`**：
  - 工具栏 `table-toolbar-actions` 增加「展开全部 / 折叠全部」按钮（根据 `areAllExpanded` 切换文案与动作）。
  - 桥接模式保持：`modelGroups` prop 不变，折叠状态经组件内 `$derived` 读取。

### 6.4 数据流（目标）

```
modelsStore.models ──buildDisplayModels()──▶ displayModels
                                                  │
                                     groupDisplayModels() ──▶ displayModelGroups
                                                  │
                expandedGroups($state Set) ──▶ isGroupExpanded(key) ──▶ ModelTable(折叠渲染)
```

### 6.5 与增量渲染批处理的交互

`filteredDisplayModelGroups` 现有按 `modelRenderLimit` 分批切片（批次 75 行/帧），组头与其当前 `rows` 同步增长。折叠只控制「是否渲染 `group.rows`」，与切片正交：折叠的组仅渲染组头（计数 `item_count_label` 仍按当前已渲染子行计算），展开后再显示其子行。无需改动批处理逻辑。

## 7. 关键风险

- **R-1（响应式冻结）**：折叠状态若直接以模块单例属性在模板读，会重演「渲染冻结」。规避：`expandedGroups` 用 `$state`，`ModelTable` 内 `const isExpanded = $derived(virtualModels.isGroupExpanded(group.key))` 或经 prop 桥接（现有 `groups` prop 模式已验证）。
- **R-2（组头操作误隐藏）**：折叠时务必只隐藏 `group.rows`，保留组级操作按钮，否则供应商级访问/定价/限流入口丢失。
- **R-3（i18n 遗漏）**：新增 key 必须 en/zh 双向同步并跑 `npm run i18n:compile`，否则构建/运行期缺 key 报错。

## 8. 任务清单（TDD，按阶段）

### 阶段 A：纯函数 + 单测（RED→GREEN）
- [ ] `displayRows.js` 新增 `defaultGroupExpanded` / `toggleGroupExpanded` / `areAllGroupsExpanded` / `toggleAllGroups`
- [ ] `web/dashboard/tests/models-virtual-models.test.js` 新增用例：默认态（虚拟模型组 true、供应商组 false）、toggle 不可变返回、全展开判定、toggleAll 双向
- [ ] `cd web/dashboard && npm test` 通过

### 阶段 B：store 折叠状态
- [ ] `virtualModels.svelte.js` 加 `expandedGroups`（`$state Set`）+ `isGroupExpanded` / `toggleGroup` / `expandAll` / `collapseAll` / `areAllExpanded`
- [ ] 默认态判定落到 `defaultGroupExpanded`（阶段 A 纯函数）
- [ ] `npm test` 通过

### 阶段 C：ModelTable 折叠渲染
- [ ] 组头加展开器 chevron（`aria-expanded` + `aria-label` + 键盘可操作）
- [ ] 折叠时跳过 `{#each group.rows}`，展开时 `transition:slide`
- [ ] 组级操作按钮保持可见（FR-2 回归检查）
- [ ] `npm run build` 通过

### 阶段 D：一键「展开全部 / 折叠全部」
- [ ] `ModelsPage.svelte` 工具栏加按钮，文案随 `areAllExpanded` 切换
- [ ] `npm run build` 通过

### 阶段 E：i18n
- [ ] `messages/en.json` + `messages/zh.json` 新增 key（`models_group_expand` / `models_group_collapse` / `models_expand_all` / `models_collapse_all` 等，命名与现有 `models_*` 一致）
- [ ] `npm run i18n:compile`

### 阶段 F：收口验证
- [ ] `cd web/dashboard && npm test`（全量，含新增用例）
- [ ] 根目录 `make lint` / `make build`
- [ ] 手动/curl 验证：默认首屏仅虚拟模型组展开、供应商组折叠；点击组头折叠/展开；一键全展开/折叠

## 9. 工作量估计

| 阶段 | 内容 | 预估 |
|---|---|---|
| A | 纯函数 + 单测 | 0.5h |
| B | store 状态 | 0.5h |
| C | ModelTable 折叠渲染 | 1h |
| D | 一键按钮 | 0.5h |
| E | i18n | 0.5h |
| F | 构建/测试/验证 | 0.5h |
| **合计** | | **≈3.5h** |

## 10. 待确认

- 阶段划分与默认折叠策略（FR-3）是否认可；认可后请回复「继续」，我将从阶段 A 开始按 TDD 逐项推进。
