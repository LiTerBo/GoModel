# 「模型」页面：功能入口与 API 可用性检查

> 检查日期：2026-09-07
> 环境：`http://localhost:8080`（本地 gomodel 服务，HEAD `155b2b79` 纯净构建）
> 认证：`Authorization: Bearer <GOMODEL_MASTER_KEY>`（config/.env）

## 1. Sidebar「模型」模块功能入口

### 1.1 页面地址

| 入口 | 地址 | 说明 |
|---|---|---|
| 模型页（浏览器） | `http://localhost:8080/admin/dashboard/models` | Sidebar「模型」点击后路由目标 |
| 模型页（Svelte 路由组件） | `web/dashboard/src/pages/models/ModelsPage.svelte` | 页面组件 |
| 路由注册 | `web/dashboard/src/lib/router.svelte.js` | `path: "/models"` → ModelsPage |
| Dashboard 入口 | `http://localhost:8080/admin/dashboard/` | 侧边栏根（SPA 内导航） |

> 注：Dashboard 为 SPA，Sidebar 切换「模型」是前端路由（URL 变 `/admin/dashboard/models`），不触发整页刷新；强制刷新（Enter）时由后端 `internal/admin` 静态托管兜底返回 index.html。

### 1.2 页面内子功能

| 子功能 | 交互位置 |
|---|---|
| 分类过滤 | 顶部 tab（all / text_generation / embedding / ...） |
| 虚拟模型管理 | 虚拟模型组（别名/redirect 编辑，`VirtualModelEditor`） |
| 模型测试触发 | ModelRow 行内「Test」按钮（stage J-2 新增） |
| 能力徽标/告警 | ModelRow 行内徽标 + WARN 图标（stage J/K 新增） |

## 2. 「模型」页面调用后台的 API 清单

### 2.1 页面加载时调用（ModelsPage onMount）

| # | Method | Path | 调用方 | 用途 |
|---|---|---|---|---|
| 1 | GET | `/admin/models` | models store（`models.svelte.js`） | 全部真实模型清单（含 metadata/能力） |
| 2 | GET | `/admin/models/categories` | models store | 分类 + 各分类计数 |
| 3 | GET | `/admin/virtual-models` | virtualModels store | 虚拟模型（别名/redirect）清单 |
| 4 | GET | `/admin/model-pricing-overrides` | pricingOverrides store | 定价覆盖 |
| 5 | GET | `/admin/rate-limits`（页面级） | rateLimits store | 限流规则（gauge 按钮） |
| 6 | GET | `/admin/models/capability-errors` | capabilityErrors store（stage K 新增） | 能力错误聚合（WARN 标识） |

### 2.2 按需调用（子组件/交互）

| # | Method | Path | 调用方 | 触发时机 |
|---|---|---|---|---|
| 7 | POST | `/admin/models/test` | modelTest store（stage J 新增） | 点击模型行「Test」按钮 |
| 8 | GET | `/admin/models/test-results` | modelTest store | 读取最近测试结果 |
| 9 | GET | `/admin/models/observed-suggestions` | client.js 封装（stage L 新增） | 被动观测建议 |

## 3. API 可用性模拟测试（5 次异步调用）

### 3.1 测试方法

- 工具：`curl`，`Authorization: Bearer <GOMODEL_MASTER_KEY>`
- 对 5 个页面加载端点各发起 **5 次异步调用**（`-o /dev/null -w %{http_code}` 测状态码 + 独立调用取响应体）
- 时间：2026-09-07

### 3.2 测试结果总表

| Endpoint | Run1 | Run2 | Run3 | Run4 | Run5 | 响应体 |
|---|---|---|---|---|---|---|
| `GET /admin/models` | 200 | 200 | 200 | 200 | 200 | 26,321 B（40 模型） |
| `GET /admin/models/categories` | 200 | 200 | 200 | 200 | 200 | 411 B（6 类） |
| `GET /admin/virtual-models` | 200 | 200 | 200 | 200 | 200 | 2,806 B（6 虚拟模型） |
| `GET /admin/model-pricing-overrides` | 200 | 200 | 200 | 200 | 200 | 2 B（空数组） |
| `GET /admin/models/capability-errors` | 200 | 200 | 200 | 200 | 200 | 40 B（空能力错误） |

**结论：5 端点 × 5 次 = 25/25 全部 HTTP 200，无超时（95–133ms），无 401/403/500。**

### 3.3 各端点响应体详情

#### GET /admin/models（200，26,321B）— 截断预览

```json
[{"model":{"id":"deepseek-v4-flash","object":"model","owned_by":"deepseek","created":0,
"metadata":{"display_name":"DeepSeek V4 Flash","description":"D..."}}}, ...]
```

- 返回 40 个真实模型（`/admin/models/categories` count 合计 40）
- 每个模型含 `id` / `owned_by` / `metadata.display_name` / `description` 等完整元数据

#### GET /admin/models/categories（200，411B）

```json
[{"category":"all","display_name":"All","count":40},
 {"category":"text_generation","display_name":"Text Generation","count":35},
 {"category":"embedding", ...}, ...]
```

- 6 个分类：all(40) / text_generation(35) / embedding / ...

#### GET /admin/virtual-models（200，2,806B）

```json
[{"source":"cheap","kind":"redirect","targets":[{"provider":"openai","model":"gpt-5-nano-2025-08-07","weight":1},
 {"provider":"groq","model":"llama-3.1..."} ...]}, ...]
```

- 6 个虚拟模型（redirect 类型为主，含 targets 权重）

#### GET /admin/model-pricing-overrides（200，2B）

```json
[]
```

- 空数组（无定价覆盖配置，正常）

#### GET /admin/models/capability-errors（200，40B）

```json
{"capability_errors":[],"window_days":7}
```

- 无能力错误（7 天窗口），正常

## 4. 检查结论

1. **API 层完全健康**：模型页依赖的 5 个核心端点 25/25 全部 200，数据完整（40 模型 / 6 分类 / 6 虚拟模型）。
2. **后端数据流正常**：`/admin/models` 返回的模型元数据完整，capability-errors 接口可用（空结果符合预期）。
3. **回归不在 API 层**：渲染缺失（仅虚拟模型组显示）不能归因于后端接口——数据已正确返回，问题位于**前端 Svelte 渲染链路**（详见《issue-8-12-impact-analysis.md》第 6 节：stage J/K 前端变更）。
4. **后续检查建议**：若需验证前端修复，浏览器实测时观察 Network 面板确认 6 个端点均 200 且 `filteredDisplayModelGroups` 派生结果包含真实模型组。
