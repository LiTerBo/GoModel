# 工作任务清单：添加通用 OpenAI 兼容供应商类型

> 关联: [需求说明](../requirements/req-openai-compatible-provider.md) | [架构设计分析](../arch/analysis-provider-supplier.md)
> 里程碑: 通用供应商类型支持
> 估算: 2 人天

---

## 任务一览

| ID | 任务 | 模块 | 预估 | 依赖 | 状态 |
|----|------|------|------|------|------|
| AD-A1 | 注册 `openai-compatible` 供应商类型 | `run/providers.go` | 0.5d | — | ⏳ |
| AD-A2 | 添加 i18n 消息 | Dashboard 消息 | 0.2d | AD-A1 | ⏳ |
| AD-A3 | 单元测试 | `internal/providers` | 0.3d | AD-A1 | ⏳ |
| AD-A4 | 用手动测试验证 | 端到端验证 | 0.3d | AD-A1~A3 | ⏳ |
| AD-A5 | 更新文档 | `docs/` | 0.2d | AD-A1 | ⏳ |

---

## AD-A1: 注册 `openai-compatible` 供应商类型

**目标**: 在 `run/providers.go` 的 `defaultProviderFactory` 中添加新的通用供应商类型注册，使 Dashboard 自动识别。

**验收标准**:
- [ ] `run/providers.go` 中调用 `factory.Add(openaiCompatibleRegistration)`
- [ ] 类型名为 `"openai-compatible"`
- [ ] 构造函数使用 `openai.NewChatCompatible`
- [ ] `DiscoveryConfig` 配置：
  - `DefaultBaseURL`: `"http://localhost:8000/v1"`
  - `AllowAPIKeyless`: `true`
  - `RequireBaseURL`: `false`
- [ ] 凭证字段显式声明：`api_keys` (可选) + `base_url` (可选)
- [ ] 编译通过，`make build` 成功
- [ ] 现有测试全部通过（`make test`）

**设计要点**:
- 复用 `openai` 包的 `Registration` 结构，因为 `openai` 包已经导入了 `run/providers.go`
- 构造函数闭包使用 `openai.NewChatCompatible` 而非 `openai.New`，因为通用类型不需要 OpenAI 特有的 `setHeaders` 逻辑
- `models` 字段自动追加到凭证 Schema 末尾（由 `credentialSchema` 统一处理）

---

## AD-A2: 添加 i18n 消息

**目标**: 在 Paraglide 消息文件中添加 `openai-compatible` 类型的本地化显示名。

**验收标准**:
- [ ] `messages/en.json` 中添加 `providers_type_openai_compatible` 键
- [ ] `messages/zh.json` 中添加对应中文翻译
- [ ] `messages/pl.json` 中添加对应波兰语翻译

**i18n 键建议**:
```json
{
  "providers_type_openai_compatible": {
    "message": "OpenAI Compatible",
    "description": "Provider type name for generic OpenAI-compatible endpoints"
  }
}
```
中文: `"OpenAI 兼容"`

---

## AD-A3: 单元测试

**目标**: 为新类型添加注册、创建、Schema 验证三个层面的单元测试。

**验收标准**:
- [ ] 测试 `run/providers.go` 的 `defaultProviderFactory` 返回的工厂中包含 `"openai-compatible"` 类型
- [ ] 测试 `ProviderFactory.CredentialSchemas()` 返回的 Schema 包含 `"openai-compatible"` 条目
- [ ] 测试该类型的 Schema 字段包含 `api_keys`、`base_url`、`models`
- [ ] 测试 `api_keys` 字段的 `Required` 为 `false`
- [ ] 测试 `base_url` 字段的 `Required` 为 `false`
- [ ] 测试 `ProviderFactory.Create("openai-compatible", ...)` 返回非 nil Provider
- [ ] 测试创建的 Provider 的 `GetBaseURL()` 返回配置的 Base URL 或默认值

**测试文件位置**:
- `run/providers_test.go` — 扩展已有测试
- 或 `internal/providers/factory_test.go` — 扩展工厂测试

---

## AD-A4: 手动验证

**目标**: 端到端验证新类型在 Dashboard 中的表现。

**验收标准**:
- [ ] Dashboard 类型选择下拉框中出现 `"openai-compatible"` 选项
- [ ] 选择后表单字段正确显示（api_keys 可选、base_url 可选、有默认值提示）
- [ ] 填写有效信息后保存成功
- [ ] 保存后供应商在列表中显示，类型为 `"openai-compatible"`
- [ ] 编辑和删除功能正常
- [ ] 删除后重新注册正常

---

## AD-A5: 更新文档

**目标**: 在文档中说明新类型的使用方法。

**验收标准**:
- [ ] 在 `docs/` 相关文档中说明 `openai-compatible` 类型的用途
- [ ] 提供使用示例（Dashboard 操作步骤 + config.yaml 等效配置）
- [ ] 说明与 `openai` 类型的区别

---

## 排期依赖图

```
AD-A1 (注册) ──→ AD-A2 (i18n)
   │               │
   ├──→ AD-A3 (测试) │
   │               │
   ├──→ AD-A5 (文档) │
   │               │
   └──→ AD-A4 (验证) ←┘
```

AD-A1 是唯一前置依赖，AD-A2/A3/A5 可并行，AD-A4 需等待所有前置完成。

---

## 文件改动清单

| 文件 | 改动类型 | 说明 |
|------|----------|------|
| `run/providers.go` | 修改 | 添加 `openai-compatible` 注册 |
| `messages/en.json` | 修改 | 新增 i18n 键 |
| `messages/zh.json` | 修改 | 新增 i18n 翻译 |
| `messages/pl.json` | 修改 | 新增 i18n 翻译 |
| `run/providers_test.go` 或 `internal/providers/factory_test.go` | 修改 | 新增测试 |
| `docs/`（相关文档） | 修改 | 新增使用说明 |
| 其他文件 | 无变化 | 无 |