// Inline test-result panel (phase E): per-row expansion state + confirm
// flow. Pure logic lives in modelTestPanel.js so node:test can exercise it
// directly; the Svelte side is verified by source-contract assertions (the
// established models-group-collapse-ui.test.js pattern) plus svelte-check.

import test from "node:test";
import assert from "node:assert/strict";

import {
  panelKey,
  isPanelOpen,
  togglePanel,
  closePanel,
  runnableCaps,
  probeLabel,
} from "../src/pages/models/modelTestPanel.js";

test("panelKey builds the provider/model selector", () => {
  assert.equal(panelKey("local", "my-llm"), "local/my-llm");
});

test("togglePanel opens, closes, and is keyed per row", () => {
  let open = new Set();
  open = togglePanel(open, "local/my-llm");
  assert.ok(isPanelOpen(open, "local/my-llm"));
  assert.ok(!isPanelOpen(open, "other/model"));

  open = togglePanel(open, "local/my-llm");
  assert.ok(!isPanelOpen(open, "local/my-llm"), "second toggle closes");

  // togglePanel replaces the set (Svelte 5 $state reassignment pattern).
  open = togglePanel(open, "local/my-llm");
  const snapshot = open;
  open = closePanel(open, "local/my-llm");
  assert.ok(!isPanelOpen(open, "local/my-llm"));
  assert.ok(isPanelOpen(snapshot, "local/my-llm"), "closePanel must not mutate its input");
});

test("runnableCaps keeps every probed capability with its verdict", () => {
  const results = [
    { probe: "chat", verdict: "pass" },
    { probe: "function_calling", verdict: "inconclusive" },
  ];
  // The chat probe rides the text_generation capability (no boolean chat
  // flag exists); inconclusive maps to an explicit pass=false confirmation.
  assert.deepEqual(runnableCaps(results), [
    { key: "text_generation", pass: true },
    { key: "function_calling", pass: false },
  ]);
});

test("runnableCaps maps the chat probe to the text_generation capability", () => {
  // The backend grants no boolean "chat" flag; the routable capability is
  // the text_generation category (see capabilityIcons.js stateOf).
  const results = [{ probe: "chat", verdict: "pass" }];
  assert.deepEqual(runnableCaps(results), [{ key: "text_generation", pass: true }]);
});

test("runnableCaps ignores unknown probes", () => {
  assert.deepEqual(runnableCaps([{ probe: "telepathy", verdict: "pass" }]), []);
  assert.deepEqual(runnableCaps(null), []);
});

test("probeLabel keys match the backend probe names", () => {
  assert.deepEqual([...runnableCaps([{ probe: "embeddings", verdict: "pass" }])], [
    { key: "embedding", pass: true },
  ]);
  assert.equal(
    probeLabel("chat", { chat: () => "文本生成" }),
    "文本生成",
    "labels resolve through the injected i18n map",
  );
  assert.equal(probeLabel("telepathy", { chat: () => "文本生成" }), "telepathy");
});

// --- ModelRow panel rendering (source contract; no component-render infra) -

test("ModelRow renders the inline test panel with verdicts and a confirm action", async () => {
  const { readFile } = await import("node:fs/promises");
  const src = await readFile(
    new URL("../src/pages/models/ModelRow.svelte", import.meta.url),
    "utf8",
  );

  // The flask button opens the panel and runs the test in one action.
  assert.match(src, /onclick=\{runAndOpen\}/);
  // Panel row: full-width <tr> under the model row, gated on panelOpen.
  assert.match(src, /capability-test-panel-row/);
  assert.match(src, /\{#if panelOpen\}/);
  // One line per probe with verdict and latency.
  assert.match(src, /capability-test-probe-\{probe\.verdict\}/);
  assert.match(src, /probe\.latency_ms/);
  assert.match(src, /probe\.detail/);
  // Confirm action calls the store's probe-derived confirm; the panel
  // closes on success (confirmFromPanel closes after a non-failed outcome).
  assert.match(src, /modelTest\.confirmFromProbes\(/);
  assert.match(src, /closeResultPanel\(\)/);
  // Elapsed-seconds counter runs only while the panel is open and running.
  assert.match(src, /elapsedSeconds/);
  assert.match(src, /setInterval/);
  assert.match(src, /clearInterval\(timer\)/);
});

test("capability test panel i18n keys exist in both language catalogs", async () => {
  const { readFile } = await import("node:fs/promises");
  const en = JSON.parse(
    await readFile(new URL("../messages/en.json", import.meta.url), "utf8"),
  );
  const zh = JSON.parse(
    await readFile(new URL("../messages/zh.json", import.meta.url), "utf8"),
  );

  const keys = [
    "models_cap_panel_title",
    "models_cap_panel_empty",
    "models_cap_panel_confirm",
    "models_cap_panel_confirming",
    "models_cap_panel_confirmed",
    "models_cap_panel_verdict_pass",
    "models_cap_panel_verdict_inconclusive",
    "models_cap_probe_chat",
    "models_cap_probe_embeddings",
    "models_cap_probe_function_calling",
  ];
  for (const key of keys) {
    assert.equal(typeof en[key], "string", `en.${key} missing`);
    assert.equal(typeof zh[key], "string", `zh.${key} missing`);
    assert.ok(en[key].trim() && zh[key].trim());
  }
});
