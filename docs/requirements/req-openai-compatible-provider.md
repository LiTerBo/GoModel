# 需求说明：添加通用 OpenAI 兼容供应商类型

> 版本: v1.0
> 日期: 2026-09-06
> 关联: [架构设计分析](../arch/analysis-provider-supplier.md)

---

## 1. 目标与痛点

### 1.1 现状

GoModel 管理页面（Dashboard）的"添加供应商"弹窗中，类型选择下拉框仅列出编译时注册的 31 个具体供应商类型（`openai`、`deepseek`、`ollama`、`vllm`……）。用户无法通过管理页面添加一个**非预注册的、任意 OpenAI 兼容 API 端点**作为供应商。

### 1.2 用户痛点

| # | 痛点 | 场景示例 |
|---|------|----------|
| P1 | 管理页面无法添加"自定义"供应商类型 | 用户自建了一个 vLLM 服务 `http://my-vllm:8000/v1`，但 Dashboard 没有"自定义"选项可选 |
| P2 | 只能选 `openai` 类型后改 base_url，语义混淆 | 审计/统计时，所有自定义 endpoint 和 OpenAI 官方混在一起，无法区分 |
| P3 | 新用户找不到添加自定义 endpoint 的入口 | 文档中 indirect 建议用 `openai` 类型，但 Dashboard 没有任何提示 |
| P4 | 无法通过管理页面快速试用新 endpoint | 需要修改 config.yaml 并重启才能生效（虽然本功能可以热加载，但入口不直观） |

### 1.3 目标

在 GoModel 管理页面中，提供一个**通用的 "OpenAI 兼容" 供应商类型**，让用户能够：

- 添加任意 OpenAI 兼容 API 端点（如自建 vLLM、SGLang、llama.cpp、自定义代理等）
- 通过 Dashboard 直观配置，无需修改代码或重启服务
- 在审计/统计中与具体供应商类型区分开

---

## 2. 流程思路

### 2.1 用户操作流程

```
用户登录 Dashboard
  → 进入 供应商配置 页面
  → 点击 "添加" 按钮
  → 类型选择下拉框中出现 "openai-compatible" 选项
  → 选择该类型，填写：
      - 名称（如 my-vllm）
      - API Key（可选，有些 endpoint 不需要）
      - Base URL（如 http://my-vllm:8000/v1）
  → 点击保存
  → 系统自动：
      1. 验证 Base URL 可达性
      2. 调用 `/v1/models` 发现模型清单
      3. 注册到运行中路由表
      4. 持久化凭证到数据库
  → 用户可在 Dashboard 中看到新供应商在线
```

### 2.2 数据流

```
Dashboard → PUT /admin/provider-credentials
  → admin handler → CredentialsService.Upsert
    → ProviderFactory.Create("openai-compatible", {...})
      → openai.NewChatCompatible(...)  [通用引擎]
    → ModelRegistry 注册模型
  → 返回成功
```

---

## 3. 技术难点

| # | 难点 | 说明 | 对策 |
|---|------|------|------|
| D1 | 通用类型与具体类型共享引擎 | `openai-compatible` 需要复用 `openai` 包的 `CompatibleProvider` / `ChatCompatible`，但它们是为特定供应商设计的 | 直接复用 `NewChatCompatible`，提供配置对象即可，已有接口通用性足够 |
| D2 | 凭证 Schema 不同 | 通用类型不需要强制 API Key，base_url 应标注为常用字段而非高级设置 | 通过 `DiscoveryConfig.CredentialFields` 显式声明 |
| D3 | 默认 Base URL 选择 | 通用类型没有官方默认值 | 设为 `http://localhost:8000/v1`（常见本地推理服务端口） |
| D4 | 模型发现兼容性 | 不是所有 OpenAI 兼容 endpoint 都实现 `/v1/models` | 利用现有 `CompatibleProvider.ListModels` 的容错机制，允许失败 |
| D5 | UI 国际化 | 新类型名称 "openai-compatible" 需要在 Paraglide i18n 中加入中文翻译 | 更新 `messages/zh.json`、`messages/en.json`、`messages/pl.json` |

---

## 4. 功能需求与非功能需求

### 4.1 功能需求

| ID | 描述 | 优先级 |
|----|------|--------|
| F-01 | 在 `run/providers.go` 中注册 `openai-compatible` 供应商类型 | P0 |
| F-02 | 该类型使用 `openai.NewChatCompatible` 作为构造函数 | P0 |
| F-03 | 凭证表单不强制要求 API Key（`AllowAPIKeyless: true`） | P0 |
| F-04 | Base URL 默认值设为 `http://localhost:8000/v1` | P0 |
| F-05 | Dashboard 自动显示新类型，无需修改前端代码 | P0 |
| F-06 | 类型名在 i18n 消息中有本地化显示 | P1 |
| F-07 | 支持模型手动指定（`models` 字段，置于高级设置） | P1 |
| F-08 | 文档中说明该类型的用途和使用方法 | P1 |
| F-09 | 添加该类型的单元测试（factory 注册、创建、schema 验证） | P1 |

### 4.2 非功能需求

| ID | 描述 | 指标 |
|----|------|------|
| NF-01 | 新增类型不影响现有供应商注册和路由 | 现有测试全部通过 |
| NF-02 | 热注册/热删除功能与现有类型一致 | 无需重启 |
| NF-03 | 代码改动量最小化 | 不超过 30 行 Go 代码 |
| NF-04 | 审计日志中能区分 `openai` 和 `openai-compatible` | 类型字段不同 |

---

## 5. 数据模型

### 5.1 新增类型注册

```go
// 注册类型名
const TypeOpenAICompatible = "openai-compatible"

// DiscoveryConfig
DiscoveryConfig{
    DefaultBaseURL:  "http://localhost:8000/v1",
    RequireBaseURL:  false,   // base_url 可选（使用默认值）
    AllowAPIKeyless: true,    // API Key 可选
    CredentialFields: []CredentialField{
        {Name: "api_keys", Required: false},
        {Name: "base_url", Required: false},
    },
}
```

### 5.2 存储格式（无变化）

现有 `ManagedProviderCredential` 结构体已支持所有字段，类型值设为 `"openai-compatible"` 即可。无需新增字段或表结构。

---

## 6. API

### 6.1 Admin API（无变化）

现有 `GET /admin/provider-credentials/types` 自动返回新注册的类型，无需新增端点。

```json
// 响应中新增一条
{
  "type": "openai-compatible",
  "default_base_url": "http://localhost:8000/v1",
  "fields": [
    {"name": "api_keys", "required": false, "advanced": false},
    {"name": "base_url", "required": false, "advanced": false},
    {"name": "models", "required": false, "advanced": true}
  ]
}
```

### 6.2 PUT 请求示例

```json
{
  "name": "my-vllm",
  "type": "openai-compatible",
  "api_keys": ["sk-xxx"],
  "base_url": "http://my-vllm:8000/v1",
  "models": ["Qwen2.5-14B-Instruct", "Qwen2.5-32B-Instruct"]
}
```

---

## 7. 客户端边界

- **Dashboard**（Svelte 5）：无需修改，类型自动出现在下拉框中
- **OpenAPI / Swagger**：无需修改，admin API 的 types 响应自动包含新类型
- **CLI 工具**：无影响

---

## 8. 安全

- 通用类型使用与现有 `openai` 类型相同的认证机制（Bearer Token）
- API Key 存储和脱敏处理与现有类型一致
- 无新增安全风险

---

## 9. 部署

| 部署方式 | 影响 |
|----------|------|
| 源码编译 | 需要重新编译，新增代码在 `run/providers.go` |
| Docker 镜像 | 需要重新构建 |
| Helm Chart | 无影响 |
| 热更新 | 新类型通过 `PUT /admin/provider-credentials` 热注册，无需重启 |

---

## 10. 差异分析

### 与现有 `openai` 类型的对比

| 维度 | `openai` | `openai-compatible` |
|------|----------|---------------------|
| 默认 Base URL | `https://api.openai.com/v1` | `http://localhost:8000/v1` |
| API Key 要求 | 必填 | 可选 |
| 语义 | 专指 OpenAI 官方 API | 通用 OpenAI 兼容端点 |
| 认证方式 | Bearer Token | Bearer Token（可选） |
| 模型发现 | 调用 `/v1/models` | 调用 `/v1/models` |
| 引擎 | `CompatibleProvider` | `ChatCompatible`（轻量版） |

### 与 config.yaml 声明式配置的关系

config.yaml 中已支持 `type: openai` + 任意 `base_url` 的方式连接自定义端点。本功能是在 Dashboard 中提供等效能力，**不改变** config.yaml 的行为。

---

## 11. 决策附录

### D1: 类型命名

- **选项 A**: `openai-compatible`（推荐）
- **选项 B**: `custom`（过于通用，无法体现协议）
- **选项 C**: `generic-openai`（冗长）
- **决策**: `openai-compatible`，语义清晰，与 K8s 命名风格一致

### D2: 默认 Base URL

- **选项 A**: `http://localhost:8000/v1`（推荐，常见推理端口）
- **选项 B**: `http://localhost:11434`（Ollama 默认端口，但 Ollama 已有独立类型）
- **选项 C**: 无默认值（强制用户填写）
- **决策**: `http://localhost:8000/v1`，减少用户填写量

### D3: API Key 要求

- **选项 A**: 可选（推荐）
- **选项 B**: 必填
- **决策**: 可选，因为本地推理服务（vLLM、llama.cpp 等）通常不需要认证

### D4: 引擎选择

- **选项 A**: `NewChatCompatible`（推荐）
- **选项 B**: `NewCompatibleProvider`（全功能，含音频/文件）
- **决策**: `NewChatCompatible`，因为通用类型主要用于聊天/补全/嵌入，全功能接口会增加不必要的复杂性