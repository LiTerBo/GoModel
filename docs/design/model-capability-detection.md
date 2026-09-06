# 模型能力探测：现状分析与改造方案

> 作者：Hermes Agent · 2026-09-06
> 基于 GoModel 代码库 `internal/modeldata/` + `internal/core/types.go` 分析，对标 OmniRoute 能力探测架构

---

## 目录

1. [现状梳理](#1-现状梳理)
2. [问题分析](#2-问题分析)
3. [改造方案](#3-改造方案)
   - [A. 单一真源 `isVisionModelID`](#a-单一真源-isvisionmodelid)
   - [B. 能力来源可追溯](#b-能力来源可追溯)
   - [C. InferModes 与 InferCapabilities 合并 + 反证黑名单](#c-infermodes-与-infercapabilities-合并--反证黑名单)
4. [实施路线图](#4-实施路线图)
5. [与 OmniRoute 架构对比](#5-与-omniroute-架构对比)

---

## 1. 现状梳理

### 1.1 模型类型与能力探测的现有层

当前 GoModel 有三个来源，**顺序已经正确**（catalog 为 base，discovered 为 override，config 为最高覆盖）：

```
# 优先级：↑ 低 → 高 ↓

Layer 1: 远程模型注册表 (models.json)
  ├─ 加载：fetcher.go Fetch() → Parse()
  ├─ 合并：merger.go Resolve() → buildMetadata()
  └─ 字段：Modes[]string, Capabilities map[string]bool, Modalities, etc.

Layer 2: Provider 自身上报的 metadata
  ├─ 入口：enricher.go Enrich(accessor, list)
  ├─ 逻辑：catalog = Resolve(list, providerType, modelID)
  │        discovered = accessor.DiscoveredMetadata(modelID)
  │        merged   = MergeMetadata(catalog, discovered)
  └─ 原则：catalog 是 base，discovered 是 override（provider 知道自己的真实部署状态）

Layer 3: 配置声明 (config.yaml)
  ├─ 入口：config/provider_models.go RawProviderModel.Metadata
  ├─ 合并：merge.go MergeMetadata(base, override) 逐字段 override
  └─ 字段覆盖：modes, capabilities, context_window, max_output_tokens, pricing …
```

### 1.2 类型识别（Modes）的核心逻辑

`internal/modeldata/infer.go:60 InferModesFromID()`

```go
func InferModesFromID(modelID string) []string
```

函数按**固定顺序**逐级判断（从最具体到最默认）：

| 优先级 | 规则 | 返回 modes |
|--------|------|-----------|
| 1 | ID 含 `rerank` | `["rerank"]` |
| 2 | whole-token 匹配 embedding 家族（bge/e5/gte/minilm） | `["embedding"]` |
| 3 | ID 含 `embed` 子串 | `["embedding"]` |
| 4 | ID 含 ASR 标记（whisper/sensevoice/…） | `["audio_transcription"]` |
| 5 | ID 含 TTS 标记（bark/kokoro/…） | `["audio_speech"]` |
| 6 | ID 含对话音频标记（audio/voice/…） | `["chat","audio_transcription"]` |
| 7 | ID 含 image gen 标记（flux/dall-e/…） | `["image_generation"]` |
| 8 | ID 含 vision 标记（qwenvl/vision/vl） | `["chat"]`（仅模式，不标记 capability） |
| 9 | 默认 | `["chat"]` |

### 1.3 能力探测（Capabilities）的现有逻辑

`internal/modeldata/infer.go:140 InferCapabilitiesFromID()`

```go
func InferCapabilitiesFromID(modelID string) map[string]bool
```

只探测 vision——逻辑与 `InferModesFromID` 的 vision 段**几乎完全重复**：

```go
// infer.go:150
if strings.Contains(id, "qwenvl") || strings.Contains(id, "vision") {
    return map[string]bool{"vision": true}
}
for _, token := range strings.FieldsFunc(id, isModelIDDelimiter) {
    if _, ok := visionFamilyTokens[token]; ok {
        return map[string]bool{"vision": true}
    }
}
```

### 1.4 能力标记的合并机制

`merge.go:18 MergeMetadata(base, override *core.ModelMetadata)`

Capabilities 的合并是 **key-by-key 覆盖**（override 的 key 替换 base 的同名 key）：

```go
// merge.go:57-61
if len(override.Capabilities) > 0 {
    out := make(map[string]bool, len(merged.Capabilities)+len(override.Capabilities))
    maps.Copy(out, merged.Capabilities)
    maps.Copy(out, override.Capabilities)
    merged.Capabilities = out
}
```

### 1.5 数据模型

```go
// core/types.go:195
type ModelMetadata struct {
    Modes        []string          // 模型类型（chat/embedding/…）
    Capabilities map[string]bool   // 能力标记（vision:true/…）
    // …
    PricingSources map[string]string // 各价格字段的来源标记（已存在，但不是所有字段都有）
}
```

---

## 2. 问题分析

### 问题 1：vision 判定逻辑重复（违反 DRY）

`InferModesFromID` 中第 8 步的 vision 检查（`infer.go:124-131`）与 `InferCapabilitiesFromID` 中的 vision 检查（`infer.go:150-157`）逻辑几乎完全一样。两处都维护着 `visionFamilyTokens` 和 `"qwenvl"/"vision"` 子串匹配。

**后果**：新加一个 vision 模型家族（如 `qwen3-vl`）时，忘记同步其中一个函数就会导致 mode 判定正确但 capability 判定错误（或反之）——这正是 OmniRoute 历史上踩过的坑（`visionModels.ts` 注释明确记录）。

### 问题 2：Capabilities 来源不可追溯

`Capabilities map[string]bool` 单纯是一个布尔值，无法知道它来自 registry 标注、provider 自报、config 覆盖还是 heuristic 推断。

**后果**：debug 时无法回答"为什么这个模型被标记为 `vision:true`？"；未来若支持更丰富的能力（`function_calling`、`reasoning`、`streaming`），来源混乱会加剧。

### 问题 3：缺少反证黑名单

`TOOL_CALLING_UNSUPPORTED_PATTERNS` 等反证列表在 GoModel 中不存在。目前仅 `InferModesFromID` 通过模式匹配**归类**（embedding/audio/rerank 不走 chat 路由），但：

- 没有机制防止某个**非 chat 模型**被错误赋予 `function_calling:true` 或 `vision:true`（如果 registry 不小心标了）
- 没有类似 OmniRoute `KNOWN_TEXT_ONLY_DESPITE_SYNC` 的硬编码反证列表

### 问题 4（潜在）：InferCapabilitiesFromID 与 InferModesFromID 的返回不一致

`InferModesFromID` 返回 `["chat"]` 对 vision 模型（mode 没变化），而 `InferCapabilitiesFromID` 返回 `{"vision":true}`。两个函数各自返回**不同类型的 json**——一个是 `[]string`、一个是 `map[string]bool`。如果未来有调用方只调其中一个，就会得到不完整的模型画像。

---

## 3. 改造方案

### A. 单一真源 `isVisionModelID`

**目标**：消除 vision 判定逻辑重复，确保所有路径共用同一裁决。

**改动**：在 `infer.go` 中新增一个导出函数，两个现有函数都调用它。

```go
// infer.go — 新增
// visionFamilyTokens 保持不变（全小写，whole-token 匹配）

// IsVisionModelID checks whether a model ID looks like a vision-capable model.
// Case-insensitive, prefers the final segment of namespaced IDs (hf repo paths).
// This is the single source of truth for ID-based vision detection.
// Use it from InferModesFromID, InferCapabilitiesFromID, and any
// compression/preprocessing path that needs to decide whether images
// can be passed to this model.
func IsVisionModelID(modelID string) bool {
    id := strings.ToLower(strings.TrimSpace(modelID))
    if idx := strings.LastIndex(id, "/"); idx >= 0 {
        id = id[idx+1:]
    }
    if id == "" {
        return false
    }
    if strings.Contains(id, "qwenvl") || strings.Contains(id, "vision") {
        return true
    }
    for _, token := range strings.FieldsFunc(id, isModelIDDelimiter) {
        if _, ok := visionFamilyTokens[token]; ok {
            return true
        }
    }
    return false
}
```

**修改 `InferModesFromID`** 第 6 段（`infer.go:122-131`）：

```go
// 原来
if strings.Contains(id, "qwenvl") || strings.Contains(id, "vision") {
    return []string{"chat"}
}
for _, token := range strings.FieldsFunc(id, isModelIDDelimiter) {
    if _, ok := visionFamilyTokens[token]; ok {
        return []string{"chat"}
    }
}

// 改为
if IsVisionModelID(modelID) {
    return []string{"chat"}
}
```

**修改 `InferCapabilitiesFromID`** 整个函数体（`infer.go:140-159`）：

```go
// 原来
func InferCapabilitiesFromID(modelID string) map[string]bool {
    // ... 复制粘贴的 vision 判断 ...
}

// 改为
func InferCapabilitiesFromID(modelID string) map[string]bool {
    if IsVisionModelID(modelID) {
        return map[string]bool{"vision": true}
    }
    return nil
}
```

**影响范围**：
- 新增：`IsVisionModelID` 函数（导出，可被其他 package 使用）
- 修改：`InferModesFromID` 第 6 段，`InferCapabilitiesFromID` 全部
- 删除：`visionFamilyTokens` 在 `InferModesFromID` 中的独立引用变为集中调用
- 测试：`infer_test.go` 所有 vision 用例保持通过（测试不变，逻辑不变）

### B. 能力来源可追溯

**目标**：对每个能力标记，能追溯其来源（registry / discovered / config / heuristic）。

**改动**：在 `ModelMetadata` 中新增 `CapabilitySources` 映射，与 `PricingSources` 模式一致。

```go
// core/types.go — 在 ModelMetadata struct 中新增
CapabilitySources map[string]string `json:"capability_sources,omitempty" yaml:"-"`

// 值枚举：
const (
    CapSrcRegistry   = "registry"    // 来自 models.json 远程注册表
    CapSrcDiscovered = "discovered"  // 来自 provider 自身上报
    CapSrcConfig     = "config"      // 来自 config.yaml 中的 metadata
    CapSrcHeuristic  = "heuristic"   // 来自 InferCapabilitiesFromID 启发式
)
```

**`buildMetadata` 中记录来源**（`merger.go:258` 附近）：

```go
// 在 buildMetadata 中，当从 model.Capabilities 赋值时：
if model != nil && len(model.Capabilities) > 0 {
    meta.Capabilities = model.Capabilities
    meta.CapabilitySources = make(map[string]string, len(model.Capabilities))
    for k := range model.Capabilities {
        meta.CapabilitySources[k] = CapSrcRegistry
    }
}
```

**`MergeMetadata` 中按来源更新**（`merge.go:57-61`）：

```go
// 合并 capabilities 时，保持或更新来源
if len(override.Capabilities) > 0 {
    out := make(map[string]bool, len(merged.Capabilities)+len(override.Capabilities))
    maps.Copy(out, merged.Capabilities)
    maps.Copy(out, override.Capabilities)
    merged.Capabilities = out

    // 来源合并：override 的 key 来源更新为 override 来源
    if override.CapabilitySources != nil {
        if merged.CapabilitySources == nil {
            merged.CapabilitySources = make(map[string]string)
        }
        for k, src := range override.CapabilitySources {
            merged.CapabilitySources[k] = src
        }
    }
}
```

**`Enrich` 中标记 discovered 来源**（`enricher.go:58` 附近）：

```go
// 在 catalog 为基础、discovered 为 override 的合并后，
// 标记被 override 的能力为 discovered 来源
merged := MergeMetadata(catalog, discovered)
if discovered != nil && len(discovered.Capabilities) > 0 {
    if merged.CapabilitySources == nil {
        merged.CapabilitySources = make(map[string]string)
    }
    for k := range discovered.Capabilities {
        merged.CapabilitySources[k] = CapSrcDiscovered
    }
}
```

**`InferCapabilitiesFromID` 标记 heuristic 来源**：

```go
func InferCapabilitiesFromID(modelID string) map[string]bool {
    // 原有逻辑不变，但调用方需要区分来源
    // 来源标记在调用方（enricher 或 init 阶段）完成
}
```

### C. InferModes 与 InferCapabilities 合并 + 反证黑名单

**目标**：统一两个函数的类型判定逻辑，并引入反证黑名单防止非 chat 模型被错误赋予 chat 能力。

**改动 1：合并 Vision 判断为共享 helper**

参考 A 方案，已实现 `IsVisionModelID` 单一真源。

**改动 2：新增反证黑名单**

```go
// infer.go — 新增
// unsupportedCapabilityPatterns lists model ID substrings whose models should
// NEVER receive the corresponding capability, even if the registry or provider
// reports it. This prevents specialty models (whisper, embedding, etc.) from
// optimistically inheriting tool-calling, vision, or reasoning through
// registry metadata that was written for a different family.
//
// Inspired by OmniRoute's TOOL_CALLING_UNSUPPORTED_PATTERNS / REASONING_UNSUPPORTED_PATTERNS.
var unsupportedCapabilityPatterns = map[string][]string{
    "function_calling": {
        "whisper",
        "tts-1",
        "omni-moderation",
        "moderation",
        "rerank",
        "embedding",
        "dall-e",
        "flux-",
        "stable-diffusion",
    },
    "vision": {
        // Future: known text-only models that the registry might mislabel as vision
    },
}
```

**改动 3：`applyCapabilityBlacklist` 函数**

```go
// infer.go — 新增
// ApplyCapabilityBlacklist removes capabilities that a model's ID
// identifies as an unsupported surface. Mutates the map in place and returns it.
func ApplyCapabilityBlacklist(modelID string, caps map[string]bool) map[string]bool {
    if caps == nil {
        return nil
    }
    id := strings.ToLower(strings.TrimSpace(modelID))
    for capability, patterns := range unsupportedCapabilityPatterns {
        if !caps[capability] {
            continue
        }
        for _, pattern := range patterns {
            if strings.Contains(id, pattern) {
                delete(caps, capability)
                break
            }
        }
    }
    return caps
}
```

**调用点**：在 `MergeMetadata` 结果最终确定后、或 `InferCapabilitiesFromID` 返回后，调用 `ApplyCapabilityBlacklist`。

---

## 4. 实施路线图

### 阶段 1：A 方案（单一真源）—— 1 小时

| 步骤 | 文件 | 内容 |
|------|------|------|
| 1.1 | `infer.go` | 新增 `IsVisionModelID(modelID string) bool` 函数 |
| 1.2 | `infer.go` | `InferModesFromID` 第 6 段改为调用 `IsVisionModelID` |
| 1.3 | `infer.go` | `InferCapabilitiesFromID` 改为调用 `IsVisionModelID` |
| 1.4 | `infer_test.go` | 验证所有 vision 用例不变（预期无变化） |

### 阶段 2：B 方案（来源可追溯）—— 2 小时

| 步骤 | 文件 | 内容 |
|------|------|------|
| 2.1 | `core/types.go` | `ModelMetadata` 新增 `CapabilitySources map[string]string` |
| 2.2 | `core/types.go` | 新增 `CapSrc*` 常量枚举 |
| 2.3 | `merger.go` | `buildMetadata` 中填充 `CapabilitySources` |
| 2.4 | `merge.go` | `MergeMetadata` 中合并 `CapabilitySources` |
| 2.5 | `enricher.go` | `Enrich` 中标记 discovered 来源 |
| 2.6 | `infer.go` | 启发式来源标记 |
| 2.7 | 测试 | 补充 merge/enrich 的来源追踪测试 |

### 阶段 3：C 方案（反证黑名单）—— 1 小时

| 步骤 | 文件 | 内容 |
|------|------|------|
| 3.1 | `infer.go` | 新增 `unsupportedCapabilityPatterns` 映射 |
| 3.2 | `infer.go` | 新增 `ApplyCapabilityBlacklist` 函数 |
| 3.3 | 调用点 | 3 处：`InferCapabilitiesFromID` 内部、`Enrich` 合并后、`applyInferredModelMetadata` 合并后 |
| 3.4 | `infer_test.go` | 补充黑名单测试用例 |

**实施记录（2026-09-06）**：A/B/C 三阶段均已实现并经过 `go test ./internal/modeldata/... ./internal/providers/...` 35 包全部 PASS 验证。变更一览：

- `infer.go`：新增 `IsVisionModelID`（A）、`unsupportedCapabilityPatterns`（C）、`ApplyCapabilityBlacklist`（C）
- `infer_test.go`：新增 `TestIsVisionModelID`（A）、`TestApplyCapabilityBlacklist`（C）
- `core/types.go`：`ModelMetadata` 新增 `CapabilitySources` 字段 + 4 个 `CapSrc*` 常量（B）
- `merger.go`：`buildMetadata` 中标记 registry 来源（B）
- `merge.go`：`MergeMetadata` 中合并 `CapabilitySources`（B）
- `enricher.go`：`Enrich` 中标记 discovered 来源 + 调用黑名单（B+C）
- `registry_metadata.go`：`applyInferredModelMetadata` 中标记 heuristic 来源 + 调用黑名单（B+C）
- `docs/design/model-capability-detection.md`：本文档同步更新

---

## 5. 与 OmniRoute 架构对比

| 维度 | OmniRoute | GoModel（当前） | 差距 |
|------|-----------|----------------|------|
| 模型来源 | 5 路（静态/同步/OpenRouter/自定义/专用注册表）+ combo | 3 路（registry/provider 上报/config） | 足够，不扩容 |
| 类型识别 | `classifyModelSupportedEndpoints()` + `getOpenRouterModelType()` + 专用注册表写死 | `InferModesFromID()` ID 启发式 | 主路径缺 endpoints → type 映射 |
| 能力探测 | 5 级优先级链（override → synced → registry → spec → heuristic） | 3 级（catalog → discovered → config） | 核心差距在**反证黑名单**和**来源可追溯** |
| 能力粒度 | `tool_calling`/`reasoning`/`vision`/`attachment`/`structured_output`/`temperature`/`effort_tiers` | 仅 `vision`（`map[string]bool`） | 未来扩展需先解决来源可追溯 |
| 保守策略 | 多个反证列表 + `KNOWN_TEXT_ONLY_DESPITE_SYNC` 硬编码覆盖 | 无 | 当前来源少，但规模放大后需要一个 |
| 单点判定 | `visionModels.ts` 唯一真源 + 3 处调用共享 | 2 处重复的 vision 判定 | **A 方案直接解决** |
| 来源可追溯 | `metadata.source` 对象（`providerRegistry`/`staticSpec`/`syncedCapability`/`reasoningEffortsOverride`） | 仅 `PricingSources` 有 | **B 方案直接解决** |

---

## 附录：相关文件索引

| 文件 | 行数 | 作用 |
|------|------|------|
| `internal/modeldata/infer.go` | 166 | 模型类型/能力的 ID 启发式判定 |
| `internal/modeldata/infer_test.go` | ~160 | 判定逻辑的测试用例 |
| `internal/modeldata/types.go` | 201 | 远程注册表的数据模型 |
| `internal/modeldata/fetcher.go` | 189 | 远程注册表的拉取/解析 |
| `internal/modeldata/merger.go` | 322 | 注册表查询与逐层合并 |
| `internal/modeldata/merge.go` | 104 | ModelMetadata 逐字段合并 |
| `internal/modeldata/enricher.go` | 66 | 注册表→provider 的 enrichment 协调 |
| `internal/core/types.go` | 522 | 核心数据模型（ModelMetadata 等） |
| `config/provider_models.go` | 93 | 配置声明的模型 metadata |