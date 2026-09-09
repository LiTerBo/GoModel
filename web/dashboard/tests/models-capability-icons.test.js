// Capability icon row (phase D): a per-model icon strip in the model name
// cell — one icon per capability (text generation, embeddings, function
// calling, vision), three visual states (confirmed / declared / hidden).
//
// The state computation is pure logic in capabilityIcons.js (node:test
// exercises it directly); the ModelRow wiring is verified by source-contract
// assertions (the established models-page pattern) plus svelte-check in CI.

import test from "node:test";
import assert from "node:assert/strict";

import {
  CAPABILITY_ORDER,
  capabilityIconStates,
  capabilityIconTitle,
} from "../src/pages/models/capabilityIcons.js";

const row = (capabilities, sources = {}, extra = {}) => ({
  is_alias: false,
  model: { id: "my-llm", metadata: { capabilities, capability_sources: sources } },
  ...extra,
});

test("capabilityIconStates: hidden when the capability is absent or false", () => {
  const states = capabilityIconStates(row({ vision: true, function_calling: false }));
  assert.equal(states.find((s) => s.key === "chat"), undefined);
  // function_calling explicitly false = unsupported, not declared.
  assert.equal(states.find((s) => s.key === "function_calling"), undefined);
  // vision true with no source = declared (static) state.
  const vision = states.find((s) => s.key === "vision");
  assert.equal(vision.state, "declared");
});

test("capabilityIconStates: confirmed only when the capability value is true despite a confirmed source", () => {
  // Regression: an operator-confirmed false (explicitly unsupported)
  // must not light up the icon — only the stored source leads to
  // "confirmed" even when the boolean flag is false.
  const states = capabilityIconStates(
    row(
      { function_calling: false, vision: false },
      { function_calling: "test", vision: "observed" },
    ),
  );
  assert.equal(
    states.find((s) => s.key === "function_calling"),
    undefined,
    "function_calling=false + test source must hide the icon",
  );
  assert.equal(
    states.find((s) => s.key === "vision"),
    undefined,
    "vision=false + observed source must hide the icon",
  );
  // Category-backed icons (chat, embeddings) are unaffected — no
  // boolean flag to check against; the source alone suffices.
});

test("capabilityIconStates: confirmed when the capability value and source both agree", () => {
  const states = capabilityIconStates(
    row(
      { function_calling: true, vision: true },
      { function_calling: "test", vision: "observed" },
    ),
  );
  assert.equal(states.find((s) => s.key === "function_calling").state, "confirmed");
  assert.equal(states.find((s) => s.key === "vision").state, "confirmed");
});

test("capabilityIconStates: declared when a non-verification source set the flag", () => {
  const states = capabilityIconStates(
    row({ function_calling: true }, { function_calling: "heuristic" }),
  );
  assert.equal(states.find((s) => s.key === "function_calling").state, "declared");
});

test("capabilityIconStates: chat icon rides the text_generation category, not a chat flag", () => {
  // A text-generation model has no boolean "chat" capability; the backend
  // flags it via the categories list.
  const states = capabilityIconStates(
    row({}, {}, { model: { id: "gpt-x", metadata: { categories: ["text_generation"] } } }),
  );
  assert.equal(states.find((s) => s.key === "chat").state, "declared");
  const embeddingRow = row(
    {},
    {},
    { model: { id: "bge", metadata: { categories: ["embedding"] } } },
  );
  assert.equal(
    capabilityIconStates(embeddingRow).find((s) => s.key === "embeddings").state,
    "declared",
  );
});

test("capabilityIconStates: confirmed beats declared for the chat category", () => {
  const states = capabilityIconStates(
    row(
      {},
      { chat: "test" },
      { model: { id: "gpt-x", metadata: { categories: ["text_generation"], capability_sources: { chat: "test" } } } },
    ),
  );
  assert.equal(states.find((s) => s.key === "chat").state, "confirmed");
});

test("capabilityIconStates: aliases and metadata-less rows render nothing", () => {
  assert.deepEqual(capabilityIconStates({ is_alias: true, alias: {} }), []);
  assert.deepEqual(capabilityIconStates({ is_alias: false }), []);
  assert.deepEqual(capabilityIconStates(null), []);
});

test("capabilityIconStates: embeddings capability key matches the probe name", () => {
  // Backend stores the embeddings capability under "embeddings" (probe
  // naming); a row confirmed by probe must surface it.
  const states = capabilityIconStates(
    row({ embeddings: true }, { embeddings: "test" }),
  );
  assert.equal(states.find((s) => s.key === "embeddings").state, "confirmed");
});

test("CAPABILITY_ORDER is stable and icon keys are fixed", () => {
  assert.deepEqual(CAPABILITY_ORDER.map((c) => c.key), [
    "chat",
    "embeddings",
    "function_calling",
    "vision",
  ]);
});

test("capabilityIconTitle composes capability name and provenance", () => {
  const labels = {
    capability: (key) => (key === "function_calling" ? "函数调用" : key),
    confirmedTest: () => "实测验证",
    confirmedObserved: () => "流量验证",
    declared: () => "静态推断",
  };
  assert.equal(
    capabilityIconTitle("function_calling", "confirmed", labels, "test"),
    "函数调用 · 实测验证",
  );
  assert.equal(
    capabilityIconTitle("function_calling", "confirmed", labels, "observed"),
    "函数调用 · 流量验证",
  );
  assert.equal(
    capabilityIconTitle("function_calling", "declared", labels),
    "函数调用 · 静态推断",
  );
});

test("ModelRow renders the capability icon strip from the pure module", async () => {
  const { readFile } = await import("node:fs/promises");
  const src = await readFile(
    new URL("../src/pages/models/ModelRow.svelte", import.meta.url),
    "utf8",
  );
  assert.match(src, /capabilityIconStates/);
  assert.match(src, /capability-icon-row/);
  assert.match(src, /capabilityIconTitle/);
});

test("capability icon i18n keys exist in both language catalogs", async () => {
  const { readFile } = await import("node:fs/promises");
  const en = JSON.parse(
    await readFile(new URL("../messages/en.json", import.meta.url), "utf8"),
  );
  const zh = JSON.parse(
    await readFile(new URL("../messages/zh.json", import.meta.url), "utf8"),
  );
  const keys = [
    "models_cap_icon_chat",
    "models_cap_icon_embeddings",
    "models_cap_icon_function_calling",
    "models_cap_icon_vision",
    "models_cap_state_confirmed_test",
    "models_cap_state_confirmed_observed",
    "models_cap_state_declared",
  ];
  for (const key of keys) {
    assert.equal(typeof en[key], "string", `en.${key} missing`);
    assert.equal(typeof zh[key], "string", `zh.${key} missing`);
    assert.ok(en[key].trim() && zh[key].trim());
  }
});
