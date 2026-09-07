import test from "node:test";
import assert from "node:assert/strict";

// No component-render test infra exists in this repo, so the Svelte side of
// the group-collapse feature is verified by source-contract assertions (the
// established modelTest.test.js pattern) plus svelte-check in CI. Pure state
// logic lives in displayRows.js and is covered by models-virtual-models.test.js.

test("ModelTable renders a group expander and hides collapsed rows", async () => {
  const { readFile } = await import("node:fs/promises");
  const src = await readFile(
    new URL("../src/pages/models/ModelTable.svelte", import.meta.url),
    "utf8",
  );

  // Header expander: accessible toggle mirroring the audit-log thread control.
  assert.match(src, /aria-expanded=/);
  assert.match(src, /toggleGroupExpanded\(group\.key\)/);
  assert.match(src, /[eE]xpanded\(group\)\s*\?\s*ChevronDown\s*:\s*ChevronRight/);
  assert.match(src, /common_action_collapse\(\)/);
  assert.match(src, /common_action_expand\(\)/);

  // Rows render only for expanded groups (collapse actually hides them).
  assert.match(src, /\{#if groupExpanded\(group\)\}/);
  assert.match(src, /\{#each group\.rows as row \(row\.key\)\}/);
});

test("ModelsPage toolbar offers the one-click expand/collapse-all control", async () => {
  const { readFile } = await import("node:fs/promises");
  const src = await readFile(
    new URL("../src/pages/models/ModelsPage.svelte", import.meta.url),
    "utf8",
  );

  assert.match(src, /toggleAllGroupsExpanded\(/);
  assert.match(src, /allGroupsExpanded\(\)/);
  assert.match(src, /models_expand_all\(\)/);
  assert.match(src, /models_collapse_all\(\)/);
});

test("expand/collapse-all labels exist in both language catalogs", async () => {
  const { readFile } = await import("node:fs/promises");
  const en = JSON.parse(
    await readFile(new URL("../messages/en.json", import.meta.url), "utf8"),
  );
  const zh = JSON.parse(
    await readFile(new URL("../messages/zh.json", import.meta.url), "utf8"),
  );

  for (const key of ["models_expand_all", "models_collapse_all"]) {
    assert.equal(typeof en[key], "string", `en.${key} missing`);
    assert.equal(typeof zh[key], "string", `zh.${key} missing`);
    assert.ok(en[key].trim() && zh[key].trim());
  }
});
