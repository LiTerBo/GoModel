// Source-contract assertions for the allowlist pickers (the established
// modelTest / group-collapse pattern: no render infra, so Svelte wiring is
// checked against the source text while the option building itself is covered
// by model-selectors.test.js).
import test from "node:test";
import assert from "node:assert/strict";

async function read(relativePath) {
  const { readFile } = await import("node:fs/promises");
  return readFile(new URL(relativePath, import.meta.url), "utf8");
}

const EDITORS = [
  ["../src/pages/auth-keys/AuthKeyEditor.svelte", "authKeySelectorOptions", "store.formOpen"],
  ["../src/pages/auth-keys/AuthKeyAllowedModelsEditor.svelte", "authKeySelectorOptions", "store.allowedModelsEditor.open"],
  ["../src/pages/users/UserEditor.svelte", "userSelectorOptions", "store.formOpen"],
];

for (const [path, builder, openFlag] of EDITORS) {
  test(`${path} offers virtual models by name in the allowlist picker`, async () => {
    const src = await read(path);

    assert.match(src, /\$pages\/models\/virtualModels\.svelte\.js/);
    // The builder receives the alias list, e.g.
    //   authKeySelectorOptions(modelsStore.models, virtualModels.aliases)
    assert.ok(
      src.includes(`${builder}(modelsStore.models, virtualModels.aliases)`),
      `${builder} receives virtualModels.aliases`,
    );
    // Lazy: the alias list is only fetched once the dialog is opened.
    assert.ok(
      src.includes(`if (${openFlag})`),
      `the alias load is guarded by ${openFlag}`,
    );
    assert.match(src, /virtualModels\.ensureAliasesLoaded\(\)/);
  });
}

test("both allowlist builders describe virtual models with the existing label", async () => {
  for (const path of [
    "../src/pages/auth-keys/authKeysLogic.js",
    "../src/pages/users/usersLogic.js",
  ]) {
    const src = await read(path);

    assert.match(src, /aliases,/);
    assert.match(src, /describeAlias: \(\) => m\.models_virtual_model\(\)/);
  }
});

test("the virtual-model store exposes a one-shot alias loader", async () => {
  const src = await read("../src/pages/models/virtualModels.svelte.js");

  assert.match(src, /#aliasesLoaded = false;/);
  assert.match(src, /async ensureAliasesLoaded\(\) \{/);
  assert.match(src, /if \(this\.#aliasesLoaded \|\| this\.aliasLoading\)/);
  assert.match(src, /await this\.fetchVirtualModels\(\)/);
  assert.match(src, /this\.#aliasesLoaded = true;/);
});

test("the option builder keeps the provider-first order and appends aliases last", async () => {
  const src = await read("../src/lib/utils/modelSelectors.js");

  assert.match(src, /providers\.map\(\(name\) => \(\{ value: name \+ "\/\*"/);
  assert.match(src, /selectors\.map\(\(selector\) => \(\{ value: selector/);
  assert.match(src, /virtualModels\.sort\(\(a, b\) => a\.value\.localeCompare\(b\.value\)\)/);
});
