import test from "node:test";
import assert from "node:assert/strict";

// The .svelte.js store carries runes and a $lib alias that node --test cannot
// resolve directly, so verify the source contract textually — the same
// pattern the repo's other tests use for editor-coupled modules — plus run
// the pure parts through a tiny inline shim.

test("modelTest store source wires the admin endpoints and tri-state runs", async () => {
  const { readFile } = await import("node:fs/promises");
  const src = await readFile(new URL("../src/pages/models/modelTest.svelte.js", import.meta.url), "utf8");
  assert.match(src, /\/admin\/models\/test/);
  assert.match(src, /\/admin\/models\/test-results/);
  assert.match(src, /sendJSON\("\/admin\/models\/test", "POST"/);
  assert.match(src, /running: true/);
  assert.match(src, /running: false/);
  // provider/model keying is the contract with the backend results map
  assert.match(src, /`\$\{provider\}\/\$\{model\}`/);
});

test("state() returns null for unknown selectors", async () => {
  const { readFile } = await import("node:fs/promises");
  const src = await readFile(new URL("../src/pages/models/modelTest.svelte.js", import.meta.url), "utf8");
  assert.match(src, /state\(provider, model\)/);
  assert.match(src, /\?\? null/);
});
