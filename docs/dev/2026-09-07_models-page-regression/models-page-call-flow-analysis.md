# 「模型」页面调用逻辑分析 + codebase-memory 影响度分析

> 分析日期：2026-09-07（会话 20260906_234124_56c524）
> 分析对象：GoModel @ `155b2b79`（HEAD 纯净构建，已 git stash 全部未提交改动）
> 方法：源码调用链梳理 + codebase-memory 知识图谱影响度分析 + 编译产物静态分析 + 浏览器实测

## 1. 「模型」页面调用逻辑

### 1.1 页面组件结构

```
ModelsPage.svelte (路由入口, $effect 触发数据加载)
├── AuthBanner / category-tabs / FilterInput / LoadingState
└── ModelTable.svelte            (表格主体, #each 组渲染)
    ├── ModelGlobalActions.svelte (全局操作)
    └── ModelRow.svelte           (每模型行, stage J 新增徽标+测试按钮)
```

### 1.2 数据加载调用链（onMount → API）

`ModelsPage.svelte:32-39` 的 `$effect`（依赖 `router.page === "models"`）：

| 调用 | 方法 | 触发的 API |
|---|---|---|
| `virtualModels.fetchVirtualModels()` | VirtualModelsStore | `GET /admin/virtual-models` |
| `pricingOverrides.fetchModelPricingOverrides()` | PricingOverridesStore | `GET /admin/model-pricing-overrides` |
| `rateLimits.fetchRateLimitsPage()` | RateLimitsStore | `GET /admin/rate-limits/page` |
| `void capabilityErrors.refresh()` | CapabilityErrorsStore (stage K 新增) | `GET /admin/models/capability-errors` |

`modelsStore.fetchModels()` 与 `fetchCategories()` 由 **sidebar 路由守卫**（router）或共享 store 初始化触发，对应：
- `GET /admin/models`（可带 `?category=`）
- `GET /admin/models/categories`

### 1.3 渲染数据链（API 数据 → 表格行）

```
modelsStore.models (40个真实模型, $state)
  └─▶ virtualModels.displayModels = $derived(buildDisplayModels({models, aliases, virtualModelsAvailable, activeCategory}))
        = 46 行 (40 真实 + 6 虚拟)
  └─▶ virtualModels.filteredDisplayModels = $derived(filterDisplayModels(displayModels, filter))
  └─▶ virtualModels.filteredDisplayModelGroups = $derived.by(() => {
        limit = clamp(modelRenderLimit, 0, filtered.length)
        if (!filter && limit >= displayModels.length) return displayModelGroups  // 全量
        return groupDisplayModels(filtered.slice(0, limit), ...)               // 分批渲染
      })
  └─▶ ModelTable.svelte:43  {#each virtualModels.filteredDisplayModelGroups as group (group.key)}
        └─▶ {#each group.rows as row (row.key)} <ModelRow {row} {columns} />
```

**增量渲染批处理**：`ModelsPage.svelte:52-56` 的 `$effect` 读 `filteredDisplayModels.length`，调 `virtualModels.restartModelRendering(total)` 推进 `modelRenderLimit`（批次 75 行/帧），`renderBatching.js` 纯函数计算步进。

### 1.4 新增（stage J/K）调用点

- `ModelRow.svelte`：`modelTest.runTest(provider, model)` → `POST /admin/models/test`（J-2 测试按钮）
- `modelTest.fetchResults()` → `GET /admin/models/test-results`
- `capabilityErrors.refresh()` → `GET /admin/models/capability-errors`（K）

## 2. codebase-memory 影响度分析

### 2.1 变更面（git diff 9b699da8..155b2b79，issue #8–#12 全量）

- **后端**：`ext/route.go`（RouteSelector 接口扩展）、`internal/extensions/complexity/*`（复杂度引擎，新增）、`internal/gateway/*`（inference_prepare、route_summary）、`internal/admin/handler*`（capability-errors、modeltest、observed-suggestions 新增 handler）、`internal/auditlog/*`（capability_signals、stream_wrapper）、`internal/modeldata/*`（全部新增）
- **前端（6 个源码文件 + 1 个样式）**：

| 文件 | 变更 | 检查结论 |
|---|---|---|
| `messages/en.json` | +135（含 8 新 key） | 无害：大量为 JSON 数组多行格式化重构；新 key `models_cap_*` 供徽标 i18n；无 key 删除 |
| `messages/zh.json` | +8 | 无害：仅新增 `models_cap_*` 8 键 |
| `pages/models/ModelRow.svelte` | +52（J +40 / K +12） | 见 §2.4 详细分析 |
| `pages/models/ModelsPage.svelte` | +2 | 无害：`import capabilityErrors` + `void capabilityErrors.refresh()`（onMount 第 4 个异步调用） |
| `pages/models/capabilityErrors.svelte.js` | 新增 32 行 | 独立 store：`$state` byModel/loaded + `refresh()` GET capability-errors；无副作用 |
| `pages/models/modelTest.svelte.js` | 新增 41 行 | 独立 store：`$state` runs + runTest/fetchResults；无副作用 |
| `styles/forms.css` | +9 | 纯样式：`.model-warn-badge` 琥珀色 |

- **未改动**：`ModelTable.svelte`、`virtualModels.svelte.js`、`models.svelte.js`、`displayRows.js`、`client.js`

### 2.2 关键符号影响（graph）

| 符号 | 类型 | 变更 | 影响面 |
|---|---|---|---|
| `ext.RouteSelector` | 接口 | Select/OnAttemptStart/OnAttemptEnd | 所有 virtual model 路由执行路径 |
| `internal/extensions/complexity/*` | 新增 | estimator/engine/selector | 复杂度路由核心 |
| `Handler.handleCapabilityErrors` | 新增 | GET /admin/models/capability-errors | 前端 capabilityErrors store |
| `Handler.handleModeltest` | 新增 | POST /admin/models/test | 前端 modelTest store |
| `ModelRow.svelte` | 修改 | +52 行 | 模型行渲染 |
| `ModelsPage.svelte` | 修改 | +2 行 | 页面 onMount |
| `modelTest/capabilityErrors store` | 新增 | 新模块 | models 页面模块图 |

### 2.3 影响度结论

- **后端**改动影响面最大但**与页面渲染无直接因果**（新端点均为独立 handler，实测 200 可用）。
- **前端**改动**均未触碰渲染链路核心文件**（ModelTable/virtualModels/displayRows/models store 未变）。
- **渲染冻结与 stage J/K 前端改动无代码级因果**：冻结结构（`{#each virtualModels.filteredDisplayModelGroups}` 直接读跨模块 store）在基线 `9b699da8` 即已存在（git show 验证一致）。

### 2.4 ModelRow.svelte 逐行审查（stage J/K 改动最密集文件）

stage J（`26827919`）**+40 行**、stage K（`3fbb0eb8`）**+12 行**。新增代码分四组：

**① 能力来源徽标（J-1）— `ModelRow.svelte:35-40`**
```js
const capSources = $derived(row.model?.metadata?.capability_sources ?? {});
const hasTestedCaps  = $derived(Object.values(capSources).some((s) => s === "test"));
const hasObservedCaps = $derived(Object.values(capSources).some((s) => s === "observed"));
```
- 虚拟模型行 `row.model` 为 null → `?? {}` 兜底 ✓
- `capability_sources` 实测为 `map[string]string`（值 `"registry"` 等）→ `Object.values` 正常，当前无 `"test"`/`"observed"` 值 → 徽标不显示 ✓
- 潜在边界：若后端未来返回数组/字符串值，`Object.values` 仍不崩溃（字符串→字符数组）✓

**② 运行时能力错误 WARN（K）— `ModelRow.svelte:41-47`**
```js
const runtimeCapError = $derived(row.is_alias ? null : capabilityErrors.state(row.provider_name, row.model?.id));
const hasCapabilityError = $derived(Boolean(row.model?.metadata?.capability_error) || Boolean(runtimeCapError));
```
- **跨模块依赖**：ModelRow 的 `$derived` 读 `capabilityErrors.byModel`（另一 store 的 `$state`）——这是**组件内 $derived 读外部 store**，追踪正常（与表格冻结不同，这里在 runes 组件内通过方法调用读 getter）
- `row.is_alias` 短路保护虚拟行 ✓；`state()` 内部对 null provider/model 返回 null ✓
- 仅影响 WARN 徽标 title，不触碰行结构

**③ 徽标/WARN 模板渲染（J/K）— 模板段**
```svelte
{#if hasTestedCaps}...<BadgeCheck>...{/if}
{#if hasObservedCaps}...{/if}
{#if hasCapabilityError}...<AlertTriangle class="model-warn-badge">...{/if}
```
- 纯条件渲染，无 DOM 结构嵌套改动 ✓

**④ 模型测试触发按钮（J-2）— 模板段**
```svelte
{#if row.provider_name && row.model?.id}
  <TableActionButton disabled={modelTest.state(...)?.running} onclick={() => modelTest.runTest(...)}>
```
- `modelTest` 是模块级单例（`.svelte.js`），**事件回调**引用（非模板响应式表达式）→ 无追踪问题 ✓

**结论**：ModelRow 的 +52 行全部为**新增条件渲染与事件回调**，不改变行渲染主结构；唯一跨 store 依赖（capabilityErrors）走组件内 `$derived`，追踪路径正确。**与表格冻结无因果**。

## 3. 根因分析（渲染冻结机制）

### 3.1 实测现象（浏览器，HEAD 纯净构建）

```
COUNT: 46（标题，数据已到）      ← ModelsPage 模板
TBODY: 1（仅"虚拟模型 6 个别名"） ← ModelTable 模板
JS 读 window.__vmStore: fdmg=4  ← 数据层 $derived 已重算（4 组全量）
点分类后: JS fdmg=3 / 模板仍 1  ← 数据层更新，模板冻结
```

### 3.2 编译产物差异（决定性证据）

**标题**（ModelsPage，能更新到 46）：
```js
R(e=>U(n,e), [()=>py({count: W6.filteredDisplayModels.length + " / " + W6.displayModels.length})])
```
依赖 getter `()=>py({...})` 在 **effect 上下文**中求值 → `W6.displayModels.length` 触发 getter `z()` → **注册依赖** → 数据变化时重跑 ✓

**表格**（ModelTable，冻结在首帧）：
```js
G(L(s), 17, ()=>W6.filteredDisplayModelGroups, e=>e.key, (e,t)=>{...})
```
each 的数组 getter `()=>W6.filteredDisplayModelGroups` 在 **untrack 上下文**求值 → `z(this.#p)` **不注册依赖** → 一次性渲染，永不更新 ✗

### 3.3 机制结论

- Svelte 5（`^5.35.0`）中，**跨模块导出对象的属性**在模板 `{#each}` 表达式里**不建立响应式依赖**（编译期依赖收集只追踪模块内声明的 `$state`/`$derived` 标识符）。
- `virtualModels` / `modelsStore` 是模块级单例（非 Svelte 官方 store contract，无 `subscribe()`），模板里 `virtualModels.filteredDisplayModelGroups` 属**非响应式引用**。
- 数据层（`$derived` 链）完全正常——`window.__vmStore` 读到的 `fdmg=4` 就是证据。
- **首帧时序**：`ModelsPage.svelte:149` `{#if virtualModels.displayModels.length > 0}` 首帧即为真（虚拟模型 6 个立即可用）→ ModelTable 挂载（此时真实模型 0 个）→ 40 个真实模型到达后 `{#if}` 条件仍为真（46>0）→ **块不重建** → ModelTable 模板 effect 永不重跑 → 冻结在首帧（仅虚拟模型组）。

### 3.4 相关 commit 定位（git blame + git log -S 实证）

**与「Svelte 5 模板跨模块 store」直接相关的 commit 只有一个：**

| Commit | 内容 | 证据 |
|---|---|---|
| **`f8d14be33`** (#579, 2026-07-24, Jakub A. Wąsek) | **「feat(dashboard): rewrite frontend as Svelte 5 SPA」** — 前端整体重写 | `git blame`：ModelTable.svelte:43（each 直读）、:23-25（$derived）、ModelsPage.svelte:149（挂载条件）、:35（fetchVirtualModels）**全部出自此 commit** |

该 commit 一次性引入全部「冻结要素」：
1. Svelte `^5.35.0`（runes 模式，package.json 首引入，此后未变）
2. **singleton rune-class store**（提交说明原文，非 `svelte/store` contract）→ 模板跨模块属性非响应式
3. 模板直读 `virtualModels.filteredDisplayModelGroups` + 挂载条件 `{#if displayModels.length > 0 || modelsStore.filter}`
4. onMount 异步 `fetchVirtualModels()` / `fetchModels()` 并发时序
5. 内置默认 6 个虚拟模型（config.example.yaml 出厂预设，`smart` 即复杂度路由虚拟名）→ 首帧 `displayModels>0` 恒成立

**后续 commit 均未改变该模式**（git log/blame 实证）：

| Commit | 改动 | 是否触碰模板直读模式 |
|---|---|---|
| `66c0334c` (#788, 8-29) | 虚拟模型链式重构，引入 displayRows.js $derived 链 | ❌ 未动 ModelTable 模板行 |
| `ebc0aca7` (#865) | 手机端样式 | ❌ |
| `044e87de` (#682) | i18n 基础 | ❌ |
| `26827919`/`3fbb0eb8` (issue #11) | ModelRow +52 / ModelsPage +2 | ❌ 未动 each/挂载行 |

**推论**：渲染冻结的结构性根因由 `f8d14be33`（2026-07-24）引入，**早于 issue #8–#12 一个多月**；issue #8–#12 未改变该模式。9-06 才暴露的可能原因：功能上线后首次深度使用模型页，或数据规模（40 真实模型）使「冻结在首帧」更显著。**修复锚点 = `f8d14be33` 引入的模式**（组件内 `$derived` 桥接 store 值即可，与 issue #8–#12 无关）。

### 3.5 基线前端实测验证（决定性实验，2026-09-07）

**实验设计**：渲染冻结是纯前端行为——用**基线前端 dist（`9b699da8` 构建）+ HEAD 后端**（缓存已恢复，`/admin/models` 提供完整 40 模型）组合，浏览器实测模型页。避免基线后端无数据的问题干扰验证。

**执行**：`git worktree` 检出 `9b699da8` → 复用 HEAD 的 node_modules → `npm run build`（基线前端 1.8s）→ 将基线 dist 嵌入 HEAD 二进制（`make build`）→ 启动 → 浏览器实测。

**结果**：

| 指标 | 基线前端（9b699da8） | HEAD 前端（155b2b79） |
|---|---|---|
| TBODY（组数） | **1（仅「虚拟模型」）** | **1（仅「虚拟模型」）** |
| COUNT（标题计数） | **46 个模型** | **46 个模型** |
| 数据层（window store） | 正常（46 已到） | 正常（46 已到） |

**结论（实证闭合）**：基线前端**同样冻结**——与 HEAD 表现完全一致。渲染冻结的结构性根因由 `f8d14be33`（Svelte 5 SPA 重写）引入，**早在 issue #8–#12 之前即存在**（9b699da8 复现即铁证）。9-06 暴露 = v1.0.0 上线后首次深度使用模型页所致，**与 issue #8–#12 的代码变更无任何因果**。

> 注：实验前后 HEAD 环境已完整恢复（dist 备份还原、make build 重建、缓存完好、HEAD 前端 bundle `index-KdZgKwu3.js` 实测正常服务）；基线 worktree 已清理。

### 3.6 与 issue #8–#12 的关系（诚实的结论）

- **渲染冻结的结构性原因不是 issue #8–#12 引入的**：`ModelTable.svelte` 与 `ModelsPage.svelte` 的模板结构在基线 `9b699da8` 完全一致（git show 验证）。
- **stage J/K 的贡献**：ModelRow +52 行（徽标/测试按钮）与 ModelsPage +2 行（capabilityErrors）**在代码层面不触碰渲染冻结路径**；但 stage J/K 新增了两个模块（modelTest/capabilityErrors store），使 models 页面模块图变化，并新增了 onMount 的第 4 个异步请求——**不构成冻结的因果**。
- **需要基线验证才能定论**：建议下一步在 `9b699da8` 检出构建，确认基线是否同样冻结（若基线正常，则存在未发现的中间提交影响；若基线同样冻结，则问题为历史遗留，仅在今天功能上线后首次被用户注意）。当前工作区已 stash 全部未提交改动，HEAD 纯净。

## 4. 待验证项（下一步，本次未执行）

1. **基线对比**：`git worktree` 检出 `9b699da8` 构建前端，浏览器实测模型页 → 判定 issue #8–#12 是否真为引入点。
2. **修复候选（未实施）**：ModelTable 内用组件级 `$derived` 桥接 store（如 `const groups = $derived(virtualModels.filteredDisplayModelGroups)`），或在 ModelsPage 将 groups 作为 prop 传入——均不改动渲染结构。
