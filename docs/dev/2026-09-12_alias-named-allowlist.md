# API 密钥按「模型名（别名）」授权 —— 需求 + 设计 + 任务清单

> 日期：2026-09-12
> 类型：后端鉴权语义增强（语义改动 1 处；前端 0 改动、无新 API、无存储变更）
> 状态：**后端已实施**（分支 `feat/alias-named-allowlist`；issue [#36](https://github.com/LiTerBo/GoModel/issues/36)）
> 变更规模：12 个文件 +207/−19，新增 5 个测试文件 + 1 个服务端用例

## 1. 背景与痛点

API 密钥的 `allowed_models` 按**解析后的目标 selector** 判定，带来三个后果：

1. **写别名名永远匹配不到**：白名单里写 `smart` 的请求直接 `400`，`/v1/models` 对受限 key 只列具体供应商模型 → 使用方必须同时理解「真实模型」和「别名」两套命名。
2. **泄露别名背后的目标**：别名条目克隆目标元数据（`owned_by=omlx`、`created=目标时间`），使用方能推断出真实模型与供应商，且该元数据随目标可用性**漂移**。
3. **多目标别名非确定性失败**：负载均衡 / adaptive / failover 链的别名只允许部分目标时，请求按目标轮转概率 `400`（实测 12 次中 4 次 400），且失败随权重变化。

**目标**：授权单位 = **模型名（别名）**。使用方只看被授权的名字，不必知道它背后是哪些具体模型、由哪个供应商提供、有几个目标。

## 2. 需求（验收标准）

1. 白名单写别名名 → 该 key 的 `GET /v1/models` **只含该别名**，且条目不携带目标供应商/时间痕迹；
2. 白名单写别名名 → 调该别名**恒 200**（与目标数量/权重无关），调其他模型恒 `400 model_access_denied`；
3. 老 key（`provider/` 通配、具体目标清单、影子别名）**行为与展示完全不变**；
4. 无白名单的常见部署路径**零额外开销**（热路径）。

## 3. 设计

### 3.1 匹配语义：层内 OR、层间 AND

`AllowsModel(selector)` 的判定改为按「请求名 ∨ 解析后 selector」：

- **层内（单个名单）**：`MatchesName(allowed, name) || Matches(allowed, selector)` —— 任一命中即该名单放行；
- **层间（凭证层 / userPath 及各祖先层）**：仍全部必须放行（交集，子层不能放宽父层）；
- 匹配规则：条目为 `/`（全局）或**与请求名完全相等**即为名字命中；`provider/*`、`provider/model`、裸模型 ID 仍走解析后匹配（需要 provider 维度），`*` 不是规范条目（规范全局形是 `/`）。

### 3.2 可见性：按源名暴露，且用显式谓词

`virtualmodels.exposedModels` 的别名暴露条件从「存在任一被允许叶子」改为「**源名被授权**」（叶子授权退为兜底，保证老 key 不变）。

实现为 `ExposedModelsForUserPathNamed(userPath, allow, allowName)`，而**不是**复用通用 `allow` 谓词去探名字 —— 因为 `allow` 是调用方的不透明谓词，对「名字形 selector」（`{Model: 源名}`）做判定会骗过字段形状谓词（实测 `sel.Provider != "groq"` 这类谓词会放行探针，直接打红既有 `TestChain_ExposedModelsProjectLeafMetadata`）。老方法签名保持不变，`allowName=nil` 即旧行为。

### 3.3 元数据归一

别名条目只归 `owned_by`（置空）与 `created`（取别名自身创建时间）。`metadata`（modes/categories）仍来自代表目标 —— adaptive 多链路别名的能力很可能不等于任何单目标，报「代表目标」等于撒谎，因此本次**刻意不归一**（见 §6）。

### 3.4 请求名管道

`core.WithRequestedModelName/GetRequestedModelName`：优先读显式戳，缺失时回退读 ctx 内 workflow 的 `Requested.Model` —— 因此 **chat 主路径零改动**，只有自解析路径各注入 1–2 行：

| 路径 | 注入点 | 请求名来源 |
| --- | --- | --- |
| chat（主路径） | 无（走 workflow 回退） | `Requested.Model` |
| 会话内解析（model_call_service / realtime_service / passthrough_service） | `prepare` | 客户端给的名字 → 同时写入返回 ctx，供 failover 目标过滤使用 |
| 批处理（batch_selection） | 逐 item | `requested.Model` |

### 3.5 决策记录

| # | 决策 | 理由 |
| --- | --- | --- |
| D1 | 用「请求名」而非新增 `vm/` 前缀语法 | 别名名直接可用，零新语法、零存储变更 |
| D2 | 名字命中 ⇒ 该请求全部目标视为已授权 | 授权与目标轮转解耦，这正是消除非确定 400 的机制；代价是「改指向即自动扩权」（护栏见 §6） |
| D3 | 可见性用显式 `allowName` 参数 | 通用谓词不能对合成 selector 做名字判定（见 3.2） |
| D4 | 老方法/老接口签名不变，新能力走附加接口（`NamedUserPathExposedModelLister`） | 调用方与既有测试零影响 |
| D5 | 无白名单 + 无 userPath 的早返回提到函数最前 | 该函数在请求路径上对每个候选 selector 调用；抽出的 `allowsModel` 不做闭包，避免每次调用一次堆分配 |
| D6 | 元数据只归 `owned_by`/`created` | 能力/定价归一需要独立规则，本次不做（§6） |

## 4. 任务清单

- [x] T1 语义核心：`internal/users/selectors.go` 新增 `MatchesName`；`internal/users/service.go` 改层内 OR / 层间 AND + 热路径早返回
- [x] T2 请求名管道：`internal/core/context.go` 新增 ctx helper；4 处自解析路径注入（chat 主路径零改动）
- [x] T3 可见性：`internal/virtualmodels/resolve.go` 新增 `ExposedModelsForUserPathNamed` + `exposedModels` 名字分支；`service.go` 接口断言
- [x] T4 管理端贯通：`internal/server/exposed_model_lister.go` 附加接口；`internal/server/models_endpoint.go` 优先走 Named 版本
- [x] T5 元数据归一：别名条目 `owned_by=""`、`created=`别名自身创建时间
- [x] T6 测试：5 个新测试文件 + `internal/server/models_endpoint_test.go` 服务端级用例（真跑 auth → users → virtualmodels 组合）
- [x] T7 验证：受影响包全绿；全量门禁 99 包 ok；`tests/perf` 既有基线失败经 `git stash -u` 对照确认与本次改动无关
- [x] T8 端到端：临时实例（独立端口 + 独立缓存目录）+ mock 上游 + 临时 key，验证 §2 的 3 条验收标准
- [x] T9 文档：本文件 + `docs/features/users.mdx`、`docs/config-manual.md`、`docs/advanced/admin-endpoints.mdx` 语义反转
- [x] T10 登记：issue #36
- [ ] T11 前端：`admin/dashboard/auth-keys` 与 Users 页可选清单加入虚拟模型（后端能力已就绪）
- [ ] T12 admin `effective_models` 反映名字授权（否则 key 行显示与真实授权不自洽）
- [ ] T13 别名演进护栏：改指向时提示影响面 / 可选锁定目标集合（对应 D2 的代价）
- [ ] T14 上游失败错误文本归一化（`model_access_denied` 的 message 目前直接透传上游形态）

## 5. 验证证据

**单元/服务端测试**（新增 6 个测试函数、17 条用例通过）：

`TestMatchesName`、`TestService_AllowsModelByRequestedName`、`TestExposedModels_AuthorizedNameAliasOnly`、`TestExposedModels_NoSiblingLeakByName`、`TestRequestedModelNameContext`、`TestListModels_AliasNamedAllowlistSeesOnlyThatAlias`

**受影响包**：`core / users / virtualmodels / server / gateway / admin / authkeys` 全部 `ok`。
**全量门禁**：`go test ./... -count=1` → 99 包 ok；唯一 FAIL `tests/perf/TestHotPathPerfGuard` 在改动前的树上（`git stash -u`）数字**逐项相同**（94/92、114/103、116/104、105/103、175/161）→ 既有基线失败。

**端到端**（临时实例 18081 + mock 上游 18099 + 临时 key，非生产库）：

```
[alias key: allowed_models=["smart"]]
  GET /v1/models        -> 200, ids=['smart']              # 验收 1
  smart entry           -> owned_by='' created=1789221526   # 无目标痕迹
  chat smart            -> 200                              # 验收 2
  chat sibling          -> 400 model_access_denied
  chat mock/tiny-a      -> 400 model_access_denied
  GET /v1/models/smart  -> 200
[legacy key: allowed_models=["mock/"]]
  GET /v1/models        -> ['mock/tiny-a','mock/tiny-b','sibling','smart']
  chat mock/tiny-a      -> 200                              # 验收 3
稳定性（双目标别名）: chat smart x12 -> {200: 12}            # 旧语义实测 8/12
```

## 6. 风险与后续

| 风险 | 说明 | 现状 |
| --- | --- | --- |
| 改指向即扩权 | 名字授权后，别名新增/更换目标不改变该 key 的可用范围（D2 的代价） | 接受现状；T13 提供护栏 |
| 能力元数据不准确 | 别名条目 `metadata` 仍来自代表目标，adaptive 别名可能报错能力 | 已知，待独立规则 |
| admin 显示不自洽 | key 行 `effective_models` 仍只列具体模型 | T12 |
| 既有性能预算失败 | `tests/perf` 在改动前即失败 | 建议单独登记 issue，避免污染全量门禁判断 |
