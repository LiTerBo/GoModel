# 别名演进护栏（T13）：影响面预览 + 审计与来源标记 + 锁定守卫

- **日期**：2026-09-12
- **类型**：后端（1 个只读 admin 端点 + `virtual_models` 新增 1 列 + 迁移 + 管理事件）+ 前端（编辑器保存前预览、锁定开关、白名单来源标记）+ 文档；**无** `/v1/*` 变化、**无**鉴权接口签名变化
- **状态**：设计已裁决（issue [#44](https://github.com/LiTerBo/GoModel/issues/44) 的 Q1–Q5，2026-09-12）；实施待 TDD；分支待开 `feat/alias-evolution-guardrails`

## 1. 背景与痛点

「按虚拟模型名授权」（issue #36 / PR #38）把**别名本身**变成了授权单位：密钥策略里写 `smart`，就能调用 `smart` 当前及将来的全部目标。代价是 **D2**：

> 别名**改指向**（编辑 target 列表）或**新增目标**时，按名授权该别名的密钥的**实际可用范围随之变化**，而密钥策略一字未改，控制台也看不出「谁正按这个名字授权」。

### 1.1 实测事实（决策依据，均已核对代码）

| 事实 | 位置 |
| --- | --- |
| 可见性判定：先按名字（`allowName(source)` 命中即暴露），否则回退「至少一个 leaf 的 selector 被允许」 | `internal/virtualmodels/resolve.go:127-171`（`exposedModels`，名字分支 :145、回退分支 :147-152） |
| 反查是**纯内存 O(n)**：密钥与路径策略都在内存快照里 | `authkeys.Service.ListViews()`（`snapshot.byID/order`）、`users.Service.List()`（`snapshot.byPath/order`） |
| admin Handler **已同时持有三个服务**，反查零接线成本 | `internal/admin/handler.go:43-45` |
| 别名定义持久化在 `virtual_models` 表，**加列有现成惯例** | `internal/virtualmodels/store_sql.go:18-49`（`sqlSchema` + 5 条 `ALTER TABLE ADD COLUMN` 先例） |
| 别名删除**只校验 source 非空**，无引用检查 | `internal/admin/handler_virtualmodels.go:146-166` |
| 审计子系统有「安全事件记录器」先例 | `ext/auth.go:47-76`（`AuthenticationEvent` / `AuthenticationEventRecorder`）+ `internal/auditlog/authentication_events.go` + 绑定于 `internal/app/init_server.go:251` |

### 1.2 D2 有**两个受害者**，影响面必须分三类

| 受害方 | 判定路径 | 改指向后的后果 | 数量变化 |
| --- | --- | --- | --- |
| **按名授权** | 名字谓词命中（名字分支） | 别名**仍在**，但**背后模型换了** → 静默跟随（能力/价格/合规随之变） | **不变** |
| **目标级授权** | 回退分支（至少一个 leaf 被允许） | 新增一个「被允许的」target → 别名**新可见**；移除最后一个被允许的 target → 别名**消失** | **±1** |

结论：只报「范围扩大/缩小」对按名授权的 key 是**误导**（它数量不变）。护栏必须区分 **follow / gained / lost** 三类，否则提示本身就不可信。

## 2. 需求（验收标准）

| 编号 | 需求 | 可验证方式 |
| --- | --- | --- |
| FR-1 | 别名编辑器保存前可查看**影响面**：受影响的密钥与用户路径，按 follow / gained / lost 分组，并给出依据（名字命中，或命中的目标选择器） | 端点返回 + 前端预览块；粗粒度语义在测试中锁定 |
| FR-2 | 只读端点 `GET /admin/virtual-models/authorized-by?source=<name>` | 端点测试（含 400/404 边界） |
| FR-3 | **锁定守卫**：`locked=true` 的别名改指向必须显式解锁，否则拒绝 | 端点测试：locked 且未解锁 → 409；解锁后 → 200 |
| FR-4 | 允许模型字段中，**别名条目带「跟随别名」标记**，悬停显示其当前指向 | dashboard 测试（标记函数 + 悬停文案） |
| FR-5 | 删除别名时，存在按名授权引用则**无 `force` 拒绝并回传影响清单** | 端点测试：无 force → 409 + 清单；`force` → 204 |
| FR-6 | 改指向与删除写**管理事件**（source、old/new targets、locked、受影响计数、是否 force、操作者） | 事件写入断言 |
| NFR-1 | **授权语义零变更**：不改变 `AllowsModel`/可见性判定；老 key、老条目行为逐项不变 | 既有 users/virtualmodels/admin 用例全绿 |
| NFR-2 | 反查**纯内存**，不新增存储扫描路径 | 实现只读快照；评审检查 |
| NFR-3 | 不改 `/v1/*` 响应结构、不改鉴权接口签名、不引入新选择器语法 | diff 检查 |
| NFR-4 | 新增文案 en / zh-CN 双语 | i18n key 集一致 |

## 3. 设计

### 3.1 影响面计算（粗粒度：零新增解析能力）

输入：`source`（别名名）、`old`（已存 target 选择器集合）、`new`（提案 target 选择器集合）、`grants`（既有密钥的 `allowed_models` + 既有用户路径策略的 `allowed_models`）。

判定规则（镜像 `resolve.go:127-171`，但**不解析提案**）：

1. **名字命中**：`users.MatchesName(entry, source)` 为真 → `change=follow`；若该路径/密钥受 `user_paths` scope 排除则不列（与可见性同口径）。
2. **目标级命中**：`entry ∩ (old ∪ new)` 非空 → `change=potential`，`reason` 列出命中的选择器。**不下 ±1 结论**（那需要精确预览，见 §6）。
3. `old == new`（未改指向）时仍可查询，用于「谁在跟随这个别名」的只读盘点。

输出结构（草案）：

```json
{
  "source": "smart",
  "locked": false,
  "old_targets": ["openai/gpt-4o"],
  "new_targets": ["openai/gpt-4o", "anthropic/claude-sonnet"],
  "grants": [
    {"kind": "credential", "id": "hms", "label": "hms key", "change": "follow",
     "matched_by": "name", "selectors": ["smart"]},
    {"kind": "user_path", "path": "/acme", "change": "potential",
     "matched_by": "selector", "selectors": ["openai/"]}
  ],
  "summary": {"follow": 1, "potential": 1}
}
```

落点：`internal/admin/impact.go` —— 纯函数，输入为「已被投影的 grant 列表 + 选择器集合」，不反向依赖 `virtualmodels`（admin 已 import `users` / `virtualmodels` / `authkeys`，无循环风险）。

### 3.2 端点契约

| 项 | 值 |
| --- | --- |
| 方法/路径 | `GET /admin/virtual-models/authorized-by?source=<name>`（与既有 `/virtual-models` 不带 `:id`、`source` 走 query/body 的风格一致；`/auth-keys/:id/*` 那套不适用） |
| 鉴权 | 与其它 `/admin/*` 相同（`global` scope） |
| 400 | `source` 缺失/空白 |
| 404 | 别名不存在 |
| 200 | §3.1 结构；`new_targets` 缺省 = 当前定义（只读盘点） |
| 可选参数 | `new_targets`（重复参数或逗号分隔）——保存前预览用；缺省即当前值 |

### 3.3 存储变更（锁定位）

- 列：`locked`（SQLite/PG 用 `sqlx.TypeBool NOT NULL DEFAULT FALSE`；Mongo `bson:"locked,omitempty"`）。
- 迁移：`virtualModelMigrations` 追加 `ALTER TABLE virtual_models ADD COLUMN locked ... DEFAULT FALSE`；`selectVirtualModelColumns` 与 `upsertVirtualModelSQL` 列清单同步。
- 语义：**锁定只防误操作**，不改变授权语义 —— `locked=true` 时改指向需要显式 `unlock=true`（或先关锁再改）。锁定状态本身可读写（`View` 暴露 `locked`）。

### 3.4 审计（D6 待裁决）

| 方案 | 做法 | 代价 |
| --- | --- | --- |
| **D6-a（推荐）** | 沿用安全事件先例：`ext` 新增管理事件 + `auditlog` recorder + `app` 绑定（与 `AuthenticationEventRecorder` 同构） | 3 处新增文件/接线 + handler 调用；事件可被审计查询/控制台消费 |
| D6-b | 只记服务日志（`log.Printf` 级），不新增事件类型 | 最便宜；但审计查询与前端看不到，等于 Q1c 只做了一半 |

### 3.5 前端

| 落点 | 内容 |
| --- | --- |
| `web/dashboard/src/pages/models/VirtualModelEditor.svelte` + `virtualModelEditor.svelte.js` | 保存前**影响面预览块**（调 `authorized-by`，按 follow/potential 分组，空则提示「无按名授权方」）；`locked` 开关；锁定时改指向需先解锁（前端守卫 + 后端 409 兜底） |
| `web/dashboard/src/pages/auth-keys/AuthKeyAllowedModelsEditor.svelte`（共享 `SearchSelect`） | 别名条目加「跟随别名」标记；悬停显示当前指向（数据来自既有的 `virtualModels.aliases` 懒加载，未加载时**降级为不标记**，不阻塞输入） |
| i18n | 新增 key 落在既有前缀：`virtual_models_*`（预览/锁定/删除守卫）与 `api_keys_allowed_models_*`（跟随标记），en 与 zh-CN 同批 |

### 3.6 决策记录

| 编号 | 决策 | 理由 / 代价 |
| --- | --- | --- |
| D1 | 护栏范围 = **(a) 影响面预览 + (c) 审计与来源标记** | 用户裁决；两者共用同一次反查，边际成本低 |
| D2 | 精度 = **粗粒度优先**（选择器交集），精确预览（follow/+1/−1 与新旧指向对比）**后补** | 精确预览需要新增「按提案定义试算」能力（现有解析全部基于已存定义）；先上能回答「谁受影响」的版本 |
| D3 | 锁定 = **(i) 显式解锁守卫**（1 列 + 迁移 + Upsert 守卫），**不做**「按名授权钉在快照的真锁定」 | 真锁定需在解析/鉴权路径区分调用方来源并引入别名修订概念，与「别名=授权单位」冲突；D2 的真实风险源是误操作 |
| D4 | 来源标记 = **逐条标记 + 悬停显示当前指向** | 用户裁决；需给共享 `SearchSelect` 加一个**可选**标注钩子（其它使用点不受影响） |
| D5 | 删除 = **无 `force` 时 409 + 影响清单** | 复用 D1 的反查能力；`force` 路径仍写审计（FR-6） |
| D6 | 审计落地形态：**a（新增管理事件类型）** vs b（仅日志） | **待裁决**；推荐 a，理由是 b 无法被审计查询/控制台消费，等于 Q1c 半成品 |
| D7（实现者定） | 端点路径 `GET /admin/virtual-models/authorized-by?source=` | 与既有路由风格一致；已在 §3.2 写明，用户可否决 |

### 3.7 明确不改

- 授权判定（`AllowsModel` / `MatchesName` / `exposedModels`）与 `/v1/*` 响应结构；
- 选择器语法（不引入 `vm/x` 之类新命名空间）；
- 别名重名/遮蔽规则（仍属「待裁决」项，见 issue #44 遗留）；
- 别名条目 `metadata` 的能力/定价取值规则（独立议题）。

## 4. 任务清单

- [x] **T13-A** 影响面纯函数（`internal/admin/impact.go`）：表驱动单测（名字命中 / 目标级命中 / scope 排除 / old=new 盘点 / 空 grants）
- [x] **T13-B** 端点 `GET /admin/virtual-models/authorized-by`：400/404/200 边界 + `new_targets` 预览参数
- [x] **T13-C** `locked` 列 + 迁移 + Store 双实现 + Upsert 守卫（locked 且未解锁 → 409）；迁移+双写测试
- [x] **T13-D** 删除守卫：无 `force` 且存在按名授权引用 → 409 + 影响清单；`force=true` → 204
- [x] **T13-E** 管理事件（D6 裁定后落地）+ 写入断言
- [x] **T13-F** 前端：编辑器预览块 + 锁定开关 + `SearchSelect` 标注钩子（跟随标记/悬停指向）+ i18n 双语 + dashboard 测试
- [x] **T13-G** 文档同步：`docs/advanced/admin-endpoints.mdx`（新端点/新参数）、`docs/features/users.mdx`（跟随语义与标记）、`docs/features/virtual-models.mdx`（锁定/预览/删除守卫）、本文件勾选
- [x] **T13-H** 端到端实测：`go test -tags=e2e -run TestAliasCRUD_E2E` 通过 7 步全链（创建/列表/authorized-by/锁定/解锁改指向/删除/验证已删）

## 5. 验证证据

_待实施后填写（要求：测试计数、全量门禁、CI 结论、端到端输出）。_

## 6. 风险与后续

| 风险 | 说明 | 现状 |
| --- | --- | --- |
| 粗粒度预览的**语义边界** | `change=potential` 不等于真的 ±1（需精确预览），UI 文案必须写成「可能受影响」而非「将失去」 | 设计已限定措辞；精确预览见下 |
| 精确预览（后补） | 需要「按提案定义试算」能力（临时 `VirtualModel` + 解析），属独立增量 | 登记为后续 |
| 锁定是**防误操作**，不防恶意 | 有 admin 权限者仍可 `unlock` 后改指向（审计留痕即可） | 接受 |
| 前端标记依赖别名已加载 | 懒加载未触发时降级为不标记（不阻塞输入） | 接受，测试覆盖降级分支 |
| 删除守卫的 `force` 路径 | 必须仍写审计（含 force 标记），否则等于开了后门 | FR-6 覆盖 |
| 能力/定价元数据不准确 | 别名条目 `metadata` 仍来自代表目标 | 独立议题，未在本设计内 |

## 7. 事后修订（2026-09-13）

- **守卫范围**：FR-5/D5 的删除守卫只适用于 **redirect（有 targets 的别名）**。实现原先对**访问策略行**（无 targets）也生效，而策略行在 `classifyGrantImpact` 下必然命中「无白名单」持有人，导致任何部署里的单个模型「下架后无法上架」。已在 `DeleteVirtualModel` 收窄，并补两条回归用例。
- **提示语言**：FR-5 的 409 文案由控制台按 `code` + `authorized-by` 计数渲染（后端 message 保持英文），前端原先读错返回信封（`body` 而非 `data`），强制删除确认链路实际不可达。
- 详见 `docs/dev/2026-09-13_model-unpause-guard-scope/修复说明.md` 与同目录 `实现复盘.md`。
