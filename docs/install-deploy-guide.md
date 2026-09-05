# GoModel 安装部署指南（Step by Step）

> 基于对仓库构建体系的实际分析，关键路径均已在 macOS (arm64) + Go 1.27.0 + Node v22 上实测验证（2026-09-05，commit ac48611d）。
>
> GoModel 是一个 AI 网关：接受 OpenAI 兼容（`/v1`）与 Anthropic 兼容（`/v1/messages`）请求，路由到多家模型供应商，并内置 Dashboard、用量/成本追踪、缓存、限流、虚拟模型等能力。

---

## 1. 构建方式总览（先读懂再动手）

| 维度 | 说明 |
| --- | --- |
| 语言/版本 | Go **1.27.0**（`go.mod`），`CGO_ENABLED=0` 纯静态编译，产出**单个二进制** |
| 入口 | `cmd/gomodel`（网关主程序）、`cmd/recordapi`（契约测试录制工具，部署不需要） |
| 前端 | Dashboard 为 Svelte 5 + Vite（`web/dashboard`），构建产物输出到 `internal/admin/dashboard/static/dist`，**通过 go:embed 嵌入二进制** |
| 关键顺序 | **必须先构建前端再编译 Go**，否则 `make build`/`make test` 直接报错；`dist/` 不入 Git，每份工作区都要自己生成 |
| 配置体系 | 优先级：内置默认 → `config/config.yaml`（可选）→ `.env`（可选）→ 导出的环境变量（最高） |
| 存储 | 审计日志/用量：SQLite（默认）/ PostgreSQL / MongoDB（`STORAGE_TYPE`）；缓存：本地或 Redis（`REDIS_URL`） |
| 观测 | Prometheus `/metrics`、OpenTelemetry、Swagger（需 `-tags=swagger` 构建并 `SWAGGER_ENABLED=true`） |
| 发布产物 | `.goreleaser.yaml`：linux/darwin/windows × amd64/arm64 的 tar.gz/zip + checksums；Docker 镜像 `enterpilot/gomodel` |

四种部署方式按需选择：

- **方式 A：源码构建**（开发者/定制场景，最常用）
- **方式 B：Docker / Docker Compose**（服务器部署推荐）
- **方式 C：一键安装脚本**（最终用户，直接用官方发行版二进制）
- **方式 D：Kubernetes Helm**（集群部署）

---

## 2. 环境要求

| 依赖 | 版本 | 说明 |
| --- | --- | --- |
| Go | ≥ 1.27.0 | 本机没有 1.27.0 时 Go 会自动下载 toolchain（国内网络见 §3.2 的坑） |
| Node.js + npm | ≥ 20（实测 v22.23.2 / npm 10.9.8） | 仅构建 Dashboard 时需要 |
| Docker + Compose v2 | 任意近期版本 | 方式 B 需要 |
| make / git / curl | 系统自带 | — |

---

## 3. 方式 A：源码构建与运行

### Step 1 克隆仓库

```bash
git clone https://github.com/LiTerBo/GoModel.git   # 或你自己的 fork / 内网地址
cd GoModel
```

### Step 2 配置 Go 模块代理（国内网络必做）

Go 需要 `go1.27.0` toolchain 与全部依赖模块。**直连 `storage.googleapis.com` 在国内网络会 EOF 失败**，且此方案完全不依赖本地代理软件：

```bash
# 写入 profile 持久化，或仅当前 shell export
go env -w GOPROXY=https://goproxy.cn,direct
```

> 验证结果：设置后 toolchain 与全部依赖一次性下载成功，`go build` 通过。
> 如使用本地代理软件，注意 curl 默认会走 `http_proxy`，**本地回环测试时务必 `--noproxy '*'`**（见 §8 常见问题）。

### Step 3 构建 Dashboard（先于 Go 编译，顺序不能反）

```bash
make frontend
```

等价手动操作：

```bash
cd web/dashboard
npm ci --no-audit --no-fund --ignore-scripts   # --ignore-scripts：不执行 npm 生命周期脚本，构建不需要
npm run build
cd -
```

产物落在 `internal/admin/dashboard/static/dist/`（含 `index.html`）。可通过 `ls internal/admin/dashboard/static/dist/index.html` 确认。

### Step 4 编译网关二进制

```bash
make build    # 产出 bin/gomodel，ldflags 注入版本号/commit/构建时间
```

验证版本信息（实测输出示例）：

```bash
$ ./bin/gomodel --version
gomodel v0.1.87-3-gac48611d (commit: ac48611d, built: 2026-09-05T01:22:12Z, go1.27.0)
```

> 手动编译等价命令（注意必须嵌入 Dashboard 之后）：
> `go build -ldflags "-X github.com/enterpilot/gomodel/internal/version.Version=dev" -o bin/gomodel ./cmd/gomodel`

### Step 5 最小配置

```bash
cp .env.template .env
```

编辑 `.env`，**强烈建议**设置主密钥（不设置则以 UNSAFE MODE 启动并打印安全警告）：

```bash
# .env
GOMODEL_MASTER_KEY=your-secret-key-here
# 至少配一家供应商的 key（也可稍后在 Dashboard 的 Providers 页配置，免重启）
# OPENAI_API_KEY=sk-...
```

完整变量清单见 `.env.template`（约 700 行，含全部供应商与开关注释）。

### Step 6 启动

```bash
# 开发模式：带 swagger tag 重新编译并前台运行（Ctrl+C 干净退出）
make run            # 默认 LOG_LEVEL=debug, SWAGGER_ENABLED=true

# 或直接运行已编译产物
GOMODEL_MASTER_KEY=your-secret-key-here ./bin/gomodel
```

可选：灌入 90 天滚动演示数据（850 请求/天量级）再启动，Dashboard 立即有图有数：

```bash
make demo   # = seed-demo-data + GOMODEL_DEMO_MODE=true 启动
```

### Step 7 手动启动与停止（实测命令）

```bash
# ── 启动 ────────────────────────────────────────────────
cd ~/gitlab/GoModel && ./bin/gomodel            # 前台运行；自动加载工作目录的 .env
./bin/gomodel > data/gomodel.log 2>&1 &        # 或后台运行（日志重定向）

# ── 验证 ────────────────────────────────────────────────
curl --noproxy '*' http://localhost:8080/health        # → 200
./bin/gomodel --health                                  # 容器 HEALTHCHECK 同款探针
./bin/gomodel --ready                                   # 就绪探针

# ── 停止（SIGTERM 优雅退出，自动清理 pid 文件）──────────
kill "$(cat data/gomodel.pid)"                  # 首选：pid 文件在 SQLite 数据目录旁
pkill -f bin/gomodel                            # 兜底：按进程名匹配（效果相同）

# ── 改配置后热重载（不重启进程）─────────────────────────
./bin/gomodel --reload                          # 依据 pid 文件通知运行中进程
```

注意：
- **必须从项目根目录启动**——`.env`、`data/`、`SQLITE_PATH` 均按工作目录解析；
- pid 文件与 SQLite 同目录（`SQLITE_PATH=data/gomodel.db` 时为 `data/gomodel.pid`；未建 `data/` 时在系统用户数据目录）；
- `make run` 是开发模式（swagger tag 重编译 + debug 日志），日常运行用 `./bin/gomodel` 即可；
- 优雅退出日志特征：`shutting down application...` → `application shutdown complete`。

### Step 8 验证（以下均为实测结果）

```bash
# 1) 存活检查
curl --noproxy '*' http://localhost:8080/health          # → 200

# 2) Dashboard
open http://localhost:8080/admin/dashboard               # → 200，浏览器打开

# 3) 鉴权生效（未带 key 应拒绝）
curl --noproxy '*' http://localhost:8080/v1/models
# → 401 {"error":{"message":"missing credentials: send 'Authorization: Bearer ..."}} ✔

# 4) 带主密钥调用（已配置至少一家供应商后）
curl --noproxy '*' http://localhost:8080/v1/models \
  -H "Authorization: Bearer $GOMODEL_MASTER_KEY"          # → 200 模型列表
# 注意：若尚未配置任何供应商，此接口返回 503 "model registry has no models"——
# 属预期行为：路由 registry 由已配置供应商的模型构成。到 Dashboard 的
# Providers 页添加供应商（免重启），或在 .env 配置 <PROVIDER>_API_KEY 后重启。

# 5) 对话请求
curl --noproxy '*' http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $GOMODEL_MASTER_KEY" \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}'
```

启动日志应出现（JSON 格式，非 TTY 自动切换）：`authentication enabled mode=master_key` → `storage configured type=sqlite` → `starting server address=:8080` → `model list loaded models=897 providers=20`。

### SQLite 数据落在哪里（易踩坑）

默认路径规则：**当前目录存在 `./data/` 时用 `data/gomodel.db`；否则落到系统用户目录**（macOS 为 `~/Library/Application Support/gomodel/gomodel.db`，Linux 为 `~/.local/share/gomodel/gomodel.db`）。想让数据跟着仓库走，先 `mkdir -p data` 再启动，或显式 `SQLITE_PATH=data/gomodel.db`。

---

## 4. 方式 B：Docker / Docker Compose

### 4.1 直接使用官方镜像（最快）

```bash
docker run --rm -p 8080:8080 \
  -e GOMODEL_MASTER_KEY=your-secret-key-here \
  -e OPENAI_API_KEY="your-openai-key" \
  enterpilot/gomodel
```

镜像特性：多阶段构建（`golang:1.27.0-alpine` 编译 → `distroless/static-debian12:nonroot` 运行），内置 `HEALTHCHECK`（`/gomodel --health`），非 root 用户运行，工作目录 `/app`。

### 4.2 本地构建镜像

```bash
docker build -t gomodel:local \
  --build-arg VERSION=$(git describe --tags --always) \
  .
# 注意：Dockerfile 不构建前端，需先在宿主机执行 make frontend 让 dist 就位
```

### 4.3 Docker Compose（推荐的生产化路径）

Compose 文件提供两组服务：

| 组 | 命令 | 包含 |
| --- | --- | --- |
| 仅基础设施 | `docker compose up -d`（或 `make infra`） | Redis 7、PostgreSQL 18、MongoDB 8（副本集）、Adminer（:8081） |
| 全栈 | `docker compose --profile app up -d`（或 `make image`） | 上述 + GoModel 应用 + Prometheus（:9090） |

```bash
cp .env.template .env          # 填入 GOMODEL_MASTER_KEY 与供应商 key
docker compose --profile app up -d
```

要点：

- 应用容器从 `.env` 读取全部供应商 key（`env_file` 链），`REDIS_URL`/`STORAGE_TYPE` 已在 compose 内置好；
- Compose 默认 `STORAGE_TYPE=mongodb`（审计/用量入库 Mongo）；改用 PostgreSQL 把该行换成 `STORAGE_TYPE=postgresql`（注意同步修正 `POSTGRES_URL` 中的密码）；
- 持久化：`gomodel_data` 卷挂载 `/app/data`（SQLite 状态、pid、安装身份），重建容器不丢数据；
- 端口占用：网关 8080、Adminer 8081、Prometheus 9090、Redis 6379、PG 5432、Mongo 27017。

---

## 5. 方式 C：一键安装脚本（最终用户）

```bash
# macOS / Linux
curl -fsSL https://gomodel.enterpilot.io/install.sh | sh
gomodel

# Windows (PowerShell)
irm https://gomodel.enterpilot.io/install.ps1 | iex
gomodel
```

脚本行为：从 GitHub Releases 下载最新二进制 → 校验 SHA-256 → 安装到 `/usr/local/bin`（不可写则 `~/.local/bin`）。可覆盖变量：`GOMODEL_VERSION=v0.1.50`（指定版本）、`GOMODEL_INSTALL_DIR`（安装目录）。

---

## 6. 方式 D：Kubernetes（Helm Chart）

仓库自带 `helm/`（要求 K8s 1.29+、Helm 3）：

```bash
# 最小安装：设置 key 即自动启用对应 provider
helm install gomodel ./helm \
  -n gomodel --create-namespace \
  --set providers.openai.apiKey="sk-..."

# 多 provider + Redis 缓存
helm install gomodel ./helm -n gomodel --create-namespace \
  --set providers.openai.apiKey="sk-..." \
  --set providers.anthropic.apiKey="sk-ant-..." \
  --set redis.enabled=true

# GitOps 友好：key 走已有 Secret
helm install gomodel ./helm -n gomodel --create-namespace \
  --set providers.existingSecret="llm-api-keys" \
  --set providers.openai.enabled=true
```

默认 `replicaCount=2`，镜像 `enterpilot/gomodel`，含 HPA/Ingress/ServiceMonitor（Prometheus Operator）/PDB 模板，详见 `helm/README.md`。

---

## 7. 生产部署要点清单

| 项 | 建议 |
| --- | --- |
| 主密钥 | **必设** `GOMODEL_MASTER_KEY`，否则 UNSAFE MODE |
| 存储后端 | 多实例部署必须 `STORAGE_TYPE=postgresql` 或 `mongodb`（SQLite 仅单实例）；高频日志量推荐 MongoDB |
| 缓存 | 配 `REDIS_URL` 启用分布式精确/语义缓存 |
| 指标 | `METRICS_ENABLED=true`，Prometheus 抓 `/metrics`（compose 已含 Prometheus） |
| 日志 | 容器/管道环境自动 JSON（`LOG_FORMAT=json` 可强制）；`LOG_LEVEL=info` |
| 超时 | `HTTP_TIMEOUT=600`（默认 10 分钟，与 OpenAI/Anthropic SDK 对齐） |
| 离线/内网 | `GOMODEL_OFFLINE=true` 一键禁用全部出站调用（版本检查等） |
| 配置校验 | `CONFIG_STRICT=true` 拒绝未知配置键，防拼写错误静默失效 |
| 优雅热更 | 改配置后 `gomodel --reload` 通知运行中进程重载（需 `PID_FILE`） |

---

## 8. 常见问题（实测踩坑记录）

| 现象 | 根因 | 解决 |
| --- | --- | --- |
| `go: download go1.27.0: ... EOF` / 模块下载失败 | 直连 Google 存储/模块代理被墙 | `go env -w GOPROXY=https://goproxy.cn,direct`（不依赖本地代理） |
| 本机 curl `http://localhost:8080` 返回 **502** | shell 的 `http_proxy` 指向本地代理软件，回环流量被代理 | 测试加 `curl --noproxy '*'`，或 `export NO_PROXY=localhost,127.0.0.1` |
| `make build`/`make test` 报 `static/dist is missing: run 'make frontend' first` | Dashboard 产物未生成（不入 Git） | 先执行 `make frontend`，再 build/test |
| `go run` Ctrl+C 退出码 1（`make: *** [run] Error 1`） | `go run` 作为监督进程转发信号的行为 | 用 `make run`（Makefile 内已用 `exec` 替代 shell 解决） |
| 想让 SQLite 落在项目目录却出现在 `~/Library/...` | `./data/` 目录不存在时的默认回退规则 | `mkdir -p data` 或显式 `SQLITE_PATH` |
| Swagger 页面 404 | 二进制未带 `swagger` 构建标签 | `make run`（自动带 tag），或 `go build -tags=swagger` 且 `SWAGGER_ENABLED=true` |
| Compose 里 Mongo 起不来 | 单节点需副本集模式初始化 | compose 已内置 init 健康检查，等待 `start_period` 即可；勿自行去掉 `--replSet rs0` |
| `/v1/models` 返回 503 `model registry has no models` | 未配置任何供应商（registry 由供应商模型构成，与内置静态目录是两回事） | Dashboard → Providers 页添加（免重启），或 `.env` 配 `<PROVIDER>_API_KEY` 后重启 |

---

## 9. 构建体系速查（Makefile 目标地图）

```text
all          = frontend + build
frontend     npm ci + vite build → internal/admin/dashboard/static/dist（嵌入二进制）
build        go build → bin/gomodel（依赖 frontend）
run          swagger tag 重编译 + 前台运行（LOG_LEVEL=debug, SWAGGER_ENABLED=true）
demo         种子演示数据 + GOMODEL_DEMO_MODE 启动
test         Go 单测（前置 frontend-check）；test-race / test-dashboard / test-e2e（免Docker）/ test-integration（需Docker）/ test-contract / test-all
lint         golangci-lint v2.13.1（make install-tools 安装，build tags 覆盖 e2e/integration/contract）
swagger      重新生成 Swagger/OpenAPI（swag v2 → docs/openapi.json）
infra        compose 起 Redis/PG/Mongo/Adminer
image        compose --profile app 全栈（含本地镜像构建）
record-api   录制真实供应商响应到契约测试 golden 文件（需 OPENAI_API_KEY）
```

---

*文档生成：Hermes Agent · 实测环境 macOS 26.6.2 (arm64) / Go 1.27.0 / Node v22.23.2 / 仓库 commit ac48611d*
