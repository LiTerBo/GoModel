# 遮蔽别名的写入口收口：跨 kind 写入需显式手势

> **2026-09-13 修订**：本文描述的守卫判据（「请求不带指向即 409」）**已收窄**——见
> `docs/dev/2026-09-13_metadata-only-upsert-and-mongo-confirmations.md` §4。现在省略指向的写入
> **保留**既有定义（默认不再拒绝），409 只保留给显式空指向 `targets: []`。前端部分（遮蔽行无开关、
> 两处清理发手势）不变。

- **日期**：2026-09-13
- **类型**：后端（`PUT /admin/virtual-models` 新增 `clear_targets` 手势 + 409 机器码）+ 前端（遮蔽行不再渲染上下架开关；两处合法的清理路径补发手势）
- **关联**：issue #55（本文件即其收口说明）；相邻改动见 `2026-09-13_model-list-delete-force-entry.md`（删除守卫的 force 入口）

## 1. 现象

**遮蔽别名（masking alias）**：一条重定向的 `source` 与某个具体模型行同名（`openai/gpt-4o-mini` 这类 `provider/model` 形状）。仪表盘上该模型行同时渲染了 **上下架开关** 与 **编辑 redirect** 铅笔。

点那个开关会发 `PUT /admin/virtual-models {source, enabled:false}`。`source` 是存储主键、`Upsert` 按整行替换，于是这条重定向被**替换成一条访问策略**：`kind` 由 `redirect` 变为 `policy`，`targets` 与 `strategy` 一起清空 —— 别名定义丢失，且**没有任何提示**。

## 2. 真机实测复现（隔离实例，管理 API）

```text
PUT  {source:"tmp-shadow", target_model:"mock/tiny-a"}          -> 200
读回 {kind:"redirect", targets:[{model:"mock/tiny-a"}], strategy:"round_robin", enabled:true}

PUT  {source:"tmp-shadow", enabled:false}      # 开关发出的正是这个负载
读回 {kind:"policy", targets:null, strategy:null, enabled:false}
```

证据等级：从 issue 建立时的「存储层单测探针」升级为「真机管理 API 实测」。

## 3. 改动

**后端**：把「跨 kind 替换」从隐式行为改成需要显式手势（`kindChangeRejection` 与既有 `lockRejection` 同形，紧邻放置）。

| 文件 | 改动 |
| --- | --- |
| `internal/admin/handler_virtualmodels.go` | 请求体新增 `clear_targets`（文档注明用途）；在 lock 守卫之后加 `kindChangeRejection(stored, vm, clearTargets)`：**仅当** 存储行是 redirect、且新写入没有 targets 时才拦；错误 `virtualModelKindChangeError` 走 409 + `code=virtual_model_kind_change` + `param=clear_targets`（后端 message 保持英文）；swagger 注释同步 |
| `internal/admin/errors.go` | 新增 409 构造 `virtualModelKindChangeError(source)` |
| `internal/admin/handler_virtualmodels_test.go` | 两条既有用例（移除 redirect 保留策略字段、空表单丢弃 no-op 行）补发 `clear_targets: true` —— 它们正是「有意清除」的合法路径 |

**前端**：遮蔽行不再提供开关；两处合法清理补发手势；新码按目录渲染中文。

| 文件 | 改动 |
| --- | --- |
| `src/pages/models/displayRows.js` | 新增纯函数 `rowAccessToggleVisible(row)`：遮蔽行（`!is_alias && masking_alias`）不显示开关，别名行与普通行照旧 |
| `src/pages/models/ModelRow.svelte` | 非别名分支的 `AccessToggle` 由该函数门控 |
| `src/pages/models/virtualModels.svelte.js` | `toggleRowEnabled` 加同一门控兜底（陈旧点击/程序化调用 → 中文提示，不写盘）；`removeRedirectRow` 的负载补 `clear_targets: true`；`virtualModelFailureText` 把 source 传给码→文案映射 |
| `src/pages/models/vmForm.js` | 新增纯函数 `withRedirectClearGesture(payload, {isRedirect, wasRedirect})`：仅「存储行是 redirect 且本次写入没有指向」时补 `clear_targets`，不改动入参 |
| `src/pages/models/virtualModelEditor.svelte.js` | 新增打开态 `vmFormStoredRedirect`（`openVirtualModelEditAlias` 置真、`resetVirtualModelForm` 复位）；保存前应用手势；错误渲染传 source |
| `src/pages/models/vmImpactPreview.js` | `virtualModelErrorText(result, source)` 支持新码；新增 `kindChangeBlockedText(source)` |
| `messages/en.json`、`messages/zh-CN.json` | 新增 `vm_kind_change_blocked`（同位置同序） |

**语义边界**：redirect→redirect（改指向/改名）不需要手势；policy→redirect（升级）不需要手势；redirect→policy（移除指向）需要手势。配置型（config.yaml / `VIRTUAL_MODELS`）虚拟模型走 `SetConfigModels` 合并路径，不经 `Upsert`，因此声明式定义不受影响。

## 4. 验证

- **Go**：新增 `TestUpsertVirtualModelRedirectTakeoverNeedsGesture`（5 个子用例：开关负载被拒且行保持 redirect / 带手势可接管 / redirect→redirect 放行 / 无存储行的策略不受影响 / policy→redirect 放行）；先红后绿（红=200，绿=409）。两条既有用例随契约更新。`go test ./internal/admin/ ./internal/virtualmodels/ -count=1` ok。
- **前端**：新增 `tests/vm-masking-alias-guard.test.js` 6 条 —— 3 条纯函数（开关可见性、手势构造不改入参、新码渲染的中文句子且不泄漏后端英文）、3 条源码契约（`ModelRow` 用 `rowAccessToggleVisible` 门控、`removeRedirectRow` 带 `clear_targets`、编辑器记住 stored redirect 并在复位时清标志）。门禁 `npm test` 794/794（基线 782，+6 force 流 +6 本项）、`npm run check` 0 error/0 warning、`npm run build` ok。

## 5. 未覆盖 / 遗留

- **CLI 或第三方脚本直接 PUT**：现在会拿到 409 `virtual_model_kind_change`，需按文档补 `clear_targets`（这是刻意的破坏性变更，已写入 `docs/advanced/admin-endpoints.mdx`）。
- 遮蔽行「移除 redirect」按钮的端到端点击未做浏览器实测（本轮验证用管理 API + 源码契约；浏览器实测的配方见 `gomodel-feature-delivery` §8）。
- 相邻的守卫与 force 两条后端路径仍只有单测覆盖，e2e 覆盖并入 #55 的回归项清单。
