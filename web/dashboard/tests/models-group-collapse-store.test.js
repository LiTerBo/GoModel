import test from "node:test";
import assert from "node:assert/strict";

// The virtualModels store is a .svelte.js singleton (runes + $lib alias), so
// node --test cannot import it directly. Assert the wiring contract textually
// — the same pattern modelTest.test.js uses — while the pure collapse helpers
// themselves are covered by models-virtual-models.test.js.

test("virtualModels store carries two collapse sets wired to the pure helpers", async () => {
  const { readFile } = await import("node:fs/promises");
  const src = await readFile(
    new URL("../src/pages/models/virtualModels.svelte.js", import.meta.url),
    "utf8",
  );

  // Two explicit $state sets (issue #14 encoding; never persisted).
  assert.match(src, /collapsedGroups = \$state\(new Set\(\)\)/);
  assert.match(src, /expandedGroups = \$state\(new Set\(\)\)/);

  // State transitions go through the pure displayRows helpers.
  assert.match(src, /toggleGroupOverride\(/);
  assert.match(src, /toggleAllGroups\(/);
  assert.match(src, /areAllGroupsExpanded\(/);
  assert.match(src, /import \{[^}]*areAllGroupsExpanded[^}]*\} from "\.\/displayRows\.js"/s);

  // $state swaps must replace the Set objects (reactivity), not mutate.
  assert.match(src, /this\.collapsedGroups = next\.collapsedGroups/);
  assert.match(src, /this\.expandedGroups = next\.expandedGroups/);

  // Row toggle flips from the resolved current state.
  assert.match(src, /!this\.isGroupExpanded\(key\)/);
});
