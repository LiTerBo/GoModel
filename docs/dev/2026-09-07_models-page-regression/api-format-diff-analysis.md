# 新旧版本 API 数据格式对比（issue #8–#12 前后）

> 分析日期：2026-09-07
> 基线：`9b699da8`（issue #8 之前，v1.0.0 里程碑前）
> 当前：`155b2b79`（HEAD，含 issue #8–#12 全部实现）
> 方法：git diff 代码层对比 + 运行时实测（HEAD 服务 + 基线版本 worktree 构建）

## 1. 结论速览

| 维度 | 结论 |
|---|---|
| `/admin/models` handler | **未变更**（routes.go 仅新增 6 个端点，原注册不动） |
| `/admin/models/categories` handler | **未变更** |
| 模型数据结构 `ModelMetadata` | **未变更**（types.go 仅新增 4 个常量） |
| `capability_sources` 字段 | **基线已有**（字段与 4 个产出文件均早于 issue #8） |
| 响应 JSON 外层结构 | **未变更**：`[{access, model, provider_name, provider_type, selector}]` |
| 新增端点 | 6 个（test / test-results / capabilities / capability-errors / observed-suggestions / observed-capabilities）——**新增不影响原端点** |

**核心结论：issue #8–#12 未改变 `/admin/models` 及 `/admin/models/categories` 的返回数据格式。** 前端模型页能正确拿到与基线完全同构的数据。

## 2. 代码层证据（git diff）

### 2.1 路由注册（`internal/admin/routes.go`）

```diff
 	g.GET("/models", h.ListModels)
 	g.GET("/models/categories", h.ListCategories)
+	g.POST("/models/test", h.RunModelTest, global)
+	g.GET("/models/test-results", h.ListModelTestResults, global)
+	g.PUT("/models/capabilities", h.ConfirmModelCapabilities, global)
+	g.GET("/models/capability-errors", h.handleCapabilityErrors, global)
+	g.GET("/models/observed-suggestions", h.ListObservedSuggestions, global)
+	g.PUT("/models/observed-capabilities", h.ConfirmObservedCapabilities, global)
```

原有 `/models`（ListModels）与 `/models/categories`（ListCategories）**注册未动**。

### 2.2 Handler 依赖注入（`internal/admin/handler.go`）

仅 +12 行：新增 `WithModelTest` option（modelTest 适配器 + 结果存储），**未触及 ListModels/ListCategories 实现**。

### 2.3 数据结构（`internal/core/types.go`）

`9b699da8..155b2b79` 对 types.go 的全部变更 = **新增 4 个常量**：

```go
+	CapSrcTest = "test"        // 离线探测确认
+	CapSrcObserved = "observed" // 审计流量观测确认
```

`ModelMetadata` 结构（含 `CapabilitySources map[string]string json:"capability_sources,omitempty"`）**在基线 `9b699da8` 已完整存在**（types.go:209），4 个产出文件（`providers/registry_metadata.go`、`modeldata/merge.go`、`enricher.go`、`merger.go`）基线亦已具备。

### 2.4 modeldata 变更 = 纯新增

`9b699da8..155b2b79` 对 `internal/modeldata/` 的 19 个变更文件**全部为新增**（capability_errors.go、modeltest/、observation.go 及其测试）——**无任何既有文件被修改**。

## 3. 运行时实测（HEAD `155b2b79`）

### 3.1 `/admin/models` 响应结构（40 模型）

```
每条 entry 的 keys: [access, model, provider_name, provider_type, selector]
├─ access:   {selector, default_enabled, effective_enabled, user_paths?, override?}
├─ model:    {id, object, owned_by, created, metadata}
├─ metadata: [capabilities, capability_sources, categories, context_window,
│             description, display_name, max_output_tokens, modes, pricing,
│             pricing_sources, rankings]   ← 12 个字段
├─ provider_name:   "oMLX" / "openai-compatible" / "deepseek"
├─ provider_type:   provider 类型
└─ selector:        "provider/model"
```

| 项目 | 实测值 |
|---|---|
| 模型总数 | 40（deepseek 3 + oMLX 20 + openai-compatible 17） |
| 含 `capability_sources` 的模型 | 14/40 |
| `capability_sources` 值域 | `"registry"`（当前无 `"test"`/`"observed"`，尚未运行模型测试/观测确认） |
| 模型来源 | `~/Library/Caches/gomodel/models.json` 缓存（HEAD 特有「缓存优先服务」路径） |

### 3.2 `/admin/models/categories` 响应（6 分类）

```
[{category:"all", display_name:"All", count:40},
 {category:"text_generation", ...count:35}, {embedding...}, {image...}, {audio...}, {utility...}]
```

### 3.3 新增端点实测（均 200）

- `GET /admin/models/capability-errors` → `{"capability_errors":[],"window_days":7}`
- `GET /admin/model-pricing-overrides` → `[]`
- `GET /admin/virtual-models` → 6 个虚拟模型（与基线同构）

## 4. 基线运行时对比（如实记录）

**尝试**：`git worktree` 检出基线 `9b699da8`，构建后端（`go build` 成功），复制 HEAD 的 config.yaml + etc/models.json，启动于 8181/8182 端口。

**结果**：基线 `/admin/models` 返回 `[]`（空），`/admin/models/categories` 返回全 0 计数。**原因**：基线版本不走「缓存优先服务」路径（`loaded models from cache, models:40` 为 HEAD 运行时日志，基线加载 `cached_models:0`），且基线进程无 provider 配置（HEAD 的 3 个 provider 定义存在于模型缓存中，基线启动时缓存已被其自身刷新覆盖为空 `providers:{}`）。

**事故与止损（重要）**：基线实验期间，基线版本的后台刷新把 `~/Library/Caches/gomodel/models.json` 覆盖为 `providers:{}`（原 40 模型 + 3 provider 定义丢失）。已通过 HEAD 的 `POST /admin/runtime/refresh` 触发 `model_registry_cache` 持久化，**缓存已完整恢复**（deepseek 3 + oMLX 20 + openai-compatible 17 = 40，`updated_at` 2026-09-07T06:27:31Z），HEAD 服务正常。基线 worktree 已清理。

**基线数据格式获取受限的结论**：无法在完全相同 provider 配置下实测基线响应；但代码层证据（handler 未变、结构未变、字段基线已有）已充分证明响应格式在 issue #8–#12 前后**无结构性差异**。

## 5. 对渲染回归的意义

- 前端拿到的 `/admin/models` 数据与基线**同构**（同字段、同结构、同来源机制）→ **排除「API 数据格式变化导致模型页渲染异常」**。
- `capability_sources` 字段（stage J/K 前端徽标读取 `row.model?.metadata?.capability_sources`）在基线即存在且值为 `"registry"` 字符串映射——前端 `Object.values(...)` 兼容（不崩溃，当前无 `"test"`/`"observed"` 值故不显示徽标）。
- 渲染回归的根因仍在**前端 Svelte 模板追踪**（见 `models-page-call-flow-analysis.md` §3），与 API 数据格式无关。
