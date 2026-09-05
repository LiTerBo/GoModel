# 模型供应商设计分析报告

> 基于软件架构 4+1 视图 + UI 设计分析
> 日期：2026-09-06

## 目录

1. [场景视图 (Scenario View)](#1-场景视图)
2. [逻辑视图 (Logical View)](#2-逻辑视图)
3. [开发视图 (Development View)](#3-开发视图)
4. [进程视图 (Process View)](#4-进程视图)
5. [部署视图 (Deployment View)](#5-部署视图)
6. [UI 设计分析 (UI Design Analysis)](#6-ui-设计分析)
7. [差距分析：缺少自定义模型供应商](#7-差距分析)
8. [改进建议](#8-改进建议)

---

## 1. 场景视图

### 1.1 核心用例

| 用例 | 角色 | 描述 |
|------|------|------|
| UC-01 添加供应商 | 运维人员 | 通过管理页添加一个新的模型供应商（名称、类型、API Key、Base URL） |
| UC-02 编辑供应商配置 | 运维人员 | 修改已有供应商的凭证、模型列表、启用/禁用 |
| UC-03 删除供应商 | 运维人员 | 从运行中网关移除一个供应商 |
| UC-04 供应商健康监控 | 运维人员 | 查看供应商在线状态、熔断器状态、模型发现进度 |
| UC-05 模型路由 | 最终用户 | 请求通过 `model` 或 `provider/model` 路由到对应供应商 |
| UC-06 供应商自动发现 | 系统 | 启动时从各供应商 `/v1/models` 拉取模型清单 |
| UC-07 多供应商模型路由 | 系统 | 同一模型 ID 由多个供应商提供时，按注册顺序/提供商限定名路由 |

### 1.2 用户痛点

> **P1：管理页面添加供应商无"自定义"类型选项**
>
> 当前 `ProviderCredentialEditor.svelte` 的 `<select type>` 下拉框仅列出编译时注册的供应商类型（openai、deepseek、azure、ollama……），用户无法选择"自定义"或"通用 OpenAI 兼容"类型来连接任意兼容 endpoint。

---

## 2. 逻辑视图

### 2.1 核心职责分层

```
┌──────────────────────────────────────────────────────────────────┐
│                        UI Layer (Svelte)                         │
│  ProvidersConfigPage → ProviderCredentialEditor → 表单提交       │
└──────────────────────────────┬───────────────────────────────────┘
                               │ HTTP /admin/provider-credentials/*
                               ▼
┌──────────────────────────────────────────────────────────────────┐
│                    Admin API Layer (Echo)                        │
│  handler_provider_credentials.go                                 │
│  - ListProviderCredentials   (GET)                               │
│  - ProviderCredentialTypes   (GET /types)                        │
│  - UpsertProviderCredential  (PUT)                               │
│  - DeleteProviderCredential  (DELETE /:name)                     │
│  handler_providers.go                                            │
│  - ProviderStatus            (GET /status)                       │
│  - RefreshRuntime            (POST /runtime/refresh)             │
└──────────────────────────────┬───────────────────────────────────┘
                               ▼
┌──────────────────────────────────────────────────────────────────┐
│                    Credentials Service Layer                      │
│  credentials.go → CredentialsService                             │
│  - 管理凭证增删改 + 热注册到 Registry                            │
│  - 验证类型是否在 Factory 中注册                                 │
│  credentials_store_sql.go / credentials_store_mongodb.go         │
│  - 持久化存储凭证                                                │
└──────────────────────────────┬───────────────────────────────────┘
                               ▼
┌──────────────────────────────────────────────────────────────────┐
│                   Provider Factory Layer                          │
│  factory.go → ProviderFactory                                    │
│  - 注册 ProviderConstructor 映射表 (type → constructor)          │
│  - Create(cfg) 根据类型实例化 Provider                             │
│  - CredentialSchemas() 返回各类型支持的凭证字段                   │
│  credential_schema.go → CredentialSchema                          │
│  - 描述每个供应商类型需要哪些字段 (API Key, Base URL, ...)        │
└──────────────────────────────┬───────────────────────────────────┘
                               ▼
┌──────────────────────────────────────────────────────────────────┐
│                    Provider Implementation Layer                  │
│  openai/ → CompatibleProvider (通用 OpenAI 兼容引擎)             │
│  deepseek/ → ChatCompatible (deepseek 特定配置)                  │
│  ollama/, vllm/, llamacpp/, ...                                  │
│  - 每个包导出 Registration 变量，包含 Type + Constructor +        │
│    DiscoveryConfig                                                │
└──────────────────────────────────────────────────────────────────┘
                               ▼
┌──────────────────────────────────────────────────────────────────┐
│                    Registry Layer                                 │
│  registry_list.go → ModelRegistry                                │
│  - Model → Provider 映射表，管理模型发现和路由                    │
│  router.go → Router                                              │
│  - 模型选择器解析 (model / provider/model)                       │
│  - 支持 provider-qualified 路由 (ADR-0005)                       │
└──────────────────────────────────────────────────────────────────┘
```

### 2.2 关键抽象

#### 2.2.1 Registration（供应商注册）

```go
// factory.go
type Registration struct {
    Type                        string                    // 供应商类型标识
    New                         ProviderConstructor       // 构造函数
    PassthroughSemanticEnricher core.PassthroughSemanticEnricher
    Discovery                   DiscoveryConfig           // 发现配置
}

type DiscoveryConfig struct {
    DefaultBaseURL     string           // 默认 Base URL
    RequireBaseURL     bool             // 是否必须配置 Base URL
    AllowAPIKeyless    bool             // 是否允许无 API Key
    SupportsAPIVersion bool             // 是否支持 API 版本
    NameSeparator      string           // 名称分隔符
    CredentialFields   []CredentialField // 自定义凭证字段（默认自动推导）
}
```

#### 2.2.2 CredentialSchema（凭证表单 Schema）

```go
// credential_schema.go
type CredentialSchema struct {
    Type           string            // 供应商类型名
    DefaultBaseURL string            // 默认 Base URL
    Fields         []CredentialField // 表单字段列表
}
```

Schema 由 `DiscoveryConfig` 自动推导生成（`defaultCredentialFields`），或由供应商注册时显式提供 `CredentialFields`。

#### 2.2.3 ProviderFactory（工厂）

```go
// factory.go
type ProviderFactory struct {
    builders             map[string]ProviderConstructor     // type → constructor
    discoveryConfigs     map[string]DiscoveryConfig         // type → 发现配置
    passthroughEnrichers map[string]core.PassthroughSemanticEnricher
}
```

关键方法：
- `Add(reg Registration)` — 注册供应商类型
- `Create(cfg ProviderConfig)` — 根据配置创建 Provider 实例
- `CredentialSchemas()` — 返回所有注册类型的凭证表单 Schema
- `RegisteredTypes()` — 返回所有注册类型名列表

### 2.3 数据流

#### 添加供应商数据流

```
用户 → ProvidersConfigPage (点击"添加")
     → ProviderCredentialEditor (打开弹窗)
     → GET /admin/provider-credentials/types (获取类型Schemas)
     → 用户选择类型、填写字段
     → PUT /admin/provider-credentials (提交)
     → handler_provider_credentials.go → buildProviderCredentialUpsert
     → CredentialsService.Upsert
         → 验证类型已注册
         → 保存到数据库
         → 调用 ProviderFactory.Create 创建 Provider 实例
         → 注册到 ModelRegistry
     → 返回成功
```

#### 请求路由数据流

```
用户请求 model=some-model
     → Router.ResolveModel
         → 检查是否 provider/model 格式
         → 查询 ModelRegistry 映射
         → 返回 Provider + ModelSelector
     → Provider.ChatCompletion
         → CompatibleProvider → llmclient → 上游 API
```

---

## 3. 开发视图

### 3.1 包依赖关系

```
cmd/gomodel/main.go
    └── run/run.go
        └── run/providers.go (注册所有供应商)
            ├── internal/providers/factory.go
            │   ├── internal/providers/credential_schema.go
            │   ├── internal/providers/credentials.go
            │   │   ├── internal/providers/credentials_store.go (接口)
            │   │   ├── internal/providers/credentials_store_sql.go
            │   │   └── internal/providers/credentials_store_mongodb.go
            │   ├── internal/providers/router.go
            │   └── internal/providers/registry_list.go
            ├── internal/providers/openai/       (Registration + CompatibleProvider)
            ├── internal/providers/deepseek/
            ├── internal/providers/ollama/
            ├── internal/providers/vllm/
            ├── internal/providers/llamacpp/
            ├── ... (31 个供应商包)
            └── internal/admin/handler_provider_credentials.go
                └── web/dashboard/src/pages/providers-config/
                    ├── ProvidersConfigPage.svelte
                    ├── ProviderCredentialEditor.svelte
                    ├── ProviderCredentialList.svelte
                    ├── ProviderCredentialField.svelte
                    ├── providersConfig.svelte.js
                    └── providersConfigLogic.js
```

### 3.2 已注册供应商类型（31 个）

在 `run/providers.go` 中注册的完整列表：

| 类型名 | 包 | 默认 Base URL | 说明 |
|--------|-----|---------------|------|
| `openai` | `openai` | `https://api.openai.com/v1` | OpenAI API |
| `openrouter` | `openrouter` | `https://openrouter.ai/api/v1` | OpenRouter |
| `azure` | `azure` | — | Azure OpenAI |
| `bailian` | `bailian` | — | 阿里百炼 |
| `oracle` | `oracle` | — | Oracle OCI |
| `anthropic` | `anthropic` | `https://api.anthropic.com` | Anthropic |
| `bedrock` | `bedrock` | — | AWS Bedrock |
| `bedrockmantle` | `bedrockmantle` | — | AWS Bedrock (Mantle) |
| `chatgpt` | `chatgpt` | `https://chatgpt.com` | ChatGPT |
| `chutes` | `chutes` | — | Chutes |
| `cohere` | `cohere` | — | Cohere |
| `deepseek` | `deepseek` | `https://api.deepseek.com` | DeepSeek |
| `elevenlabs` | `elevenlabs` | — | ElevenLabs |
| `fireworks` | `fireworks` | — | Fireworks AI |
| `gemini` | `gemini` | — | Google Gemini |
| `vertex` | `vertex` | — | Google Vertex AI |
| `groq` | `groq` | `https://api.groq.com/openai/v1` | Groq |
| `hetzner` | `hetzner` | — | Hetzner |
| `kilo` | `kilo` | — | Kilo AI |
| `kimicode` | `kimicode` | — | Kimi Code |
| `llamacpp` | `llamacpp` | — | llama.cpp |
| `llmd` | `llmd` | — | 内部 llmd |
| `meta` | `meta` | — | Meta (Llama) |
| `minimax` | `minimax` | — | MiniMax |
| `ollama` | `ollama` | `http://localhost:11434` | Ollama |
| `opencodego` | `opencodego` | — | OpenCode Go |
| `sglang` | `sglang` | — | SGLang |
| `vllm` | `vllm` | — | vLLM |
| `xai` | `xai` | `https://api.x.ai` | xAI |
| `xiaomi` | `xiaomi` | — | 小米 MiMo |
| `zai` | `zai` | `https://api.z.ai/v1` | Z.ai |

### 3.3 关键设计决策（ADR）

| ADR | 决策 | 影响 |
|-----|------|------|
| ADR-0001 | 不使用 `init()`，在 `main.go` 显式注册 | 控制流可见，无全局状态 |
| ADR-0004 | Capability Model + ProviderAttempt | 特性门控 + 执行单元 |
| ADR-0005 | `provider/model` 限定选择器 | 支持多供应商同名模型路由 |

---

## 4. 进程视图

### 4.1 运行时架构

```
┌─────────────────────────────────────────────────────────────┐
│                     GoModel 进程 (单二进制)                    │
│                                                              │
│  ┌──────────────┐   ┌─────────────────┐   ┌──────────────┐  │
│  │  HTTP Server  │   │  Admin API      │   │  Provider    │  │
│  │  (echo v5)    │   │  (Echo Group)   │   │  Factory     │  │
│  │  :8080        │   │  :8080/admin/*  │   │              │  │
│  │  /v1/*        │   │                 │   │  (singleton) │  │
│  └──────┬───────┘   └────────┬────────┘   └──────┬───────┘  │
│         │                    │                     │          │
│         ▼                    ▼                     ▼          │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │                    Provider Router                        │  │
│  │  ResolveModel → ModelRegistry → Provider 实例            │  │
│  └─────────────────────────────────────────────────────────┘  │
│         │                                                      │
│         ▼                                                      │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │                CompatibleProvider                         │  │
│  │  llmclient → HTTP → 上游 API                              │  │
│  │  (Key Rotation, Circuit Breaker, Retry)                  │  │
│  └─────────────────────────────────────────────────────────┘  │
│                                                              │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │           Credentials Store (SQLite / MongoDB)           │  │
│  │  持久化存储 Dashboard 管理的供应商凭证                     │  │
│  └─────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### 4.2 并发模型

- **ProviderFactory** — `sync.RWMutex` 保护读写，注册在启动时完成
- **ModelRegistry** — `sync.RWMutex` + 缓存失效模式，读多写少
- **CredentialsService** — 串行化 upsert 操作（`h.mutationMu`）
- **Provider 请求** — 每个请求独占 goroutine，通过 `llmclient` 并发调用上游

### 4.3 状态管理

| 状态 | 来源 | 生命周期 |
|------|------|----------|
| 供应商类型注册表 | 编译时 (`run/providers.go`) | 进程级，只读 |
| 供应商凭证 | 数据库 + 配置文件 | 持久化，热加载 |
| 已发现模型清单 | 供应商 `/v1/models` | 定期刷新，缓存 |
| 熔断器状态 | 请求健康追踪 | 运行时，内存态 |
| 凭证表单 Schema | 由 `DiscoveryConfig` 推导 | 运行时，只读缓存 |

---

## 5. 部署视图

### 5.1 部署形态

```
┌──────────────────────────────────────────────────────────────┐
│                 Docker / 裸机 / Kubernetes Pod                │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐  │
│  │  GoModel 进程                                           │  │
│  │  - 单端口 :8080 (OpenAI API + Admin API)                │  │
│  │  - 静态文件服务 (Svelte Dashboard)                       │  │
│  │  - 配置: config.yaml / .env                             │  │
│  │  - 数据: provider-credentials.db (SQLite)               │  │
│  │        或 MongoDB                                        │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐  │
│  │  外部依赖                                                │  │
│  │  - SQLite 或 MongoDB (凭证存储)                          │  │
│  │  - PostgreSQL (可选，语义缓存)                           │  │
│  └────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────┘
         │
         │  HTTPS
         ▼
┌──────────────────────────────────────────────────────────────┐
│                    上游 LLM API 供应商                        │
│  api.openai.com, api.deepseek.com, localhost:11434, ...      │
└──────────────────────────────────────────────────────────────┘
```

### 5.2 配置方式

供应商配置来源优先级（config.yaml → env vars → 管理页面）：

```
                        ┌─────────────────┐
                        │  config.yaml     │ ← 声明式配置
                        │  providers:      │
                        │    openai:       │
                        │      api_key: ...│
                        │      base_url: ..│
                        └────────┬────────┘
                                 │
                        ┌────────▼────────┐
                        │  Environment     │ ← 环境变量覆盖
                        │  OPENAI_API_KEY  │
                        │  OPENAI_BASE_URL │
                        └────────┬────────┘
                                 │
                        ┌────────▼────────┐
                        │  Dashboard       │ ← 运行时管理
                        │  /admin/provider │
                        │  -credentials    │
                        │  (存储到 DB)      │
                        └─────────────────┘
```

---

## 6. UI 设计分析

### 6.1 页面结构

```
ProvidersConfigPage.svelte
├── 页面标题 + 帮助信息
├── "添加"按钮 (→ openCreate)
├── 筛选输入框
├── ProviderCredentialList.svelte (表格视图)
│   ├── 名称 / 类型 / 认证方式 / Base URL / 模型数 / 状态
│   └── 编辑 / 删除操作
└── ProviderCredentialEditor.svelte (弹窗)
    ├── 类型选择 <select> (从 types API 获取)
    ├── 名称输入框
    ├── 凭证字段 (根据类型 Schema 动态渲染)
    │   ├── api_keys (多行 Key 输入)
    │   ├── base_url (文本输入)
    │   ├── session_sticky_keys (复选框)
    │   └── ...
    ├── 启用/禁用开关
    ├── 高级设置 (折叠)
    │   └── models (逗号分隔)
    └── 保存/取消按钮
```

### 6.2 类型选择器 UI 分析

`ProviderCredentialEditor.svelte` 第 76-90 行：

```html
<select id="provider-credential-type" bind:value={providersConfig.form.type}
        disabled={providersConfig.formMode === "edit"}>
  <option value="" disabled>选择供应商类型</option>
  {#each typeOptions as type (type)}
    <option value={type}>{type}</option>
  {/each}
</select>
```

**数据来源**：`GET /admin/provider-credentials/types` → `ProviderFactory.CredentialSchemas()` → 遍历 `f.builders` 的 key。

**呈现结果**：31 个预注册类型名的平铺列表，按字母排序。

### 6.3 凭证字段动态渲染

字段 Schema 由 `credential_schema.go` 的 `CredentialSchema` 定义：

- 默认推导（`defaultCredentialFields`）：`api_keys` (required) + `base_url` (advanced)
- 显式声明（`DiscoveryConfig.CredentialFields`）：自定义字段列表
- 所有类型自动追加：`models` (advanced) + `session_sticky_keys` (仅当有 api_keys 时)

### 6.4 前端状态管理

`providersConfig.svelte.js` 使用 Svelte 5 runes：

```js
class ProvidersConfigState {
  rows = $state([])       // 已配置的供应商列表
  types = $state([])      // 可选的供应商类型列表
  form = $state(defaultForm())  // 编辑器表单状态
  formMode = $state('create' | 'edit')
  ...
}
```

---

## 7. 差距分析：缺少自定义模型供应商

### 7.1 问题描述

当前管理页面**没有"自定义 OpenAI 兼容"供应商类型**。用户只能从预注册的 31 个具体类型中选择：

- ✅ 添加 OpenAI 官方 → 选 `openai`
- ✅ 添加 DeepSeek 官方 → 选 `deepseek`
- ✅ 添加本地 Ollama → 选 `ollama`
- ❌ 添加自建 vLLM 服务（非标准 vLLM 部署） → 只能选 `openai` 改 base_url
- ❌ 添加任意 OpenAI 兼容代理 → 无法直观表达

### 7.2 根因分析

| 层面 | 根因 | 影响 |
|------|------|------|
| **架构** | 供应商类型 = 编译时注册的 Go 包 | 无法在运行时添加新类型 |
| **API** | `RegisteredTypes()` 只返回 `f.builders` 的 key | 无"自定义"占位类型 |
| **Schema** | `CredentialSchemas()` 只遍历已注册类型 | 自定义类型无 Schema |
| **UI** | 类型选择器只渲染 `typeOptions` 列表 | 无"自定义"或"通用"选项 |
| **配置** | config.yaml 支持 `type: openai` + 任意 `base_url` | 但 Dashboard 无法表达相同语义 |

### 7.3 技术可行性

实际上，GoModel 的 `CompatibleProvider` 是**通用 OpenAI 兼容引擎**——它接受 `base_url`、`api_key`、`setHeaders` 等参数，可以连接任何 OpenAI 兼容 API。但该引擎只能通过已注册的供应商类型间接访问。

**关键事实**：`NewCompatibleProvider` 和 `NewChatCompatible` 都是公开的通用构造函数，技术上可以注册一个 `openai-compatible` 通用类型。

### 7.4 现有变通方案

1. **config.yaml 声明式配置**：`providers.my-custom: { type: openai, base_url: "https://..." }` — 但需要通过重启加载
2. **Dashboard 选 `openai` 类型**：改 base_url 为自定义地址 — 语义混淆，无"自定义"标识
3. **直接注册新的供应商包**：编写新的 Go 包，导入 `run/providers.go` — 需要重新编译

---

## 8. 改进建议

### 8.1 方案 A：添加 `openai-compatible` 通用类型（推荐）

**思路**：在 `run/providers.go` 中注册一个额外的 `openai-compatible` 类型，使用与 `openai` 相同的 `CompatibleProvider` 引擎，但明确标记为通用兼容类型。

```go
// 在 run/providers.go 中添加
var openaiCompatibleRegistration = providers.Registration{
    Type: "openai-compatible",
    New:  openai.NewCompatible,  // 或包装一层
    Discovery: providers.DiscoveryConfig{
        DefaultBaseURL: "http://localhost:8000/v1",
        NameSeparator:  "-",
        // 显式声明凭证字段
        CredentialFields: []providers.CredentialField{
            {Name: providers.CredentialFieldAPIKeys, Required: false},
            {Name: providers.CredentialFieldBaseURL, Required: false},
        },
    },
}
```

**优点**：
- 改造成本极低（~20 行代码）
- 语义清晰，用户一眼知道是通用类型
- 兼容现有 `CompatibleProvider` 引擎
- 默认 URL 可设为 `http://localhost:8000/v1` 更合理

**缺点**：
- 仍需编译时注册，无法完全动态扩展
- `openai` 和 `openai-compatible` 共享相同引擎，但 Schema 不同

### 8.2 方案 B：运行时注册 + 动态类型

**思路**：在 Dashboard 中提供"自定义"选项，用户输入类型名、Base URL、API Key 后，后端在运行时注册一个新的 Provider 类型。

**需要修改**：
- `ProviderFactory.Add()` 在运行时调用
- 构造动态的 `ProviderConstructor` 闭包
- `CompatibleProvider` 已支持动态配置

**优点**：
- 完全灵活，无需修改代码即可添加任意供应商
- 类型名由用户自定义，语义清晰

**缺点**：
- 改动较大，涉及 Factory 的运行时安全
- 需考虑持久化注册信息（重启后恢复）
- 需要处理类型冲突

### 8.3 方案 C：UI 层增加"自定义"选项，映射到 openai 类型

**思路**：在 Dashboard 类型选择器中增加一个"自定义 (OpenAI 兼容)"选项，提交时映射为 `type: openai`。

**需要修改**：
- `providersConfigLogic.js` 增加一个虚拟选项
- `ProviderCredentialEditor.svelte` 条件渲染自定义提示
- 提交时类型映射为 `openai`

**优点**：
- 前端改动为主，无需后端修改
- 用户直观

**缺点**：
- 后端仍记录为 `openai` 类型，语义不准确
- 统计/审计时无法区分"真 OpenAI"和"自定义兼容"

### 8.4 方案对比

| 维度 | 方案 A（推荐） | 方案 B | 方案 C |
|------|:---:|:---:|:---:|
| 实现成本 | 低 (~20 行) | 高 (~200+ 行) | 低 (~30 行) |
| 语义清晰度 | ★★★★★ | ★★★★★ | ★★★☆☆ |
| 运行时灵活性 | ★★☆☆☆ | ★★★★★ | ★★☆☆☆ |
| 统计区分度 | ★★★★★ | ★★★★★ | ★★☆☆☆ |
| 后续扩展性 | ★★★☆☆ | ★★★★★ | ★★☆☆☆ |
| 破坏性风险 | 极低 | 中 | 低 |

### 8.5 推荐

**短期（立即）**：实施**方案 A**——在 `run/providers.go` 中注册 `openai-compatible` 通用类型，Dashboard 自动显示为可选类型。

**中期（可选）**：评估**方案 B**——如果用户频繁需要连接自定义 endpoint，可在运行时动态注册机制，允许 Dashboard 直接创建任意类型。

### 8.6 方案 A 具体实现

```go
// 在 run/providers.go 中添加
var _ = func() {
    // 确保 openai 包已导入（已导入）
    // 注册 openai-compatible 通用类型
    openaiCompatible := providers.Registration{
        Type: "openai-compatible",
        New: func(cfg providers.ProviderConfig, opts providers.ProviderOptions) core.Provider {
            baseURL := providers.ResolveBaseURL(cfg.BaseURL, "http://localhost:8000/v1")
            return openai.NewChatCompatible(cfg.APIKey, opts, openai.CompatibleProviderConfig{
                ProviderName: cfg.Name,
                BaseURL:      baseURL,
            })
        },
        Discovery: providers.DiscoveryConfig{
            DefaultBaseURL:  "http://localhost:8000/v1",
            RequireBaseURL:  false,
            AllowAPIKeyless: true,
        },
    }
    // 注：在 defaultProviderFactory 中 factory.Add(openaiCompatible)
}()
```

---

## 附录

### A. 参考文件

| 文件 | 说明 |
|------|------|
| `run/providers.go` | 供应商注册入口 |
| `internal/providers/factory.go` | 工厂模式 + 注册表 |
| `internal/providers/credential_schema.go` | 凭证 Schema 定义 |
| `internal/providers/router.go` | 模型路由解析 |
| `internal/providers/registry_list.go` | 模型清单管理 |
| `internal/providers/openai/compatible_provider.go` | 通用 OpenAI 兼容引擎 |
| `internal/providers/openai/chat_compatible.go` | 聊天兼容适配器 |
| `internal/providers/openai/openai.go` | OpenAI 注册 |
| `internal/admin/handler_provider_credentials.go` | Admin API |
| `internal/admin/handler_providers.go` | 供应商状态 API |
| `web/dashboard/src/pages/providers-config/` | Dashboard UI |
| `docs/adr/0001-explicit-provider-registration.md` | ADR-0001 |
| `docs/adr/0004-capability-model-and-provider-attempts.md` | ADR-0004 |
| `docs/adr/0005-provider-qualified-model-selectors.md` | ADR-0005 |

### B. 相关数据

- 注册供应商类型：31 个
- 供应商实现包：33 个（含共享组件）
- 测试文件：491 个
- Dashboard 供应商配置页面组件：6 个 Svelte 文件