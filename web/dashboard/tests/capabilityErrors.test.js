import test from "node:test";
import assert from "node:assert/strict";

test("capabilityErrors store source wires the aggregate endpoint and keying", async () => {
  const { readFile } = await import("node:fs/promises");
  const src = await readFile(
    new URL("../src/pages/models/capabilityErrors.svelte.js", import.meta.url),
    "utf8",
  );
  assert.match(src, /\/admin\/models\/capability-errors/);
  assert.match(src, /capability_errors/);
  assert.match(src, /`\$\{row\.provider\}\/\$\{row\.model\}`/);
  assert.match(src, /\?\? null/);
});
