# GoModel 配置说明手册（中文）

> 对应源文件：`config/config.example.yaml`（本项目 fork 时点版本）。所有配置项都有内置默认值，**不写任何配置文件也能直接运行**；本文件是「每个键是什么、怎么组合、坑在哪」的说明。
>
> 配套文档：[安装部署指南](install-deploy-guide.md)（构建/部署）、`docs/guides/production.mdx`（生产环境建议，英文）。

---

## 0. 配置机制（先读懂再动手）

### 0.1 配置来源与优先级

```
内置默认值（代码内）  →  config/config.yaml（可选）  →  .env（可选）  →  导出的环境变量（最高）
```

- **不需要完整配置文件**：`config.yaml` 只写你想覆盖的键即可（分层合并，写多少生效多少）。
- **环境变量永远压过文件里的值**；`.env` 只填充「真实环境里没设置」的变量。
- 环境变量**空值会被跳过**（无法用空值覆盖默认）；个别键支持 `off` 哨兵来「关闭」（见 9.1）。

### 0.2 文件位置与相对路径语义

| 文件 | 位置 | 说明 |
| --- | --- | --- |
| 示例模板 | `config/config.example.yaml`（仓库内，勿直接改） | 702 行全量注释 |
| 你的配置 | **`config/config.yaml`**（gitignored） | 首次使用：`cp config/config.example.yaml config/config.yaml`，再删改 |
| 环境文件 | `.env`（gitignored，进程 CWD 下） | 放 key/secret 等敏感项 |

> ⚠️ **相对路径一律以网关进程的 CWD 为基准**（不是配置文件所在目录）：`config/config.yaml` 的查找、`.env`、`cache.model.local.cache_dir`、`models.json` 本地目录、`data/` 下的 SQLite/pid 文件、`server.pid_file` 全是如此。`make run` 在仓库根执行所以 CWD=仓库根；换目录启动、systemd/Docker 部署时必须改用容器内路径或绝对路径。

### 0.3 生效与校验

- 配置在**启动时加载一次**；修改后需重启。
- `gomodel --reload`（类 `nginx -s reload`）：重新加载全部配置并平滑生效，**新配置无效时进程继续用旧配置服务**。前提：配置了 `server.pid_file`（见 1）。
- `CONFIG_STRICT`（默认 `true`）：config.yaml 中出现未知键 → **启动直接报错拒绝**，防止拼写错误静默失效。调试时可 `CONFIG_STRICT=false` 降级为警告。

### 0.4 敏感信息放哪

`master_key`、各供应商 `api_key` 不要明文写进会提交的 yaml。推荐：

- 值引用环境变量：`api_key: "${OPENAI_API_KEY}"`（支持 `${VAR}` 与 `${VAR:-default}` 展开）；
- 或全部放进 gitignored 的 `.env` / 进程环境变量。

---

## 1. server — 网关服务

| 键 | 默认 | 环境变量 | 说明 |
| --- | --- | --- | --- |
| `port` | `"8080"` | — | 监听端口（字符串） |
| `base_path` | `"/"` | `BASE_PATH` | 挂载前缀；`/g` 表示从 `https://example.com/g/` 下服务 |
| `master_key` | — | （见 .env） | 管理员主密钥，保护 `/admin/*` |
| `body_size_limit` | `"10M"` | — | 请求体上限 |
| `swagger_enabled` | `false` | `SWAGGER_ENABLED` | 需要 `-tags=swagger` 构建的二进制 |
| `pprof_enabled` | `false` | — | `/debug/pprof/*`，仅本地排查用 |
| `enable_passthrough_routes` | `true` | — | 暴露 `/p/{provider}/{endpoint}` 直通路由 |
| `allow_passthrough_v1_alias` | `true` | — | 允许 `/p/{provider}/v1/...`（与无 `v1` 的规范形式并存） |
| `user_path_header` | `X-GoModel-User-Path` | `USER_PATH_HEADER` | 入站请求头，用于 user_path 维度隔离（限流/预算/权限） |
| `enabled_passthrough_providers` | 见注释 | — | 允许走 `/p/{provider}/...` 的供应商名单 |
| `realtime_enabled` | `true` | `REALTIME_ENABLED` | `/v1/realtime*` WebSocket 及 `/p/{provider}/v1/realtime` 升级 |
| `pid_file` | `data/gomodel.pid` | `PID_FILE` | `gomodel --reload` 靠它找进程；多实例同机必须各自独立路径；空=不写 pid 文件且禁用 reload；**改它需重启而非 reload** |

```yaml
server:
  port: "8080"
  base_path: "/"
  master_key: "${GOMODEL_MASTER_KEY}"   # 引 .env 或环境变量
  pid_file: "data/gomodel.pid"          # 启用 --reload 的开关
```

## 2. models — 模型可见性

| 键 | 默认 | 环境变量 | 说明 |
| --- | --- | --- | --- |
| `enabled_by_default` | `true` | `MODELS_ENABLED_BY_DEFAULT` | `false` 时所有模型默认不可用，需在 users/access 中显式放行 |
| `keep_only_aliases_at_models_endpoint` | `false` | `KEEP_ONLY_ALIASES_AT_MODELS_ENDPOINT` | `/v1/models` 只列启用的虚拟模型，隐藏供应商模型 |
| `unqualified_model_ids_at_models_endpoint` | `false` | `UNQUALIFIED_MODEL_IDS_AT_MODELS_ENDPOINT` | 列裸 ID（`gpt-5`）而非 `openai/gpt-5`；两个供应商同名时只列第一个，重名冲突用虚拟模型改名钉住 |
| `configured_provider_models_mode` | `"fallback"` | `CONFIGURED_PROVIDER_MODELS_MODE` | 供应商配置了 `models:` 时的策略，见下表 |

`configured_provider_models_mode` 三档：

| 值 | 行为 |
| --- | --- |
| `fallback`（默认） | 上游 `/models` 不可用/为空时才用配置清单 |
| `allowlist` | 只暴露配置的模型，配置了清单的供应商跳过上游 `/models` |
| `merge` | 在上游清单之上追加配置的模型（补上游没列的） |

## 3. tagging — 请求头打标签

从入站请求头提取标签，记录进用量与审计日志；标签可用于预算维度（见 8）。

```yaml
tagging:
  headers:
    - header: X-My-Tags          # 值 "tag-alpha, beta" →
      prefix: "tag-"             #   提取标签 "alpha"、"beta"
    - header: X-Internal-Routing
      do_not_pass: true          # 剥离，不再转发给供应商
      delimiter: ";"             # 默认 ","
```

- 标签预算按**原文匹配**，无 env 形式（见 8.2）。
- 声明在此或 env 的入口在 Dashboard 只读；**省略本节则完全交给 UI 管理**（Settings → Tagging）。

env 等价写法（同 header 名的 env **整体替换** yaml 条目，其余 env 条目追加）：

```bash
TAGGING_HEADER_1=X-My-Tags
TAGGING_HEADER_1_PREFIX=tag-        # 可选
TAGGING_HEADER_1_DONOTPASS=true     # 可选，默认 false
TAGGING_HEADER_1_DELIMITER=";"      # 可选，默认 ","
```

## 4. session — 会话识别

识别同一客户端会话（供虚拟模型会话亲和与审计分组）。以下全为默认，**整节省略=同样行为**：

```yaml
session:
  enabled: true
  auto_detect: true
  builtin_rules: true              # 内置 known-tools 注册表
  headers:
    - header: X-My-Session         # 额外头；transform "session-uuid" 可提取 session_<uuid>
```

## 5. virtual_models — 虚拟模型（路由策略即代码）

四种形态：**重定向别名 / 负载均衡 / 成本路由 / 故障转移链**。声明在此或 `VIRTUAL_MODELS` env（JSON 数组，按 source 逐条合并且 env 胜出）的条目会**覆盖同名 admin 行**且在 Dashboard 只读；省略本节则完全用 UI 管理。

| 字段 | 说明 |
| --- | --- |
| `source` | 对外暴露的模型名 |
| `target` / `targets[].model` | 目标：`provider/model`，**也可以是另一个虚拟模型**（链式，无环、最深 8 层） |
| `targets[].weight` | 加权轮询权重 |
| `slowdown` | 别名延迟加成：0.5 = 加 50%；0 可取消继承的 slowdown（有效 0.1–10） |
| `strategy` | `round_robin`（默认）\| `cost` \| `failover` \| `adaptive`（注册了路由扩展时用，否则回落 round_robin） |
| `session_affinity` | 已识别的会话钉在服务过它的目标上 |
| `failover` | 请求失败后在其余目标重试；`false` 则直接返回错误 |

```yaml
virtual_models:
  - source: smart          # 加权轮询
    strategy: round_robin
    targets:
      - { model: openai/gpt-4o, weight: 2 }
      - { model: anthropic/claude-sonnet-4-6 }
  - source: cheap          # 永远路由到最便宜
    strategy: cost
    targets: [{ model: openai/gpt-4o }, { model: groq/llama-3.3-70b }]
  - source: gpt-4o         # failover：先试真模型再按序回退
    strategy: failover
    targets:
      - { model: gpt-4o }
      - { model: azure/gpt-4o }
      - { model: gemini/gemini-2.5-pro }
```

## 6. users — 用户路径放行（即代码）

按 user_path 树配模型白名单；「组和用户 = API key 背后的 user path」。声明在此或 `USERS` env（JSON 数组，按 path 合并、env 胜出）的条目覆盖 Dashboard 同 path 行并只读。

```yaml
users:
  - path: /acme
    allowed_models: [openai/*, anthropic/*]   # 供应商通配 / provider/model / 裸 ID
  - path: /acme/eng                           # 子路径在父白名单内继续收窄
    allowed_models: [anthropic/*]
    description: Engineering only ships on Claude
```

规则：节点不写 `allowed_models` = 全部模型可用；一旦写了，该路径下的请求被限制在本节点**及所有父节点**列表的交集内。虚拟模型先解析成目标再匹配——**列目标，别列虚拟模型名**。

## 7. mcp — MCP 网关

把多个上游 MCP 服务器聚合在带鉴权的 `/mcp` 后面。工具/提示词以 `{server}_{name}` 命名空间暴露；`/mcp/{server}` 暴露单个上游的原始名字。**网关是凭据边界**：客户端 key 永远到不了上游；上游 headers 支持 `${ENV}` 引用。

| 键 | 说明 |
| --- | --- |
| `enabled` | env `MCP_ENABLED`（默认 true；无服务器时无效果） |
| `allowed_origins` | 允许调 `/mcp` 的浏览器 Origin，**默认空=拒绝浏览器**（防 DNS rebinding）。只有你确实从某网页服务 MCP 客户端才加；`"*"` 关闭检查 |
| `servers.<name>.url` | 上游地址 |
| `servers.<name>.transport` | `http`（streamable HTTP，默认）\| `sse`（旧）\| `stdio`（子进程） |
| `servers.<name>.headers` | 如 `Authorization: "Bearer ${GITHUB_PAT}"` |
| `allowed_tools` / `disallowed_tools` | 白名单（空=全放）/ 黑名单（在白名单后应用） |
| `user_paths` | 限制可见 user_path 子树；空=所有人 |
| `tool_timeout` | 单次 `tools/call` 上限，如 `30s` |

```yaml
mcp:
  enabled: true
  servers:
    github:
      url: https://api.githubcopilot.com/mcp
      headers: { Authorization: "Bearer ${GITHUB_PAT}" }
    local-files:                # stdio 只允许声明式（admin API/UI 拒绝此类服务器）
      transport: stdio
      command: npx
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/data"]
      env: { SOME_TOKEN: "${SOME_TOKEN}" }   # 子进程只继承 PATH/HOME/TMPDIR/USER/LANG，
                                             # 额外 token 必须在这里显式传
```

声明在此或 `MCP_SERVERS` env（JSON 对象，按名合并、env 胜出）的服务器在 Dashboard 只读；Dashboard 自己另管的服务器不受影响。

## 8. cache — 模型与响应缓存

### 8.1 cache.model — 模型注册表

| 键 | 默认 | 环境变量 | 说明 |
| --- | --- | --- | --- |
| `refresh_interval` | `3600` | — | 注册表刷新间隔（秒） |
| `recheck_interval` | `60` | `PROVIDER_RECHECK_INTERVAL` | 上次刷新失败的供应商多久重探一次恢复（秒；0=禁用） |
| `model_list.url` | GitHub 官方目录 | `MODEL_LIST_URL` | 见 8.1.1 |
| `local.cache_dir` | `.cache` | — | 本地缓存目录（相对 CWD）；配了 Redis 但连不上时自动降级用本地 |
| `redis.url/key/ttl` | — | — | 配了 Redis 则优先；建议保留 local 兜底降级启动 |

**8.1.1 model_list.url 三种取值**（详见下文「模型目录」专节）：

- HTTP(S) URL —— 默认 `https://raw.githubusercontent.com/ENTERPILOT/ai-model-list/refs/heads/main/models.min.json`；
- **本地文件路径**（`/etc/gomodel/models.json`、`file://...`、裸路径）—— 内网/离线安装用，每次刷新重读文件，内容 sha256 去重；
- **`off`** —— 彻底关闭目录下载。

```yaml
cache:
  model:
    refresh_interval: 3600
    local:
      cache_dir: ".cache"
    # redis:                       # 配了 Redis 优先，留 local 兜底
    #   url: "redis://localhost:6379"
    #   key: "gomodel:models"
    #   ttl: 86400
```

### 8.2 cache.response — 响应缓存

- `simple`（精确命中缓存）：整块省略即关闭（除非 `RESPONSE_CACHE_SIMPLE_ENABLED=true`）；`enabled: false` 可在保留块的情况下关闭。可配 Redis。
- `semantic`（语义缓存，ADR 0006）：需要 **embedder**（指向 `providers` 下已注册的供应商+模型）+ **vector_store**（四选一：`qdrant` / `pgvector` / `pinecone` / `weaviate`）。整块省略即关闭（除非 `SEMANTIC_CACHE_ENABLED=true`）。向量维度必须与 embedding 模型输出一致。

```yaml
cache:
  response:
    simple:
      enabled: true
      redis: { url: "redis://localhost:6379", key: "gomodel:response:", ttl: 3600 }
    semantic:
      enabled: true
      embedder: { provider: openai, model: text-embedding-3-small }
      vector_store:
        type: qdrant          # qdrant | pgvector | pinecone | weaviate
        qdrant: { url: "http://localhost:6333", collection: gomodel_semantic }
```

## 9. 模型目录（model list）专节

网关从目录获取各模型的**定价、上下文窗口、能力**元数据，用于成本追踪与路由决策。

| 键 | 环境变量 | 说明 |
| --- | --- | --- |
| `cache.model.model_list.url` | `MODEL_LIST_URL` | 三种取值见下 |

取值：

1. **HTTP URL（默认）**：官方目录在 GitHub raw。每次刷新走 ETag 条件请求（304 免重下），下载内容落到本地/Redis 缓存，断网时用缓存继续服务。**国内/内网网络常被限速断流**（症状：启动 WRN `failed to fetch model list ... EOF`）——功能本身不受影响，只是元数据退化为「缓存 + 供应商自报 + 手工配置」。
2. **本地文件**：任意路径或 `file://`。适合离线/内网：先在能联网的机器 `curl -o models.json <官方URL>` 下载一次，再指向它。每次刷新重读，未变化（sha256）则跳过解析。可配合 `offline: true`（见 20）双保险。
3. **`off`**：完全关闭下载。注意 **`MODEL_LIST_URL=""` 空值不能关闭功能**（空 env 被跳过），必须用 `off` 哨兵。

```yaml
cache:
  model:
    model_list:
      url: etc/models.json      # 相对 CWD；或绝对路径 / 文件系统路径
      # url: off                # 彻底关闭
```

> 升级目录内容无需重启：到 `refresh_interval` 即自动重读/重拉。

## 10. storage — 持久化

| 键 | 默认 | 说明 |
| --- | --- | --- |
| `type` | `"sqlite"` | `sqlite` \| `postgresql` \| `mongodb`（审计日志/用量等） |
| `sqlite.path` | `data/gomodel.db` | 相对 CWD |
| `postgresql.url` / `max_conns` | — / 10 | 连接串 + 连接池上限 |
| `mongodb.url` / `database` | — / `gomodel` | |

## 11. logging — 审计日志

| 键 | 默认 | 说明 |
| --- | --- | --- |
| `enabled` | `true` | 总开关 |
| `log_bodies` | `true` | ⚠️ 请求/响应体可能含敏感数据 |
| `log_revision_bodies` | `true` | 记录改写后的请求体（依赖 log_bodies） |
| `log_audio_bodies` / `log_image_bodies` | `false` | 音频/图片字节入库（依赖 log_bodies）；图片可再配 `log_image_bodies_scope: all\|input\|output` |
| `log_headers` | `true` | |
| `buffer_size` / `flush_interval` | `1000` / `5` | 批量落库缓冲（条）/ 冲刷间隔（秒） |
| `retention_days` | `30` | 0 = 永久保留 |
| `only_model_interactions` | `true` | 只记模型交互（省空间） |

## 12. usage — 用量与成本

| 键 | 默认 | 说明 |
| --- | --- | --- |
| `enabled` | `true` | 用量统计（另需 `USAGE_ENABLED=true` 与受支持的存储后端；旧环境变量形式仍认） |
| `pricing_recalculation_enabled` | `true` | 成本重算（需 usage.enabled 同时开启才出现） |
| `enforce_returning_usage_data` | `true` | 强制供应商回传 usage |
| `buffer_size` / `flush_interval` / `retention_days` | `1000` / `5` / `90` | 同日志缓冲语义 |

## 13. budgets — 预算（按 user_path 与标签）

> `per_child` 标注为 GoModel Pro 功能。没配任何预算时 `enabled: true` 无实际效果（env `BUDGETS_ENABLED`）。

```yaml
budgets:
  user_paths:
    - path: "/user/path/example"
      per_child: false        # Pro：true 时每个直接子路径独立预算
      limits:
        - period: "daily"     # hourly | daily | weekly | monthly（DB 存 period_seconds）
          amount: 10.00
        - period: "weekly"
          amount: 50.00
  labels:                     # 标签预算按原文匹配，无 env 形式
    - label: "Mobile-App-iOS"
      limits: [{ period: "monthly", amount: 500.00 }]
```

env 等价（`__` 表示路径分隔层级）：

```bash
SET_BUDGET_USER__PATH__EXAMPLE="daily=10,weekly=50"
```

## 14. rate_limits — 限流（三层：用户路径 / 供应商 / 模型）

| 键 | 默认 | 环境变量 | 说明 |
| --- | --- | --- | --- |
| `enabled` | `true` | `RATE_LIMITS_ENABLED` | 无规则时无效果 |
| `flush_interval` | `1` | `RATE_LIMITS_FLUSH_INTERVAL` | 周期快照秒数；0=不周期快照（仍保留加载与关停时落盘） |

`period` 取值：`minute` / `hour` / `day` / `concurrent`；`max_tokens` 需要用量统计开启（`USAGE_ENABLED=true`）。

```yaml
rate_limits:
  user_paths:
    - path: "/user/path/example"
      limits:
        - { period: "minute", max_requests: 100, max_tokens: 50000 }
        - { period: "day", max_requests: 10000 }
        - { period: "concurrent", max_requests: 10 }   # 在途请求上限
  providers:                    # 一个供应商整体上限（跨所有消费方）
    - name: "openai"
      limits: [{ period: "minute", max_requests: 500, max_tokens: 200000 },
               { period: "concurrent", max_requests: 50 }]
  models:                       # 单模型上限；无 env 形式
    - model: "openai/gpt-4o"    # 裸 ID 如 "gpt-4o" = 跨所有供应商同名模型
      limits: [{ period: "minute", max_tokens: 90000 }]
```

env 等价：`SET_RATE_LIMIT_USER__PATH__EXAMPLE="rpm=100,tpm=50000,rpd=10000,concurrent=10"`、`SET_PROVIDER_RATE_LIMIT_OPENAI="rpm=500"`。

关键联动：虚拟模型负载均衡/failover 会**跳过已饱和的供应商**；所有目标都满才回 429。

## 15. metrics / opentelemetry — 观测

```yaml
metrics:
  enabled: false
  endpoint: "/metrics"
```

OpenTelemetry：YAML 镜像标准 `OTEL_*` 变量；**环境变量优先于 YAML**，更细的设置（按信号端点、超时、压缩等）直接走 `OTEL_*`。

```yaml
opentelemetry:
  enabled: false
  service_name: "gomodel"
  # resource_attributes: { deployment.environment: production }
  endpoint: "http://localhost:4318"     # protocol grpc 时默认 localhost:4317
  protocol: "http/protobuf"             # 或 "grpc"
  # headers: { authorization: "Bearer ${OTEL_BACKEND_TOKEN}" }
  traces_exporter: "otlp"               # 或 "none"
  metrics_exporter: "otlp"
  sampler: "parentbased_always_on"      # 例：parentbased_traceidratio + sampler_arg "0.1"
  propagators: "tracecontext,baggage"   # 另：b3, b3multi, jaeger, ottrace, none
```

## 16. http / workflows — 连接与定时

| 段 | 键 | 默认 | 说明 |
| --- | --- | --- | --- |
| `http` | `timeout` | `600` | 上游请求总超时（秒，10 分钟） |
| `http` | `response_header_timeout` | `600` | 响应头超时（秒） |
| `workflows` | `refresh_interval` | `1m` | 策略工作流刷新间隔 |

## 17. resilience — 全局韧性（供应商可覆盖）

全局默认，作用于所有供应商；单个供应商内可写 `resilience:` 覆盖（见 19.1）。

```yaml
resilience:
  retry:
    max_retries: 3
    initial_backoff: 1s
    max_backoff: 30s
    backoff_factor: 2.0
    jitter_factor: 0.1
  circuit_breaker:
    enabled: true             # false = 永不熔断
    failure_threshold: 5      # 连续失败 N 次断开
    success_threshold: 2      # 半开态成功 N 次恢复
    timeout: 30s              # 断开后探测间隔
```

## 18. guardrails — 护栏（请求改写/策略注入）

| 键 | 默认 | 说明 |
| --- | --- | --- |
| `enabled` | `false` | 总开关 |
| `enable_for_batch_processing` | `false` | 是否作用于批处理 |
| `rules[]` | — | 见下 |

规则语义：

- 每条 = 一个护栏实例；**同类型可多实例**。
- **相同 `order` 并行，不同 `order` 按序执行**（从小到大）。
- 名字支持空格与 Unicode。

示例文件给出两类（实际支持类型以 schema 为准）：

```yaml
guardrails:
  enabled: false
  rules:
    - name: "safety-prompt"
      type: "system_prompt"             # 改写系统提示词
      order: 0
      system_prompt:
        mode: "decorator"               # inject | override | decorator
        content: "Always be safe and respectful."

    - name: "privacy-rewrite"           # 用辅助模型改写用户消息（默认提示词源自 LiteLLM
      type: "llm_based_altering"        # 脱敏提示词，可整体覆盖）
      user_path: "/team/privacy"        # 内部改写调用与审计日志的基准路径
      order: 1
      llm_based_altering:
        model: "gpt-4o-mini"
        roles: ["user"]
        max_tokens: 4096
        skip_content_prefix: "### safe" # 命中此前缀的内容不改写
        # prompt: "自定义改写指令..."
```

## 19. providers — 供应商

### 19.1 公共字段

| 字段 | 说明 |
| --- | --- |
| `<name>:`（键） | 供应商实例名；env 直连规则见下 |
| `type` | 驱动类型（见 19.2 表） |
| `api_key` | 支持 `${ENV}`；Bedrock 系可省略走 AWS 凭据链 |
| `base_url` | 覆盖默认端点 |
| `models` | 字符串清单 **或** 富条目 `{id, metadata}`（metadata 声明 context_window/pricing/modes/capabilities；会**合并**到远程目录条目上，操作者值逐字段胜出；远程目录没有的本地模型必须靠它补元数据） |
| `model_filter` | 清单过滤（目前见 openrouter 示例） |
| `resilience` | 可选，只覆盖指定的字段，其余用 17 的全局默认 |
| `api_keys[]` / `session_sticky_keys` | 多 key 轮换（见下） |

**provider 级环境变量规则**（重要）：

- 裸 `<PROVIDER>_*`（如 `OPENAI_API_KEY`、`OPENAI_BASE_URL`）覆盖**承载该类型名字**的 provider（如名为 `openai` 的那个）。
- 给 provider 改名后（`alpha: {type: openai}`），裸 env **只填这里留空的字段**；显式设置的值保留，被忽略的 env 会在启动日志告警。
- 同一类型跑第二个实例：用 `<PROVIDER>_<SUFFIX>_*`（如 `OPENAI_API_KEY_2`）或再写一个具名条目。
- 多 key 轮换：`OPENAI_API_KEY_2/3/...` 或 `api_keys: []`。默认行为是**会话粘性选 key**（保住供应商 prompt 缓存亲和，同时不同会话分散）；`session_sticky_keys: false` 退回严格逐请求轮询（通常对 prompt 缓存更差）。

### 19.2 类型清单（示例内出现）

| type | 说明 / 注意 |
| --- | --- |
| `openai` | OpenAI 及一切 OpenAI 兼容端点（自定义、LM Studio、Groq、Oracle…见下） |
| `anthropic` | 原生 Anthropic |
| `chatgpt` | ChatGPT 订阅（Codex 后端）；**只服务 `/v1/responses`**；key 取自 `codex login` 的 access token（`jq -r .tokens.access_token ~/.codex/auth.json`） |
| `cohere` / `bailian` / `gemini` / `xai` / `groq` / `fireworks` / `chutes` / `meta` / `zai` / `xiaomi` / `opencode_go` | 各云厂商。`bailian`（阿里百炼）可换 region base_url；`meta` 的 Muse Spark 未入目录需补 metadata |
| `vertex` | Google Vertex：`auth_type: gcp_adc\|gcp_service_account` + `vertex_project/location`；service account 三选一（文件 / JSON / base64） |
| `gemini` | 默认走原生 API；`api_mode: openai_compatible`（或 env `GEMINI_API_MODE`）切 OpenAI 兼容面（图片编辑必须原生） |
| `elevenlabs` | 纯语音（TTS/STT），无 chat；`voice` 字段必须填 ElevenLabs voice_id |
| `ollama` / `vllm` / `sglang` / `llmd` | 本地/自建推理。`llmd` 是 llm-d Router/EPP：`fairness_from_user_path`、路由不转发 `/models` 时需声明 `models` |
| `azure` | `api_version` 必填 |
| `bedrock` | 无 api_key（AWS 凭据链）；`base_url` 给 region（`us-east-1`）或完整 https 端点才启用 |
| `bedrock-mantle` | Bedrock 的 OpenAI 兼容端点；**Responses-only 模型（如 GPT-5.6 系）必需**；`api_mode: auto\|openai\|standard` |
| `deepseek` | 无原生 Responses API，GoModel 自动把 `/v1/responses` 翻译成 chat completions |
| `kilo` | Kilo AI Gateway：模型 ID 为 `provider/model` 原样转发；`KILO_MODELS` env 可逗号清单 |

### 19.3 实战示例（来自 example 注释，含多个易错点）

```yaml
providers:
  openai:                       # 名字 = 类型时，裸 OPENAI_* env 直连生效
    type: openai
    api_key: "${OPENAI_API_KEY}"

  lmstudio:                     # LM Studio 说 Ollama 兼容但只讲 OpenAI 协议——
    type: openai                # 必须配 openai（或 vllm），配 ollama 会因
    base_url: "http://localhost:1234/v1"   # /api/embed 缺失而失败
    api_key: "lm-studio"        # 任意非空，LM Studio 忽略

  openrouter:
    type: "openrouter"
    base_url: "https://openrouter.ai/api/v1"
    api_key: "${OPENROUTER_API_KEY}"
    model_filter:               # 通配大小写不敏感；* 也能匹配 /，如 *:free
      include: ["*:free"]       # 只留免费模型
      exclude: ["*-preview:free"]
      max_price_per_mtok: 0     # 0 = 不许收费模型；超价/无价模型被剔除
    # models: [openai/gpt-oss-120b]      # 配了则按 models.configured_provider_models_mode 生效

  azure:
    type: "azure"
    base_url: "${AZURE_BASE_URL}"
    api_key: "${AZURE_API_KEY}"
    api_version: "2024-10-21"

  nippur:                       # 本地模型补元数据（远程目录没有的）
    type: "ollama"
    base_url: "http://127.0.0.1:8080/v1"
    models:
      - id: GLM-4.7-Flash
        metadata:
          display_name: "GLM 4.7 Flash (local)"
          context_window: 131072
          max_output_tokens: 8192
          modes: ["chat"]
          capabilities: { tools: true }
          pricing: { currency: USD, input_per_mtok: 0, output_per_mtok: 0 }
      - Gemma4-31B              # 纯字符串条目 = 无富化模型（旧写法仍兼容）
```

## 20. offline — 一键离线（air-gapped）

```yaml
offline: true     # 默认 false；env: GOMODEL_OFFLINE=true（或 1）
```

开一个开关同时：① 关闭每日版本检查；② 丢弃 HTTP(S) 的模型目录 URL。**本地文件形式的模型目录、已声明端点与已配置供应商不受影响**。专为内网/涉密部署设计。

## 21. version_check — 版本检查

默认每天访问一次 `https://gomodel.enterpilot.io/version`（+ 每天首次打开 Dashboard 再查一次），让运维知道有新版本。上送内容仅：运行版本、发行版名、随机安装 ID、浏览器 UA 与语言。**绝不发送** key、模型名、提示词、用量、客户端地址、Dashboard 主机名。

```yaml
version_check:
  enabled: false    # 关闭所有外发（env: GOMODEL_VERSION_CHECK_ENABLED）
```

## 22. extensions — 扩展

默认**没有任何扩展**。仅当自定义发行版需要时才加具名段；核心只透传不校验，由对应扩展严格校验自己的段。

```yaml
# extensions:
#   example:
#     enabled: true
```

### 22.1 complexity_routing — 复杂度感知路由（内置扩展）

自适应负载均衡虚拟模型的 RouteSelector：对每个请求估算任务复杂度并按档位选目标。阈值**经验值试运行**（2026-09 起），一个月审计日志后 K-Means 校准。

```yaml
extensions:
  complexity_routing:
    enabled: true
    thresholds:            # 复杂度分档阈值（0~1），全部可选
      simple_medium: 0.45  # 低于此值 = simple
      medium_complex: 0.62 # 低于此值 = medium
      complex_very_complex: 0.85 # 低于此值 = complex，否则 very_complex
```

估算只读请求摘要（消息数/代码块/图片标志等计数与布尔，**不含消息正文**）；`vision` 是硬过滤（无该能力的候选直接剔除），`function_calling` 是软偏好（无匹配时降级不报错）。

### 22.2 模型能力实测与被动观测（运维语义，无配置段）

模型判型 = 静态推断 → **模型实测**（operator 手动触发，离线）→ 被动观测建议（operator 门控确认）。全部走 Admin API，无独立配置键：

| 端点 | 作用 |
| --- | --- |
| `POST /admin/models/test` | 对 `provider/model` 跑探测（chat / embeddings / function_calling，三态：pass / inconclusive / fail） |
| `GET /admin/models/test-results` | 最近一轮探测结果 |
| `PUT /admin/models/capabilities` | operator 确认探测结论 → 写入模型能力元数据（`capability_sources: test`） |
| `GET /admin/models/observed-suggestions` | 被动观测聚合建议（≥3 个不同会话一致信号才成建议；INCONCLUSIVE 永不成建议） |
| `PUT /admin/models/observed-capabilities` | operator 确认观测建议 → 写入元数据（`capability_sources: observed`） |
| `GET /admin/models/capability-errors` | 近 7 天运行时类型/能力错误聚合（`?days=N` 可调，≤90） |

审计日志新增轻量列（布尔/枚举，**不含正文**）：`capability_signals.had_tools`、`capability_signals.response_tool_calls`、`capability_signals.had_image`、`capability_error`（`type_mismatch` / `capability_mismatch`，仅检测记录，不影响请求路由）。失败探测与缺失信号**永不**覆盖既有元数据（防误杀）。

---

## 附录 A：常用环境变量速查

| 变量 | 对应配置键 | 说明 |
| --- | --- | --- |
| `BASE_PATH` | `server.base_path` | 挂载前缀 |
| `SWAGGER_ENABLED` | `server.swagger_enabled` | 需 `-tags=swagger` 构建 |
| `USER_PATH_HEADER` | `server.user_path_header` | 用户路径头 |
| `REALTIME_ENABLED` | `server.realtime_enabled` | Realtime WebSocket |
| `PID_FILE` | `server.pid_file` | `--reload` 用 |
| `MODELS_ENABLED_BY_DEFAULT` / `KEEP_ONLY_ALIASES_AT_MODELS_ENDPOINT` / `UNQUALIFIED_MODEL_IDS_AT_MODELS_ENDPOINT` / `CONFIGURED_PROVIDER_MODELS_MODE` | `models.*` | 模型可见性 |
| `MODEL_LIST_URL` | `cache.model.model_list.url` | 目录 URL/本地文件/`off` |
| `PROVIDER_RECHECK_INTERVAL` | `cache.model.recheck_interval` | 失败供应商重探 |
| `RESPONSE_CACHE_SIMPLE_ENABLED` / `SEMANTIC_CACHE_ENABLED` | `cache.response.*` | 缺省段时的开关 |
| `MCP_ENABLED` / `MCP_ALLOWED_ORIGINS` / `MCP_SERVERS` | `mcp.*` | MCP 网关 |
| `BUDGETS_ENABLED` | `budgets.enabled` | |
| `RATE_LIMITS_ENABLED` / `RATE_LIMITS_FLUSH_INTERVAL` | `rate_limits.*` | |
| `FAILOVER_ENABLED` / `FAILOVER_MAX_ATTEMPTS` / `FAILOVER_RETRY_ON_STATUSES` / `FAILOVER_RETRY_ON_ERRORS` | `failover.*` | 故障转移 |
| `SET_BUDGET_USER__PATH__EXAMPLE` | `budgets.user_paths` | 预算，`__`=路径层级 |
| `SET_RATE_LIMIT_USER__PATH__EXAMPLE` / `SET_PROVIDER_RATE_LIMIT_OPENAI` | `rate_limits` | 限流 |
| `TAGGING_HEADER_1`(+`_PREFIX/_DONOTPASS/_DELIMITER`) | `tagging.headers` | 打标签 |
| `VIRTUAL_MODELS` | `virtual_models` | JSON 数组，按 source 合并胜出 |
| `USERS` | `users` | JSON 数组，按 path 合并胜出 |
| `<PROVIDER>_API_KEY` / `<PROVIDER>_BASE_URL` / `<PROVIDER>_API_KEY_2`… / `<PROVIDER>_MODELS` | `providers` | 供应商直连 / 多 key / 清单 |
| `OTEL_*` | `opentelemetry` | 环境变量优先于 YAML |
| `GOMODEL_OFFLINE` | `offline` | 一键离线 |
| `GOMODEL_VERSION_CHECK_ENABLED` | `version_check.enabled` | 版本检查 |
| `CONFIG_STRICT` | —（加载器） | 默认 true；false = 未知键降级为警告 |

## 附录 B：常见问题

**Q1 启动报 `unknown configuration key ...`？**
`CONFIG_STRICT=true`（默认）下写了不存在的键。核对键名（对照本手册），或临时 `CONFIG_STRICT=false` 看警告明细。

**Q2 启动 WRN `failed to fetch model list ... EOF`？**
默认目录在 GitHub raw，受限网络下被限速断流。无碍启动（走缓存/供应商自报），但要拿到定价元数据就按 9 节改用本地文件或 `off`，可叠加 `offline: true`。

**Q3 改了配置没生效？**
配置启动时加载一次；`--reload` 需先配 `server.pid_file`，新配置非法时旧配置继续生效。`.env` 只填真实环境未设置的变量；导出变量优先。

**Q4 `MODEL_LIST_URL=`（空）关不掉目录？**
空 env 被跳过，用 `off` 哨兵。

**Q5 本地模型文件路径报错？**
确认是常规文件（FIFO/设备会被拒）、≤10 MB、路径相对进程 CWD（`make run` = 仓库根）。

**Q6 相对路径失效？**
相对路径全部相对**进程 CWD**，不是配置文件目录。systemd/Docker 部署用绝对路径或容器内路径。

**Q7 想跑同类型两个供应商？**
第二个实例用 `<PROVIDER>_<SUFFIX>_*` env 或另写具名条目（见 19.1）；裸 `<PROVIDER>_*` env 只认承载类型名的那个 provider。
