# metadata-only 写入保留定义 + 补齐 MongoDB 确认存储

- **日期**：2026-09-13
- **类型**：后端（`internal/admin` 的 upsert 契约、`internal/capability` 新增 MongoDB 存储）+ e2e 用例修订
- **关联**：[#52](https://github.com/LiTerBo/GoModel/issues/52)（本文件即其收口说明）；本文同时**修订 #55 的守卫口径**（见 §4）
- **承担**：CI 上两处红灯（`E2E Tests` 的 `TestAliasCRUD_E2E`、`Integration Tests` 的 MongoDB 用例），均已本地复现

## 1. 现象

**红一：E2E `TestAliasCRUD_E2E` 第 4 步**（`PUT {"source":"smart","locked":true}`）在我 #55 加守卫后由 204 变成 409 `virtual_model_kind_change`。该步骤注释写着「metadata-only change → 204」——即**只改元数据不该要求重述指向**，这是当初就记录下的设计意图。

**红二：`Integration Tests` 的 MongoDB 用例**全部失败在 `failed to create app: failed to initialize capability confirmations: mongodb handler is nil`。注意 issue #52 原描述是 ECR 拉取限流；实测 08:10 那次 run 里 `toomanyrequests` 出现 **0 次**、`mongodb handler is nil` 出现 **8 次** —— **原成因已被早先的镜像源改动解决**，剩下的是另一个真 bug。

## 2. 根因

**红一**：`PUT /admin/virtual-models` 用 `buildVirtualModelUpsert` **完全按请求体重建整行**。`enabled`/`locked` 用指针+`ResolveUpsert*` 实现了「省略即保留」，但**指向字段没有**：请求不带 `target_model`/`targets` 时 `Targets` 就变成空 —— 于是一条只改 lock 的写入会把别名降级成没有指向的访问策略，**静默丢掉 targets（连带 description/user_paths/strategy）**。这正是 #55 报的「定义被顶掉」的同一根因；#55 当时用 409 挡住了它，但把「只改元数据」这条合法路径也一并挡了。

**红二**：`internal/capability/factory.go` 里 `storage.ResolveSQLBackend[Store](...)` 的 MongoDB 位置传的是 `nil`，注释写着「MongoDB lands with a MongoDB store implementation」。`ResolveBackend` 遇到 nil 回调即报 `mongodb handler is nil` → `app.New` 失败 → **MongoDB 后端的部署根本起不来**（集成测试只是最先撞上）。

## 3. 修复

**3.1 upsert 契约（`internal/admin/handler_virtualmodels.go`）**

| 请求形态 | 结果 |
| --- | --- |
| 带指向（`target_model` 或非空 `targets`） | 整行替换（既有契约，`TestUpsertRedirectVirtualModelReplacesAccessPolicy` 明确钉住「不继承策略的 user_paths」） |
| **不提指向**（两者都没有，且非 `clear_targets`） | **metadata-only 补丁**：保留既有指向、strategy 系列、session_affinity/failover、description、user_paths、slowdown；只应用请求真的带了的字段 |
| 显式空指向（`targets: []`）且无 `clear_targets` | **409 `virtual_model_kind_change`**（#55 的守卫收窄到这一种） |
| `clear_targets: true` | 转为访问策略（行离开路由表），不再继承指向 |

实现：新增 `pointingOmitted` / `emptyPointingSent` 两个判定 + `inheritStoredDefinition`（只在「不提指向且非显式清除」时生效）；`Description` 由 `string` 改 `*string`（与 `Enabled/Locked` 同样的「省略=保留、显式空=清空」约定，注入层全走 JSON 故无破坏）；`kindChangeRejection` 签名改为接收请求体，判据换成 `emptyPointingSent`。

**3.2 MongoDB 确认存储（`internal/capability/store_mongodb.go`，新增）**

按仓库既有范式（`authkeys/store_mongodb.go`）实现 `Store` 接口：集合 `capability_confirmations` + `(provider, model, capability)` 唯一索引 + `model` 索引；`Upsert` 用 `ReplaceOne(upsert)`；`List` 按 provider/model/capability 升序；`Delete` 未命中返回 `ErrNotFound`；`Close` 无资源。`factory.go` 的 nil 位换成该构造函数，并复用既有的 `normalizeConfirmation` 校验。

## 4. 对 #55 口径的修订（重要）

#55 当初的判据是「存储行是 redirect 且请求不带指向 → 409」，理由是「别让写入把别名定义顶掉」。本轮证明这个判据**过宽**：它把「只改 lock/enabled 的元数据写入」也当成破坏性写入拒绝了（E2E 第 4 步即为此）。修订后：

- **默认行为变成「保留」而不是「拒绝」**：省略指向的写入保留既有定义 —— 报的缺陷（定义被顶掉）从此**不可能**发生，#55 的原始诉求以更强的方式满足；
- **409 只保留给显式空指向**（`targets: []`）这一种「看起来就是要清空」的写法，作为响亮的护栏；
- 前端不变：遮蔽行仍不显示上下架开关（那是 UI 层的正确性问题——开关写的是别名自己的 source），编辑器清空指向 / 行内「移除 redirect」仍发 `clear_targets`。

#55 的 5 个子用例随之改写为新契约（其中「行开关负载被拒」→「行开关负载保留指向」，「显式手势接管」保持不变），并新增 `handler_virtualmodels_metadata_write_test.go`（4 例：lock 保留定义、描述写入保留指向、显式空 user_paths 清空但保留指向、发送指向则替换）+ 策略行同场景 1 例（`{source, enabled}` 不再抹掉策略的 user_paths —— 这是同根因的第二个受害者：模型开关会把「仅 /team 可用」的限制悄悄解掉）。

## 5. 验证

- **Go 单测（`make test` 的集合）**：`cmd/config/ext/internal/run` 共 **97 包 ok / 0 FAIL**（exit 0）。
- **E2E 全套（`-tags=e2e ./tests/e2e/...`）本地实跑**：`ok 11.273s`（含修订后的 `TestAliasCRUD_E2E`；修订前该用例本地复现 409，修后 200 + 定义仍带 `test/gpt-4`）。
- **前端**：`npm test` **804/804**、`svelte-check` **0 error/0 warning**、`build` ok（本次只改了 `vm_kind_change_blocked` 的文案）。
- **Mongo 实测（真容器，已完成）**：本机 `docker run -d --name gm-mongo-cap -p 27019:27017 mongo:7` 起真 MongoDB 后，`MONGO_TEST_DSN=mongodb://127.0.0.1:27019 go test ./internal/capability/ -v` 的 **7 条 Mongo 存储子用例全 PASS（非 SKIP）**；`tests/integration` 全套 `ok 36.4s`，其中 **6 条 MongoDB 后端用例 PASS / 0 FAIL / 0 SKIP**。核对时必须用 `-v` 逐条看，否则 SKIP 也会显示 ok。
- **CI 复核**：推送后的 `CI` run（提交 `637091c7`）**10/10 job `success`** —— `Integration Tests` 与 `E2E Tests` 首次转绿，`lint` 与 `Docs Validation` 的历史旧账也在同一轮清完。
- **限流结论（#52 的镜像来源/限流验收项）**：`toomanyrequests` 不是成因 —— CI 日志中该串出现 0 次，且 Integration job 的镜像拉取实际成功（失败发生在测试内 `mongodb handler is nil`）。套件本身已带预拉取 + 4 次退避重试（`tests/integration/docker_test.go`），CI 侧无任何镜像代理配置（`.github/workflows/test.yml`），故**镜像来源保持 `public.ecr.aws/docker/library/*` 不变**即满足该项。本机曾拉不动是本机 Docker daemon 走代理（大 blob EOF）所致，与 ECR 配额无关。
- **反向验证**：`TestUpsertVirtualModelMetadataOnlyWrite` 的 4 个用例在回到旧实现时会全部失败（lock 后 pointed 为空、description/user_paths 被抹），即断言是可证伪的。

## 6. 边界（本次未做）

- **`target_model: ""` 无法与「省略」区分**（JSON 字符串字段）：想清空指向请用 `targets: []` 或 `clear_targets`，文档已写明。
- **`enabled`/`locked` 之外的布尔省略语义不同**：`session_affinity`/`failover` 省略即默认开启（既有设计），只在 metadata-only 分支里才从存储行继承 —— 这是本轮的实现边界，未改动它们原有的三态约定。
- **MongoDB 确认存储只做了接口对等**，未做「SQL 与 Mongo 行为逐条比对」的跨后端一致性测试（现有 `store_sql_test.go` 与新增 `store_mongodb_test.go` 各自覆盖同一组语义）。