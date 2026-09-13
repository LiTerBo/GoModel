# 机器码本地化映射层：控制台从 `error.code` 渲染句子

- **日期**：2026-09-13
- **类型**：前端（`$lib/api/errors.js` 单表 + 覆盖测试）
- **关联**：[#54](https://github.com/LiTerBo/GoModel/issues/54)；上游缺陷集见 `2026-09-13_model-unpause-guard-scope/修复说明.md` §5-1
- **边界**：后端所有面向机的错误文案**永久英文**（AGENTS.md），本层只做前端渲染

## 1. 现象

控制台设为中文时，绝大多数管理/网关错误提示仍是英文。当时 `internal/` 里真实存在的机器码 **24 个**（issue 写 22，另加一个是有意保留的误差，详见 §4），前端只有 2 个走目录文案（`virtual_model_locked`、`virtual_model_in_use`，另有 `dashboard_access_denied`/`feature_unavailable` 按页面特判）。

## 2. 根因

没有「码 → 文案」的映射层：错误展示点各自读 `error.message`（英文原文），个别页面自己 `if (code === ...)` 特判，于是新增码永远不会被本地化，已有的两个也散落在三个文件里。

## 3. 设计（单表 + 单一提取）

`web/dashboard/src/lib/api/errors.js` 成为唯一汇聚点（它本来就是「管理 API 错误提取」模块，且被 `client.js` 再导出给组件/store）：

1. **`apiErrorCode(result)`**：从 `{ok, stale, status, data, res}` 信封或裸负载中取 `error.code`。原先定义在 `vmImpactPreview.js`，现迁到本模块并由它 `export`，**避免两处提取**（迁移时踩到的坑见 §5）。
2. **`CODE_TEXT` 单表**：`code → { text(params), detail? }`。三个虚拟模型码复用既有键（`vm_lock_change_blocked`/`vm_delete_blocked_notice`/`vm_kind_change_blocked`），不复制文案；其余 **21 个**新键 `apierr_<code>`（en/zh-CN 同序各 21 条）。
3. **`apiErrorText(result, params)`**：命中即返回目录句子，返回 `""` 表示「本层不管」——调用方随即回退英文 `message`。**永不自带兜底**，这样英文原文永远可达。
   - 需要上下文参数的码（两个会点名行名的虚拟模型码）在缺参时返回 `""`，避免渲染出空洞句子。
4. **`detail` 规则**：服务端 `message` 里带具体信息（计数、选择器、上游原文、存储错误）的码标 `detail: true`，渲染成「中文句子 + 英文细节」；句子已自足的码不追加（否则等于重复一遍）。
5. **接线点**：`errorMessage`（信封）、`errorPayloadMessage`（裸负载，playground 用）改为「目录优先、英文兜底」；另外三个自己读 `.error.message` 的实时失败面（SSE 流内错误、tagging 设置 PUT、会话抽屉追问）同样接入。
6. **覆盖测试** `tests/api-error-i18n.test.js`：正则扫 Go 源码（`WithCode("...")` 与 `code = "..."` 两种写法，跳过 `_test.go`）得到盘点，双向断言——**后端有码必须有句子**（漏一个即红）、**句子不得比后端多**（删码后残留即红），另有「两语言都非空且不相同」（防止 zh 静默回落英文）、参数码必须点名、`detail` 码必须保留细节、未映射码必须保留英文。

## 4. 盘点结果（24 个码）

| 组 | 码 |
| --- | --- |
| 请求生命周期 | `request_canceled`、`request_timeout` |
| 模型访问 | `model_access_denied`、`model_not_found` |
| 限流/预算 | `rate_limit_exceeded`、`rate_limit_check_failed`、`rate_limit_not_found`、`budget_exceeded`、`budget_check_failed`、`budget_not_found`、`usage_status_failed` |
| 上游 | `provider_error` |
| 部署/授权/身份 | `feature_unavailable`、`quota_templates_not_entitled`、`extension_authentication_failed`、`dashboard_access_denied`、`user_managed`、`runtime_setting_not_found`、`auth_key_issue_failed`、`metadata_max_properties_exceeded`、`unsupported_response_operation` |
| 虚拟模型 | `virtual_model_locked`、`virtual_model_in_use`、`virtual_model_kind_change` |

`provider_error` 是**盘点测试抓出来、人工 grep 漏掉**的一个：它不在 `WithCode(...)` 里，而是 `internal/providers/responses_converter.go` 在 Responses 流失败时写入 `error.code` 的兜底值（解析上游 `code`/`type` 都为空时）。已补句子（`detail: true`，保留供应商原文）。`dashboard_access_denied` 同理只在 `internal/server/auth.go` 以 `code = "..."` 常量出现。

证据等级：盘点由测试在执行时实时扫描源码（不是抄来的清单）。

## 5. 踩坑（都已被测试钉住）

- **`export { x } from "..."` 不产生本地绑定**：把 `apiErrorCode` 迁走后原模块自身调用它 → `apiErrorCode is not defined`（3 个 `deleteForcePlan` 用例同时红）。必须 `import { x } from ...` + `export { x };`。
- **两者契约不同，不能顺手统一**：`errorMessage` 宽容（`data.message`／`data.error` 字符串／`data.error.message`），`errorPayloadMessage` 严格（只认 `{error:{message}}`）——既有用例明确钉住 `{error:"flat"}` → 兜底。统一后立刻打红 `errorPayloadMessage falls back on any other shape`；改为两个提取器，各自保持原契约。
- **node:test 不认 `$lib` 别名**：参与 node 测试的模块（`errors.js`、`vmImpactPreview.js`、`playgroundLogic.js`、`tagging-logic.js`）之间的 import 必须用相对路径。
- **测试的路径基数**：`tests/` 在 `web/dashboard/tests`，Go 源码在三层之上（`../../..`）。

## 6. 验证

- **前端**：`npm run i18n:compile` ok；`npm test` **804/804**（基线 794，本项 +10）；`npm run check` 0 error/0 warning；`npm run build` ok。
- **覆盖断言（可证伪）**：故意留一个后端码不写句子、或删一个码后留着句子，第 2/3 条用例即红；把 zh 键删掉（回落到 en）第 4 条即红。
- **后端**：未改任何 Go 代码（本层纯前端），但覆盖测试**读**Go 源码，故码表变化会立刻反映到前端门禁。

## 7. 未做的边界（有意）

- **审计日志的历史错误不回渲染**：`pages/audit-logs/error-text.js` 从**已落库的请求/响应体**里抽文本，展示的是「当时记录的内容」，故意保持原样（含当时的英文 `message` 与 `code`）。若要让历史记录也按码本地化，需要单独决策（涉及「历史不可变」的展示语义）。
- **后端文案不动**：`/v1/*`、`/admin/*`、日志一律英文（AGENTS.md 明令）。
- **`EXEMPT` 表为空**：24 个码全覆盖，没有豁免项；测试仍保留该出口，供将来「只在测试里出现的码」使用。
