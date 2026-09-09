// Phase F: icon-only access toggle + capability badge consolidation.
// Source-contract tests (no component-render infra) plus catalog checks.

import test from "node:test";
import assert from "node:assert/strict";

test("AccessToggle renders track+thumb only, semantics live on aria-label", async () => {
  const { readFile } = await import("node:fs/promises");
  const src = await readFile(
    new URL("../src/pages/models/AccessToggle.svelte", import.meta.url),
    "utf8",
  );

  // The visible text span is gone; the track/thumb pair remains.
  assert.doesNotMatch(src, /<span>\{rowToggleLabel\}<\/span>/);
  assert.match(src, /alias-toggle-track/);
  assert.match(src, /alias-toggle-thumb/);
  // Semantics: aria-label still describes the action (screen readers and
  // the native tooltip both read it).
  assert.match(src, /aria-label=\{rowToggleAriaLabel\}/);
  assert.match(src, /title=\{rowToggleAriaLabel\}/);
});

test("ModelRow drops the tested/observed badges; the warn badge stays", async () => {
  const { readFile } = await import("node:fs/promises");
  const src = await readFile(
    new URL("../src/pages/models/ModelRow.svelte", import.meta.url),
    "utf8",
  );

  // The icon strip's three states now carry provenance, so the
  // "Verified by probe/traffic" badges are redundant.
  assert.doesNotMatch(src, /models_cap_src_test\(\)/);
  assert.doesNotMatch(src, /models_cap_src_observed\(\)/);
  // The capability-error badge is an alert, not provenance — it stays.
  assert.match(src, /models_cap_warn\(\)/);
  // The icon strip itself is untouched.
  assert.match(src, /capability-icon-row/);
});

test("ModelRow flask button explains the test purpose in its tooltip", async () => {
  const { readFile } = await import("node:fs/promises");
  const src = await readFile(
    new URL("../src/pages/models/ModelRow.svelte", import.meta.url),
    "utf8",
  );

  assert.match(src, /models_cap_test_action_title\(\)/);
});

test("capability badge i18n keys exist in both language catalogs", async () => {
  const { readFile } = await import("node:fs/promises");
  const en = JSON.parse(
    await readFile(new URL("../messages/en.json", import.meta.url), "utf8"),
  );
  const zh = JSON.parse(
    await readFile(new URL("../messages/zh.json", import.meta.url), "utf8"),
  );

  assert.equal(typeof en.models_cap_test_action_title, "string");
  assert.equal(typeof zh.models_cap_test_action_title, "string");
  assert.ok(en.models_cap_test_action_title.trim() && zh.models_cap_test_action_title.trim());
});
