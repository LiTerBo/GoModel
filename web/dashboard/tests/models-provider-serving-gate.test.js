import test from "node:test";
import assert from "node:assert/strict";

import {
  providerServingPaused,
  rowToggleBlocked,
} from "../src/pages/models/displayRows.js";

// A provider-scoped access policy turned off takes that provider's whole model
// set off the shelf. Until now the Models page still let an operator toggle a
// single model of that provider, which reads as "this one is serving again"
// while nothing is actually served — the provider-level policy wins for
// ordinary rows and /v1/models stays empty.
//
// The gate is frontend-only and deliberately narrow: the provider group row
// owns the provider-level policy itself, so blocking it would strand a paused
// provider with no way back.

const pausedProviderViews = [
  { selector: "deepseek/", enabled: false, scope_kind: "provider" },
];

const servingProviderViews = [
  { selector: "deepseek/", enabled: true, scope_kind: "provider" },
];

function modelRow(providerName) {
  return {
    key: `model:${providerName}/some-model`,
    display_name: "some-model",
    provider_name: providerName,
    selector: `${providerName}/some-model`,
    is_alias: false,
  };
}

test("providerServingPaused reads a disabled provider-scoped policy", () => {
  assert.equal(providerServingPaused(pausedProviderViews, "deepseek"), true);
  assert.equal(providerServingPaused(servingProviderViews, "deepseek"), false);
  assert.equal(providerServingPaused([], "deepseek"), false);
  assert.equal(providerServingPaused(null, "deepseek"), false);
});

test("providerServingPaused matches the provider exactly", () => {
  assert.equal(providerServingPaused(pausedProviderViews, "omlx"), false);
  assert.equal(providerServingPaused(pausedProviderViews, ""), false);
  assert.equal(providerServingPaused(pausedProviderViews, null), false);
  // "deepseek/" must not match a provider whose name merely starts the same way.
  assert.equal(providerServingPaused(pausedProviderViews, "deepsee"), false);
});

test("rowToggleBlocked blocks a model row of a paused provider", () => {
  assert.equal(rowToggleBlocked(modelRow("deepseek"), pausedProviderViews), true);
  assert.equal(
    rowToggleBlocked(modelRow("deepseek"), servingProviderViews),
    false,
  );
  assert.equal(rowToggleBlocked(modelRow("omlx"), pausedProviderViews), false);
});

test("rowToggleBlocked blocks an alias row of a paused provider", () => {
  const aliasRow = {
    key: "alias:lite",
    display_name: "lite",
    provider_name: "deepseek",
    selector: "",
    is_alias: true,
  };

  assert.equal(rowToggleBlocked(aliasRow, pausedProviderViews), true);
  assert.equal(rowToggleBlocked(aliasRow, servingProviderViews), false);
});

test("rowToggleBlocked leaves the provider group row switchable", () => {
  const groupRow = {
    key: "provider-group:deepseek",
    provider_name: "deepseek",
    provider_type: "deepseek",
    display_name: "deepseek",
    rows: [],
  };

  assert.equal(rowToggleBlocked(groupRow, pausedProviderViews), false);
});

test("rowToggleBlocked leaves the virtual-models group row switchable", () => {
  const virtualGroupRow = {
    key: "virtual-models",
    is_virtual_models: true,
    provider_name: "",
    display_name: "virtual models",
    rows: [],
  };

  assert.equal(rowToggleBlocked(virtualGroupRow, pausedProviderViews), false);
});

test("AccessToggle disables itself and names the reason when the provider is paused", async () => {
  const { readFile } = await import("node:fs/promises");
  const toggle = await readFile(
    new URL("../src/pages/models/AccessToggle.svelte", import.meta.url),
    "utf8",
  );
  assert.match(
    toggle,
    /const rowBlocked = \$derived\(virtualModels\.rowToggleBlocked\(row\)\)/,
  );
  assert.match(
    toggle,
    /disabled=\{rowTogglingKey === row\.key \|\| !virtualModelsAvailable \|\| rowBlocked\}/,
  );

  const store = await readFile(
    new URL("../src/pages/models/virtualModels.svelte.js", import.meta.url),
    "utf8",
  );
  // The gate reads the live policy list the page already fetches.
  assert.match(
    store,
    /rowToggleBlocked\(row\) \{\s*return rowToggleBlocked\(row, this\.modelOverrideViews\);/,
  );
  // A stale click or a programmatic call cannot write a policy that cannot act.
  assert.match(store, /if \(this\.rowToggleBlocked\(row\)\) \{/);
  assert.match(store, /m\.models_toggle_provider_paused\(\)/);
});

test("rowToggleBlocked ignores the global scope row and unassigned rows", () => {
  const globalRow = {
    key: "global",
    provider_name: "",
    provider_type: "",
    display_name: "all models",
    rows: [],
  };
  const unassignedModelRow = modelRow("");

  assert.equal(rowToggleBlocked(globalRow, pausedProviderViews), false);
  assert.equal(rowToggleBlocked(unassignedModelRow, pausedProviderViews), false);
  assert.equal(rowToggleBlocked(null, pausedProviderViews), false);
});
