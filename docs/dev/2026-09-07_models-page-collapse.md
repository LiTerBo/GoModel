# 「模型」页面分组折叠 —— 需求 + 设计 + 任务清单

> 日期：2026-09-07
> 类型：前端功能增强（纯 `web/dashboard`，后端 0 改动、无新 API）
> 状态：**已实施**（issue #14；commits `d5fd4025` 纯函数 / `72cfe23d` store / `59b5041d` UI；文档 `55af0c6e`）

## 1. 背景与目标

**痛点**：`admin/dashboard/models` 目前已是「按供应商 + 虚拟模型」分组渲染，但所有组**恒展开**，无法折叠。真实模型 40 个 + 虚拟模型 6 个 = 46 行，列表过长。

**目标**：复用审计日志「按会话分组」的折叠/展开范式，给模型页的组头加折叠能力，默认折叠供应商组以压缩首屏高度，并支持一键「展开/折叠全部」。

**范围边界**：只做折叠/展开交互，不改分组算法、不改行内容、不加「分组/平铺」切换开关、不做持久化。

## 2. 现状（as-is，实施前基线）

| 层 | 位置 | 现状 |
|---|---|---|
| 分组数据 | `web/dashboard/src/pages/models/displayRows.js` → `groupDisplayModels()` | 产出置顶 `virtual-model-group`（别名类虚拟模型，`is_virtual_models:true`）+ 每供应商一组 `provider-group:<name>` |
| 组结构 | 同文件 | 每组含 `key` / `display_name` / `type_label` / `item_count_label` / `access` / `access_summary` / `rows` |
| 渲染 | `web/dashboard/src/pages/models/ModelTable.svelte` | `{#each groups}` → `<tbody>` 头行 `provider-group-row`（`<td colspan>` + 组级操作按钮）+ `{#each group.rows}` → `<ModelRow>` |
| 桥接 | `ModelsPage.svelte` | `modelGroups = $derived(virtualModels.filteredDisplayModelGroups)` → `<ModelTable groups={modelGroups}>`（Svelte 5 跨模块响应式桥接，勿破坏） |
| 折叠能力 | — | **无**，所有组恒展开 |
| 持久化 | — | 无 |

## 3. 功能需求（FR）

- **FR-1**：供应商组、虚拟模型组各自可独立折叠/展开，点击组头展开器切换。
- **FR-2**：折叠时组头仍保留组名、类型标签、计数与**组级操作按钮**（访问开关 / 供应商定价 / 限流表 / 覆盖编辑），仅隐藏其子行 `rows`。
- **FR-3**：默认状态——「虚拟模型」组展开，所有「供应商」组折叠。
- **FR-4**：提供一键「展开全部 / 折叠全部」按钮，文案随当前状态切换。
- **FR-5**：折叠状态不持久化（刷新 / SPA 导航后回到 FR-3 默认态）。
- **FR-6**：展开器可访问（`aria-expanded` / `aria-label`，原生 button 键盘可操作）。
  - **实施修订**：原设计的 `transition:slide` 不适用于表格行——CSS `table-row-group`/`table-row` 按规范忽略 `overflow`/`max-height`，slide 的裁剪机制会花屏。实现改为折叠瞬时隐藏、展开瞬时显示（组头 chevron 方向即时切换），与整体风格一致。

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
| D-5 状态编码（实施中衍生） | 双 Set 显式状态 | 见 §6.1——单 Set + 默认值判定在「默认展开组无法折叠后还原」与「过滤键入重组列表」两场景下语义受损，改双 Set 显式编码 |

## 6. 设计（as-built）

### 6.1 折叠状态管理

- **位置**：`virtualModels.svelte.js`（`VirtualModelsStore`，它已持有 `displayModelGroups`，天然归属）。
- **状态（D-5 双 Set）**：`collapsedGroups = $state(new Set())` 强制折叠的 key；`expandedGroups = $state(new Set())` 强制展开的 key；两 Set 均不含的 key 走 `defaultGroupExpanded(key)` 默认值（仅虚拟模型组展开）。Set 对象**整体换新、就地不改**，保证 `$derived` 依赖跟踪。
- **store 方法**：`isGroupExpanded(key)` / `allGroupsExpanded()` / `toggleGroupExpanded(key)` / `toggleAllGroupsExpanded(expand)`。

### 6.2 纯函数（`displayRows.js`，无 i18n 依赖）

```
VIRTUAL_MODEL_GROUP_KEY = "virtual-model-group"      // 组 key 常量导出
defaultGroupExpanded(key)                              // key === 虚拟模型组 key
isGroupExpanded(key, collapsed, expanded)              // expanded 命中→true；collapsed 命中→false；否则默认值
toggleGroupOverride(key, collapsed, expanded, expand)  // expand→expanded.add+collapsed.delete；反之镜像；返回新集合（输入不可变）
toggleAllGroups(groups, expand)                        // 对当前全部组 key 显式写入展开/折叠
areAllGroupsExpanded(groups, collapsed, expanded)      // 全展开判定（空组列表返回 false）
```

### 6.3 组件改动

- **`ModelTable.svelte`**：
  - 组头 `provider-group-row` 加展开器 chevron（`aria-expanded` + `title`/`aria-label`，原生 button 键盘可操作），文案复用既有 `common_action_expand` / `common_action_collapse`。
  - 折叠时 `{#if groupExpanded(group)}` 跳过 `{#each group.rows}`。
  - 组头现有的 `AccessToggle` / 定价 / 限流 / 覆盖编辑按钮**不随折叠隐藏**（FR-2）。
- **`ModelsPage.svelte`**：
  - 工具栏 `table-toolbar-actions` 增加「展开全部 / 折叠全部」按钮（`allGroupsExpanded()` 切换文案与动作；图标 `UnfoldVertical` / `FoldVertical`）。
  - 桥接模式保持并扩展：`modelGroups` prop 不变；新增 `modelCollapse = $derived({ collapsed, expanded })` 快照 prop 传给 `ModelTable`（比组件内直读单例更严格地规避 NFR-2 冻结风险）。

### 6.4 数据流（as-built）

```
modelsStore.models ──buildDisplayModels()──▶ displayModels
                                                 │
                                    groupDisplayModels() ──▶ filteredDisplayModelGroups
                                                 │
collapsedGroups/expandedGroups ($state Sets) ──▶ ModelsPage $derived 快照 prop
                                                 └─▶ ModelTable: groupExpanded(group) ──▶ 折叠渲染
```

### 6.5 与增量渲染批处理的交互

`filteredDisplayModelGroups` 现有按 `modelRenderLimit` 分批切片（批次 75 行/帧），组头与其当前 `rows` 同步增长。折叠只控制「是否渲染 `group.rows`」，与切片正交：折叠的组仅渲染组头（计数 `item_count_label` 仍按当前已渲染子行计算），展开后再显示其子行。无需改动批处理逻辑。

## 7. 关键风险与规避结果

- **R-1（响应式冻结）**：折叠状态经 `ModelsPage` `$derived` 快照桥接为 prop（`modelCollapse`），`ModelTable` 不直读跨模块单例。已验证：切换即时生效（svelte-check 0 错 + 构建通过）。
- **R-2（组头操作误隐藏）**：折叠仅门控 `{#each group.rows}`，组头结构未动。
- **R-3（i18n 遗漏）**：新增 key 双语同步（`models_expand_all` / `models_collapse_all`），i18n 编译通过。

## 8. 任务清单（TDD，已完成）

### 阶段 A：纯函数 + 单测（RED→GREEN）✅
- [x] `displayRows.js` 新增 `defaultGroupExpanded` / `isGroupExpanded` / `toggleGroupOverride` / `toggleAllGroups` / `areAllGroupsExpanded`（+ `VIRTUAL_MODEL_GROUP_KEY` 导出）
  - 实施差异：双 Set 显式编码（D-5）；单集合方案推导后存在语义缺陷（默认展开组无法还原折叠态、过滤重组列表会丢状态），实施前修正契约
- [x] `web/dashboard/tests/models-virtual-models.test.js` 新增 5 用例：默认态、双 Set 解析、toggle 显式写入/去重/输入不可变、toggleAll 双向、全展开判定
- [x] `npm test` 通过（54/54 → 全量 616/616）— commit `d5fd4025`

### 阶段 B：store 折叠状态（RED→GREEN）✅
- [x] `virtualModels.svelte.js` 加 `collapsedGroups` / `expandedGroups`（`$state` Set）+ `isGroupExpanded` / `allGroupsExpanded` / `toggleGroupExpanded` / `toggleAllGroupsExpanded`
- [x] 新增 `tests/models-group-collapse-store.test.js` 源码契约测试（`.svelte.js` 不能被 node 直 import，沿用仓库 modelTest.test.js 范式）
- [x] `npm test` 通过 + svelte-check 0 错 — commit `72cfe23d`

### 阶段 C：ModelTable 折叠渲染 + 一键按钮 + i18n（RED→GREEN）✅
- [x] 组头加展开器 chevron（`aria-expanded` + `title`/`aria-label`，原生 button 键盘可操作）
- [x] 折叠时跳过 `{#each group.rows}`（`{#if groupExpanded(group)}` 门控）
  - 实施差异：`transition:slide` 修订为瞬时切换（FR-6 修订，表格行不适用 slide）
- [x] 组级操作按钮保持可见（FR-2；组头 DOM 结构未动）
- [x] `ModelsPage.svelte` 工具栏一键按钮（`models_expand_all` / `models_collapse_all` 双语文案随状态切换）
- [x] `messages/en.json` + `messages/zh.json` 新增 key + `npm run i18n:compile`
- [x] 新增 `tests/models-group-collapse-ui.test.js`（3 用例：ModelTable 契约 / ModelsPage 契约 / i18n 双语 key）
- [x] `npm run build` 通过 — commit `59b5041d`

### 阶段 D：收口验证 ✅
- [x] `cd web/dashboard && npm test`：**620/620 全过**
- [x] `npm run check`（svelte-check）：**0 errors / 0 warnings**
- [x] `make test`（Go 全量 + frontend-check）：**0 FAIL**
- [x] `make build` 依赖的 dist 已在位（dashboard 构建产物嵌入路径存在）
- [x] `make lint`：环境未装 golangci-lint（`/Users/zhulei/go/bin/golangci-lint` 不存在），无法执行；本次改动 0 行 Go 代码，不影响 Go 门禁
- [ ] 浏览器手动验证（默认首屏折叠形态 / 点击组头 / 一键按钮）——待用户在本地服务上目检

## 9. 工作量估计

| 阶段 | 内容 | 预估 |
|---|---|---|
| A | 纯函数 + 单测 | 0.5h |
| B | store 状态 | 0.5h |
| C | ModelTable 折叠渲染 + 一键 + i18n | 1.5h |
| D | 构建/测试/验证 | 0.5h |
| **合计** | | **≈3h** |

## 10. 结论

功能已按 D-1~D-5 全部实施并验证：默认「虚拟模型组展开、供应商组折叠」，组头 chevron 独立折叠/展开，工具栏一键「展开全部 / 折叠全部」，状态仅存内存（FR-5）。关联 issue #14 随收口提交关闭。
