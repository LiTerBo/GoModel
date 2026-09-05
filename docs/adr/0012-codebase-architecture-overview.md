# ADR-0012: GoModel Codebase Architecture Overview

- Status: accepted
- Date: 2026-09-03
- Evidence: codebase-memory knowledge graph (23,490 nodes / 122,873 edges, full-mode index, 2026-09-03)

## PURPOSE

GoModel is a high-performance, lightweight AI gateway exposing an OpenAI-compatible API (`/v1/chat/completions`, `/v1/responses`, `/v1/conversations`, realtime, MCP gateway) that routes requests to multiple AI model providers. It accepts requests generously (Postel's Law — e.g. translating `max_tokens` to `max_completion_tokens` for reasoning models) and returns provider responses in a conservative OpenAI-compatible shape. This record captures the architecture as extracted from the code knowledge graph and is refreshed whenever the index is rebuilt.

## STACK

- **Language**: Go (1,143 files) — the entire gateway runtime.
- **Frontend**: Dashboard in JavaScript (134 files) + Svelte (113 files) under `web/dashboard/`.
- **Config/CI**: YAML (25), Bash (6); Python appears only in docs/ examples and benchmark tooling.
- **Entry points**: `cmd/gomodel/main.go` (gateway), `cmd/recordapi/main.go` (recording API).

## ARCHITECTURE

Graph-derived package boundaries (call counts):

| From → To | Calls |
|---|---|
| server → core | 729 |
| server → mcpgateway | 281 |
| server → runtimesettings | 261 |
| server → auditlog | 187 |
| server → responsecache | 175 |
| auditlog → storage | 121 |
| virtualmodels → core | 79 |
| virtualmodels → storage | 57 |

Layer classification (graph-computed):

- **entry**: `server` (only outbound calls), `virtualmodels`.
- **core** (high fan-in, no outbound): `core` (fan-in 808), `mcpgateway` (281), `runtimesettings` (261), `auditlog` (187), `storage` (178), `responsecache` (175), `ratelimit` (34), `usage` (32).
- **api**: route-defining packages (333 Route nodes incl. `/v1/chat/completions`, `/v1/responses`, `/v1/realtime`, `/vectors/*`).

Hotspots (fan-in, change with care):

| Symbol | Fan-in |
|---|---|
| `internal/server.New` | 387 |
| `internal/core.NewInvalidRequestError` | 335 |
| `internal/responsecache.exchange.Context` | 327 |
| `internal/server.Close` | 262 |
| `internal/runtimesettings.Store.Set` | 236 |
| `run.Error` | 212 |
| `internal/providers.Error` | 180 |
| `internal/admin.NewHandler` | 168 |
| `internal/core.NewProviderError` | 152 |
| `internal/providers.CredentialStore.Get` | 130 |

Leiden clusters confirm de-facto modules that cut across folder boundaries:

| Cluster (top nodes) | Members | Cohesion |
|---|---|---|
| Cost calculation (`CalculateGranularCost`, `CalculateUsageCost`) | 259 | 0.84 |
| Request translation (`ConvertResponsesRequestToChat`, `UnknownJSONFieldsFromMap`, `Lookup`) | 347 | 0.82 |
| Provider HTTP client (`Do`, `Close`, `NewWithHTTPClient`) | 211 | 0.78 |
| Core I/O (`Write`, `Close`, `Error`, `Get`, `Do`) | 608 | 0.77 |
| Load balancing (`newBalancingService`, `QualifiedModel`, `NewRequestedModelSelector`) | 238 | 0.76 |
| Model registry (`NewModelRegistry`, `RegisterProviderWithNameAndType`) | 238 | 0.73 |
| DB access (`Exec`, `Query`, `QueryRow`) | 349 | 0.73 |

## PATTERNS

- **Explicit provider registration** (ADR-0001): registry cluster with `RegisterProviderWithNameAndType`.
- **Capability model + provider attempts** (ADR-0004): provider-client clusters encode attempt/fallback chains.
- **Policy-resolved workflow** (ADR-0003) and **provider-qualified model selectors** (ADR-0005): the balancing/selector cluster.
- **Semantic response cache** (ADR-0006): `responsecache.exchange` is a top-3 fan-in hotspot.
- **Translation layer** (Postel's Law): request-translation cluster (`ConvertResponsesRequestToChat`, `UnknownJSONFieldsFromMap`, `CloneRawJSON`) normalizes liberally-parsed input before conservative output.
- **Test coverage**: 10,389 TESTS edges + 357 TESTS_FILE edges — a dense test-to-code mapping.

## TRADEOFFS

- `server` is a god-package entry: 729 outbound calls to core make it the single integration/orchestration layer; changes there need broad regression (dense TESTS edges mitigate).
- `core` has fan-in 808 with zero outbound — a dependency-free kernel that maximizes testability, at the cost of near-duplication risk with provider-specific logic kept outside (1,095 SIMILAR_TO edges flag candidates for consolidation).
- Dashboard JS/Svelte is decoupled from the Go runtime except through routes; its entry points are the shared api-client functions in `web/dashboard/src/lib/api/`.

## PHILOSOPHY

Postel's Law at the boundary (accept generously, emit conservatively); Twelve-Factor configuration; KISS and explicit, maintainable code over clever abstractions; good defaults so users rarely change configuration. The graph confirms this: a dependency-free `core`, thin orchestration in `server`, and provider adapters isolated in per-provider packages.
