# Issue #8–#12 变更影响分析（git diff + codebase-memory 图谱）

> 分析日期：2026-09-07
> 分析对象：GoModel @ `155b2b79`（v1.0.0 里程碑）
> 分析方法：`git diff <base>..<head>` 逐 issue 提取变更 + codebase-memory 知识图谱影响度分析
> 背景：模型页渲染回归（仅显示虚拟模型组、真实模型组缺失）在 issue #8–#12 实现期间引入

## 0. Issue 与提交对应关系

| Issue | 内容 | 提交（commits） | 变更文件数 |
|---|---|---|---|
| #8 | W1 复杂度感知动态路由（阶段 A–F） | `31fd03df`, `ddf2cc6d`, `0addb25c` | ~30 |
| #9 | W2a 主动探测（阶段 G–H） | `d78a027e`, `7b9e3c79` | ~20 |
| #10 | W2b 被动观测（阶段 I） | `88e515db` | ~15 |
| #11 | W3 运行时告警 + 视觉区分（阶段 J–K） | `26827919`, `3fbb0eb8` | ~10 |
| #12 | L 收口（config 手册/文档/索引） | `155b2b79` | ~8 |

## 1. Issue #8 —— W1 复杂度感知动态路由

### 1.1 变更摘要（git diff）

**新增后端扩展包**（core 之外，符合「扩展不进 core」设计）：

| 文件 | 作用 |
|---|---|
| `internal/extensions/complexity/estimator.go` | 8 维复杂度估计器（长度/代码/多步/tool/轮次） |
| `internal/extensions/complexity/engine.go` | 决策引擎（4 档分类） |
| `internal/extensions/complexity/selector.go` | `RouteSelector` 实现（选 tier 目标） |
| `internal/extensions/complexity/config.go` | 阈值/权重配置 |
| `internal/extensions/complexity/install.go` | 注册扩展 |

**核心接口扩展**：

| 文件 | 变更 |
|---|---|
| `ext/route.go` | `RouteRequest` 增加 `Content`/`Features` 字段；`Select` 增加内容感知 |
| `ext/route_content.go` | 新增：请求内容 → 路由特征提取 |
| `internal/gateway/inference_prepare.go` | 把请求内容传进 selector |

### 1.2 影响度分析（codebase-memory）

**受影响符号**：`RouteCandidate` / `RouteRequest` / `RouteTarget` / `Qualified` / `RouteOutcome`（ext/route.go，均被 selector 与 gateway 引用）
**依赖流向**：

```
inference_prepare.go ──> ResolveRequestModelWithAuthorizer ──> ResolveExecutionSelector
      │                                                            │
      └── route_content.go（Content/Features 提取）<──────────────┘
                                                                   ▼
                                                          complexity/selector.go
                                                                   ▼
                                                          complexity/estimator.go
```

**影响面**：`internal/gateway`（路由解析链）、`ext`（接口契约）、`internal/admin`（运行时刷新暴露扩展状态）。**未触及** `web/dashboard/src/pages/models/*` 渲染链路。

---

## 2. Issue #9 —— W2a 主动探测（模型测试）

### 2.1 变更摘要（git diff）

| 文件 | 作用 |
|---|---|
| `internal/admin/handler_modeltest.go` | `POST /admin/models/test`、`GET /admin/models/test-results` |
| `internal/admin/handler_modeltest_support.go` | 探测结果聚合 |
| `internal/modeldata/modeltest/modeltest.go` | 离线探测执行器（能力判定） |
| `internal/app/modeltest_adapter.go` | 应用层适配 |

**新增 admin 路由**（`internal/admin/routes.go`）：

```
POST /admin/models/test          触发离线探测
GET  /admin/models/test-results  读取最近结果
```

### 2.2 影响度分析

**受影响符号**：`modeltest` 包（探测器）、`modeldata`（能力元数据）、`admin.Handler`（路由注册）。
**依赖流向**：

```
ModelTestStore (前端) ──> POST /admin/models/test ──> handler_modeltest ──> modeldata/modeltest
                                                              │
                                                              └──> capability_metadata.go（能力字典）
```

**影响面**：admin API 层新增端点 + modeldata 能力字典。**未触及** models 页面渲染链路。

---

## 3. Issue #10 —— W2b 被动观测

### 3.1 变更摘要（git diff）

| 文件 | 作用 |
|---|---|
| `internal/auditlog/capability_signals.go` | 审计日志 → 能力信号（tool_calls/格式错误聚合） |
| `internal/auditlog/stream_wrapper.go` | 流式响应包装（观测） |
| `internal/admin/handler_observed_suggestions.go` | `GET /admin/models/observed-suggestions` |
| `internal/modeldata/capability_errors.go` | 能力错误聚合 |

### 3.2 影响度分析

**受影响符号**：`auditlog`（观测源）、`handler_observed_suggestions`（admin 端点）、`capability_errors`（模型数据层）。
**依赖流向**：

```
auditlog（流式包装）──> capability_signals ──> modeldata/capability_errors
                                        │
                                        └──> admin: GET /admin/models/observed-suggestions
```

**影响面**：审计观测链路 + admin 端点。**未触及** models 页面渲染链路。

---

## 4. Issue #11 —— W3 运行时告警 + 视觉区分（★ 前端回归重点）

### 4.1 变更摘要（git diff）

| 文件 | 变更 |
|---|---|
| `internal/gateway/capability_error.go` | 运行时能力错误检测（注入响应） |
| `web/dashboard/src/pages/models/capabilityErrors.svelte.js` | **新增**：capability-errors store（`getJSON("/admin/models/capability-errors")`） |
| `web/dashboard/src/pages/models/modelTest.svelte.js` | **新增**：模型测试 store（`sendJSON("/admin/models/test")`） |
| `web/dashboard/src/pages/models/ModelsPage.svelte` | **+2 行**：`import { capabilityErrors }` + `onMount` 中 `void capabilityErrors.refresh()` |
| `web/dashboard/src/pages/models/ModelRow.svelte` | **+52 行**（J +40、K +12）：能力徽标（capability sources）、WARN 图标（含 runtimeCapError 明细）、测试触发按钮 |
| `web/dashboard/messages/en.json` / `zh.json` | i18n：8 个 `models_cap_*` 新键 + en.json 格式化重构 |
| `web/dashboard/src/styles/forms.css` | +9：`.model-warn-badge` 警告徽标样式 |
| `internal/modeldata/capability_errors.go` | 后端能力错误聚合 |

> **修正说明**：初版文档 4.1/4.2 曾写「ModelTable.svelte 列定义扩展」——**经 git diff 核实为误记**。`git diff --stat 26827919^..3fbb0eb8 -- web/dashboard/` 的 9 个变更文件**不含 ModelTable.svelte**；ModelTable 最近真实提交为 `ebc0aca7`（#865，早于 issue #8）。同理 4.2 符号表中 `mutateVirtualModelRow`/`toggleModelRow` 为搜索匹配到的同名方法（含 "ModelRow" 子串），非本次变更。

**关键变更**（ModelsPage.svelte，stage K）：

```diff
+  import { capabilityErrors } from "./capabilityErrors.svelte.js";
   ...
   virtualModels.fetchVirtualModels();
   pricingOverrides.fetchModelPricingOverrides();
   rateLimits.fetchRateLimitsPage();
+  void capabilityErrors.refresh();   // ← 新增异步调用
```

### 4.2 影响度分析（codebase-memory）

**受影响符号（前端渲染链路）**：

| 符号 | 文件 | 影响 |
|---|---|---|
| `ModelsPage`（Module） | `ModelsPage.svelte` | **+import capabilityErrors +onMount 异步刷新** |
| `ModelTable`（Module） | `ModelTable.svelte` | 列定义扩展（消费 modelTest 状态） |
| `ModelRow`（Module） | `ModelRow.svelte` | 徽标/警告/测试按钮（+40 行） |
| `VirtualModelsStore.mutateVirtualModelRow` | `virtualModels.svelte.js` | 行数据变更入口 |
| `VirtualModelsStore.toggleModelRow` | `virtualModels.svelte.js` | 行启用切换 |

**依赖链（前端）**：

```
ModelsPage (onMount)
   ├─ virtualModels.fetchVirtualModels()      → GET /admin/virtual-models
   ├─ pricingOverrides.fetchModelPricingOverrides() → GET /admin/model-pricing-overrides
   ├─ rateLimits.fetchRateLimitsPage()         → GET /admin/rate-limits
   └─ capabilityErrors.refresh()  [stage K 新增] → GET /admin/models/capability-errors
        │
        └─ ModelTable (消费 capabilityErrors / modelTest 状态)
              └─ ModelRow (徽标/WARN/测试按钮)
```

**影响面**：前端 models 页面模块间依赖新增 2 个 store 模块（capabilityErrors、modelTest）。**这是 5 个 issue 中唯一直接修改 models 页面前端渲染代码的提交**。

---

## 5. Issue #12 —— L 收口

### 5.1 变更摘要（git diff）

| 文件 | 变更 |
|---|---|
| `docs/capability-routing/工作任务清单.md` | checkbox 登记 |
| `docs/config-manual.md` | 新增：配置手册 |
| `docs/capability-routing/需求说明.md` / `架构设计.md` | 文档收口 |
| `web/dashboard/src/lib/api/client.js` | **+2 行：observed-suggestions 端点封装**（L-1） |

### 5.2 影响度分析

**影响面**：纯文档 + 1 个 API 客户端封装。**未触及** models 页面渲染逻辑。

---

## 6. 结论：回归根因定位

### 6.1 变更面收敛

5 个 issue 中，**只有 issue #11（stage J/K）修改了 models 页面前端渲染代码**：

| Issue | 触及 models 页面渲染链路？ |
|---|---|
| #8 | ❌（纯后端 + ext 接口） |
| #9 | ❌（admin 端点 + modeldata） |
| #10 | ❌（auditlog 观测 + admin 端点） |
| **#11** | **✅ 部分（ModelsPage +2、ModelRow +52、新增 2 store）** |
| #12 | ❌（文档 + client 封装） |

**注意**：`ModelTable.svelte`、`virtualModels.svelte.js`、`models.svelte.js`、`displayRows.js`、`client.js` 在 `9b699da8..155b2b79` 期间**均未被修改**（git diff --stat 验证，9 个变更文件仅含 ModelRow/ModelsPage/2 新 store/i18n/CSS/测试）。

### 6.2 根因（第二轮编译产物分析，已更新）

**渲染冻结的机制已确认**（非模块求值顺序）：

- **数据层 100% 正常**：浏览器实测 `window.__vmStore.filteredDisplayModelGroups.length = 4`（4 组全量已重算），点分类后 `= 3` 也正确更新。`$derived` 链响应式无损。
- **模板层冻结**：ModelTable.svelte 的 `{#each virtualModels.filteredDisplayModelGroups}` 编译为 `G(L(s),17,()=>W6.filteredDisplayModelGroups,...)`——数组 getter 在 **untrack 上下文**求值，`z(this.#p)` 不注册依赖 → 一次性渲染。
- 对比 ModelsPage 标题编译为 `R(e=>U(n,e),[()=>py({count:...W6.displayModels.length...})])`——依赖 getter 在 **effect 上下文**求值 → 追踪 → 标题正确更新到 46。
- **根因**：Svelte 5（^5.35.0）中跨模块导出对象的属性在模板 `{#each}` 表达式里不建立响应式依赖；`virtualModels`/`modelsStore` 是模块级单例（非官方 store contract），模板内属性访问非响应式引用。
- **首帧时序**：`{#if virtualModels.displayModels.length > 0}` 首帧即真（虚拟模型 6 个立即可用）→ ModelTable 挂载（真实模型 0）→ 40 个真实模型到达后条件仍为真 → 块不重建 → 表格冻结在首帧。

### 6.3 与 issue #8–#12 的关系（诚实结论）

- **结构性根因不是 issue #8–#12 引入**：`9b699da8` 基线的 ModelsPage/ModelTable 模板结构与 HEAD 完全一致（git show 验证）。
- stage J/K 的 ModelsPage +2 行（capabilityErrors import + refresh）与 ModelRow +52 行在**代码层面不触碰渲染冻结路径**。
- 用户感知「今天实现新功能后出现」的可能原因：v1.0.0 里程碑本身包含 issue #8–#12（今天打 tag），功能上线后模型页被首次深度使用；或 stage J/K 新增的 onMount 第 4 个异步请求改变了时序窗口。**需基线实测才能定论**。

### 6.4 验证方案（下一步，本次未执行——用户要求先出报告）

1. **基线对比**：`git worktree` 检出 `9b699da8` 构建前端，浏览器实测模型页 → 判定 issue #8–#12 是否真为引入点。
2. **修复候选（未实施）**：ModelTable 内 `const groups = $derived(virtualModels.filteredDisplayModelGroups)` 组件级桥接，或在 ModelsPage 将 groups 作为 prop 传入——均不改渲染结构、不改 store 派生链。
3. 修复后回归：`make test`（611+ dashboard 测试）+ `make build` + 浏览器实测。

---

## 7. 结案报告（2026-09-07 修复实施与端到端验证）

### 7.1 实际根因（推翻 6.2 的「Svelte 5 响应性冻结」假设）

浏览器端子捕获到决定性证据：`Uncaught ReferenceError: capabilityErrors is not defined`（编译产物 `index-C8bqpS1o.js:20:20362`）。逐行核对源码：

- `ModelRow.svelte` 第 55 行 `runtimeCapError = $derived(row.is_alias ? null : capabilityErrors.state(...))` 引用了 `capabilityErrors`，但 **import 列表缺失该导入**（stage K 提交 `3fbb0eb8` 引入，构建期不报错——Rolldown 对 Svelte 组件内未声明标识符未拦截，611 个 node:test 也不编译 .svelte 组件）。
- 真实模型行（`is_alias=false`）渲染即抛错 → ModelTable 的 `{#each}` 迭代在第一个真实模型行处中止 → **仅虚拟模型组（别名行三元短路不触发）显示，真实模型组缺失**——与用户报障现象完全吻合。
- 6.2 的「模板层冻结」分析对象是**旧 bundle**（含重复声明编译失败前最后一次成功构建的产物），时序窗口推测不成立；数据层结论（派生链正常）仍有效。

### 7.2 修复内容（本次已实施）

| 文件 | 修复 |
|---|---|
| `ModelRow.svelte` | 补 `import { capabilityErrors } from "./capabilityErrors.svelte.js"`（根因修复） |
| `ModelsPage.svelte` | 删除 `modelGroups` 重复声明（`let=$state` 与 `const=$derived` 同名导致编译失败，手动构建从未成功）；保留 `const modelGroups = $derived(...)` 桥接并经 prop 传入 ModelTable |
| `virtualModels.svelte.js` | 移除全部 TEMP TRACE 诊断插桩（`globalThis.__tr` × 3、trace derived、`#tv/#tg/#tf/#tp` 计数器）及其遮蔽的重复 `generation` 声明 |

配套防御性桥接（与根因修复同批，均为组件级 `$derived` 中转跨模块单例读，模板经本地常量消费）：ModelRow/ModelTable/AccessToggle/ModelGlobalActions 对 `virtualModelsAvailable`/`rowDeletingKey`/`modelTestState`/`rateLimitsEnabled` 等的读法统一。

### 7.3 端到端验证（2026-09-07 16:30，HEAD 未提交工作区）

- `npm run build` ✓（2.09s）+ `npm test` **611/611 全绿**
- `make build` ✓ → 重启 `./bin/gomodel` → 后端 5 端点全 200（39 模型/3 providers）
- headless Chrome 实测 `http://localhost:8080/admin/dashboard/models`：
  - **4 个分组全渲染**：虚拟模型(6 别名) + deepseek(3) + oMLX(20) + openai-compatible(16) = 49 行
  - 分类 tab 过滤生效（嵌入 tab → 仅 oMLX 组 4 模型 + 组头）
  - AccessToggle 响应式正常（deepseek 组点击 → 「已禁用」实时更新，恢复「已启用」）
  - **console 零错误**（修复前:2 条 ReferenceError）

### 7.4 遗留事项

- 修复未提交（6 文件,与本报告同批）；commit message 建议 `fix(dashboard): models page real-model rows not rendering`
- 两个 git stash（`stash@{0}` 同修复早期 WIP、`stash@{1}` 更早期）可评估后清理
- 4 份分析文档（含本文件）待提交或归档；`etc/`、`.DS_Store` 待加 .gitignore

---

## 附录 A：影响度分析工具说明

- 工具：codebase-memory MCP（`detect_changes` 全量变更检测 + `search_code` 符号定位）
- 索引状态：GoModel 项目已索引（auto_index=false，本次分析前手动 `index_repository` 重建）
- 变更基线：`9b699da8`（issue #8 之前）→ `155b2b79`（HEAD）
- 变更文件总数：74；受影响符号：见 `impacted_symbols`（存于会话 spillover，171KB）
