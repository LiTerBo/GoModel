# 模型列表行删除命中守卫后提供强制入口

- **日期**：2026-09-13
- **类型**：前端（模型页行操作与别名编辑器共用一条守卫决策）；无后端改动、无新增接口、无新增 i18n 键
- **关联**：issue #57（本文件即其收口说明）；守卫范围与提示本地化的背景见 `2026-09-13_model-unpause-guard-scope/修复说明.md`

## 1. 现象

模型列表（`admin/dashboard/models`）点别名行的垃圾桶：

1. 弹出第一层确认框「移除虚拟模型别名 “{source}？”」；
2. 确认后命中删除守卫（`409 virtual_model_in_use`）；
3. 只弹一条中文提示「「{source}」仍可被凭据或用户路径调用，需要强制删除。」——**该页面没有任何强制删除入口**，提示让操作者「需要强制删除」却不给路走。

同样的删除动作在**别名编辑器**里却有第二层 force 确认框（接受后带 `force:true` 重发）。两条入口行为不一致，用户在列表页被卡住（issue #57）。

## 2. 根因

- `src/pages/models/virtualModels.svelte.js` 的 `mutateVirtualModelRow`：`!result.ok` 时只 `flash.error(...)` 后 `return`，既不带 `force` 重发，也不弹第二层确认框；force 链路只写在 `src/pages/models/virtualModelEditor.svelte.js`。
- 于是同一条守卫决策在两条入口各写一份，行为漂移：列表页「有提示无入口」，编辑器「有入口」。
- 顺带发现两处同源问题：编辑器的强制重入会**再次**询问第一层 policy confirm（`deleteVirtualModel()` 入口的 confirm 未判断 `vmDeleteForcePending`），全接受时会出现三次框；强制的重发若仍失败，`vmDeleteForcePending` 不复位，下一次删除会跳过守卫确认直接带 `force`。

## 3. 改动

| 文件 | 改动 |
| --- | --- |
| `src/pages/models/vmImpactPreview.js` | 新增纯函数 `deleteForcePlan(result, { forcePending, source, impact, payload })`：仅当 `409 + code=virtual_model_in_use + 未强制` 时返回 `{ confirmMessage, retryPayload }`（重发负载 = 原负载 + `force: true`），其余一律返回 `null`（调用方按错误渲染） |
| `src/pages/models/virtualModels.svelte.js` | `mutateVirtualModelRow` 改为**最多两次发送**：普通一次 → 命中守卫则弹 `deleteBlockedConfirm` 句子 → 接受则带 `force:true` 重发；拒绝则显示中文提示「…需要强制删除」。新增 `deleteGuardPlan()`（只在确实是 in-use 守卫时才拉取清单，普通失败不额外请求）与 `fetchDeleteImpact()`（`GET /admin/virtual-models/authorized-by`，两条入口共用同一份实现） |
| `src/pages/models/virtualModelEditor.svelte.js` | 409 分支改用同一个 `deleteForcePlan`；删除本文件里的 `fetchImpactInventory`（改为调用 store 的 `fetchDeleteImpact`）；强制重入跳过第一层 policy confirm；强制的重发失败时复位 `vmDeleteForcePending` |
| `tests/vm-delete-force-flow.test.js`（新增 6 条） | 纯函数 4 条：只对 in-use 守卫给计划、计划一次性（`forcePending` 后为 `null`）、其它 409/非 409 为 `null`、重发负载保留原字段且不改动调用方对象、句子取自目录而非后端英文散文；源码契约 2 条：列表行走同一条两段式、编辑器强制重入不重复询问 |

i18n 复用既有键（`vm_delete_blocked_confirm`、`vm_delete_blocked_confirm_unknown`、`vm_delete_blocked_notice`），无新增文案；后端零改动。

## 4. 验证

- 门禁：`npm test` **788/788**（基线 782，本次 +6）；`npm run check` 0 error / 0 warning；`npm run build` ok；`go test ./internal/admin/ ./internal/server/ -count=1` ok
- **隔离实例浏览器实测**（独立端口 18098 + 独立 sqlite/cache/pid 文件 + 本地 mock 上游；全程未触碰运行中的 8080 实例）：

| 场景 | 第一层 | 第二层 | 结果 |
| --- | --- | --- | --- |
| 列表行 · 有持有人 · 接受 | 「移除虚拟模型别名 “tmp-force-b”？」 | 「仍被使用 ——「tmp-force-b」可被 3 个凭据、0 条用户路径调用。仍要删除（强制）吗？」 | 行消失 → **列表页新入口生效** |
| 列表行 · 有持有人 · 拒绝 | 同上 | 同上（选择取消） | 行保留，提示「「tmp-force-c」仍可被凭据或用户路径调用，需要强制删除。」 |
| 编辑器 · 有持有人 · 接受 | 「移除 “tmp-force-f” 的虚拟模型？这将恢复为继承/默认行为。」 | 「仍被使用 ——「tmp-force-f」可被 1 个凭据、0 条用户路径调用。仍要删除（强制）吗？」 | **恰好 2 层**（无第三次重复询问），行消失 |

- 计数一致性：两处渲染的「N 个凭据 / M 条用户路径」与 `GET /admin/virtual-models/authorized-by` 的 grants 逐项一致（3 凭据 / 1 凭据各自吻合）。

## 5. 顺带复核 #55（真机层面）

在隔离实例上用管理 API 复核「同名访问策略写入顶掉 redirect」：`PUT {source: "tmp-shadow", target_model: "mock/tiny-a"}` 建立 redirect（`kind=redirect, targets=[{mock/tiny-a}]`），随后发送行开关同款负载 `PUT {source: "tmp-shadow", enabled: false}`，读回变为 `kind=policy, targets=null, strategy=null` —— redirect 定义被清空。#55 的证据由此从「存储层单测探针」升级为「真机管理 API 实测」；修复方向不变（前端遮蔽行不渲染上下架开关／后端跨 kind 加护栏）。

## 6. 未覆盖

- 列表行与编辑器的强制链路仍只有**单测 + 源码契约**级别的自动化；真实点击验证本次是手工脚本（隔离实例），未固化为 e2e 用例（与 #55 的回归项合并跟踪）。

