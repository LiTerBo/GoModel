# RFC: Dashboard 显示层币种换算(CNY)

- 状态:**已裁决,暂缓实现**(2026-09-05 朱磊裁决:采纳方案 C,Q2–Q4 均按推荐)
- 关联:issue [LiTerBo/GoModel#1](https://github.com/LiTerBo/GoModel/issues/1)(dashboard i18n 修订跟踪)
- 背景:dashboard 中文界面已上线(en/pl/zh 各 1403 键),但成本/预算/定价全部以美元展示,对中文用户不友好。

## 1. 问题与实测事实

中文 locale 下,所有金额仍显示 `$` 前缀。代码证据:

| 事实 | 位置 |
|---|---|
| 定价数据美元原生:8 处 provider 代码硬编码 `Currency: "USD"`;定价覆盖 API 文档明确 "USD-only pricing" | `internal/providers/*.go`、`internal/admin/handler_model_pricing_overrides.go:26,48` |
| 用量记录只存裸 float(`total_cost`),无币种字段 | `internal/usage/reader_mongodb.go:37` |
| 预算金额是无单位 float,与美元成本直接做比值 | `internal/budget/types.go:51,152` |
| 前端 45 处调用 3 个格式化函数,全部硬编码 `$` 前缀 | `web/dashboard/src/lib/utils/format.js`(`formatCost`/`formatPrice`/`formatPriceFine`) |
| 消息目录含 USD 表述:`models_pricing_help`("Currency is USD")、`usage_openrouter_cost_source`、`overview_calendar_cost_year` 的 `${total}` 模板 | `messages/en.json` |
| 全栈无汇率基础设施(换算/配置/缓存均无) | 实测 grep 零命中 |

**关键架构判断**:成本真相源是美元——供应商按 USD 报价、OpenRouter 按 USD 信用额度扣费、操作员抄录美元价目。这是数据语义问题,不是前端格式问题。

## 2. 红蓝军对抗结论(2026-09-05)

- **蓝军(保持美元+标注)**:成本是合同性数据,换算引入 ~1% 汇率浮动误差,预算比对与供应商账单对不上。弱点:体验不友好,治标不治本。
- **红军(全栈多币种)**:usage 加 currency 字段、按录入币种存储。弱点:真相源仍是 USD,存 CNY 只改变换算时机,却引入 schema 迁移、预算比值重写、历史数据兼容——数人月,过度设计。
- **裁决:方案 C(显示层换算)**——存储与 API 保持 USD 真相源,显示与录入层换算。

## 3. 方案 C 设计(待实现)

### 3.1 核心原则

存储与 API 不变(USD);**单一换算咽喉在前端格式化层**;汇率来自配置而非实时 API;换算值诚实标注。

### 3.2 显示层

- 改造 `formatCost` / `formatPrice` / `formatPriceFine`(`src/lib/utils/format.js`):
  - locale 为 `zh` 时按配置汇率换算,货币符号 `¥`,小数位规则同步调整;
  - 小额阈值同步换算(`< $0.0001` → `< ¥{0.0001 × rate}`,保留 4 位有效);
  - 45 处调用点自动生效,不逐点改动;
  - locale 为 `en`/`pl` 时维持现状(美元)。
- 图表坐标轴、tooltip 走同一格式化函数,自动一致。
- 消息目录修正:`overview_calendar_cost_year`/`overview_calendar_cost_on_date` 的 `${total}` 模板改为传已格式化字符串,避免模板内嵌美元符号。

### 3.3 汇率来源(裁决 Q2:启动配置 + 设置页可改)

- 后端新增配置项(如 `display.fx_rate`,环境变量 `GOMODEL_DISPLAY_FX_RATE`),默认 `1.0`(即不换算);
- 经 `/admin` 运行时配置接口暴露,Settings 页可改(遵循"简单配置留设置页"惯例);
- **不做实时汇率 API**:无外部依赖、无失败模式、无缓存策略问题;
- 语义:1 USD = fx_rate CNY。fx_rate ≤ 0 视为配置错误,回落 1.0。

### 3.4 诚实标注

- 换算显示值带「≈」前缀(如 `≈¥7.20`),或 tooltip「按配置汇率 7.2 换算,实际以美元账单为准」(具体形式实现时定,倾向 tooltip 减少视觉噪音);
- `usage_openrouter_cost_source` 等成本来源消息在 zh 目录补充"美元账单"语境说明。

### 3.5 预算录入(裁决 Q3:录入人民币,存储换算为 USD)

- BudgetEditor 录入人民币金额(界面标注 ¥);
- 提交时 `amount_usd = amount_cny / fx_rate` 存储后端;
- 列表/详情显示时乘回 `formatCost(amount_usd × fx_rate)`;
- 操作员全程只见人民币,`Spent/Amount` 比值计算口径一致;
- 改动点:`budgets.svelte.js` 提交/加载两处换算,BudgetEditor 标注。

### 3.6 定价覆盖编辑器(裁决 Q4:保持 USD 录入)

- `PricingOverrideEditor` 录入界面**不做换算**,保持 USD;
- 理由:操作员抄录的是供应商美元价目表,强制换算反而刁难;页面已有 "USD Value" 字段与 "Currency is USD" 帮助文案;
- 该页帮助文案在 zh 目录明确「此处按美元录入」。

## 4. 实现清单(暂缓,启动时按此执行)

1. `format.js` 三函数改造 + locale 判断(复用 `$lib/i18n/locale.js` 的 `getLocale`);
2. 后端 `display.fx_rate` 配置项 + `/admin` 暴露(Go 侧小改);
3. Settings 页汇率编辑项;
4. `budgets.svelte.js` 录入/显示换算;
5. 消息目录:zh 补充 ≈ 标注、USD 语境说明;en/pl/zh 同步(同 commit);
6. 测试:`format.js` 换算单测(locale=zh、fx_rate 边界)、预算换算单测;
7. 验证:`npm test` + `npm run check` + `npm run build`,dist 产物确认。

预计 2-3 天。无后端存储改动、无 schema 迁移。

## 5. 明确不做

- 后端多币种存储 / usage 表 currency 字段(schema 迁移不值);
- 实时汇率 API 与缓存;
- 非 zh locale 的换算显示(en/pl 维持美元);
- 定价覆盖录入的换算(保持 USD)。
