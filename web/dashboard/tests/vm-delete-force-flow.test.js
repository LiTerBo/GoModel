// The delete guard's force flow, shared by both entries that can delete a
// virtual model: the models-page row action and the editor.
//
// Background: the 409 virtual_model_in_use guard is meant to be escapable —
// the operator confirms once more and the request is resent with force: true.
// The editor walked that flow, the models-page row did not: it stopped at a
// notice ("a forced delete is required") with no way to force it from the page
// the operator was on. The decision is a pure function now so both entries
// consume the same one and cannot drift apart again; the wiring itself is
// checked against the source text (the modelTest / group-collapse pattern).
import test from "node:test";
import assert from "node:assert/strict";

import {
  deleteForcePlan,
  parseAuthorizedByResponse,
} from "../src/pages/models/vmImpactPreview.js";

async function read(relativePath) {
  const { readFile } = await import("node:fs/promises");
  return readFile(new URL(relativePath, import.meta.url), "utf8");
}

function squash(src) {
  return src.replace(/\s+/g, " ");
}

// The envelope getJSON/sendJSON answer: the machine code rides in data.error.
// The prose deliberately names counts that differ from the inventory fixture
// (9/7) so a sentence built from the backend message instead of the catalog's
// tally cannot pass below.
const IN_USE_409 = {
  ok: false,
  status: 409,
  data: {
    error: {
      code: "virtual_model_in_use",
      param: "force",
      message:
        'virtual model "smart" is still reachable by 9 credential(s) and 7 user path(s); send force to delete it',
    },
  },
};

const INVENTORY = parseAuthorizedByResponse({
  source: "smart",
  grants: [
    { kind: "credential", id: "a", change: "follow" },
    { kind: "credential", id: "b", change: "unrestricted" },
    { kind: "credential", id: "c", change: "follow" },
    { kind: "user_path", id: "/acme", change: "unrestricted" },
  ],
});

test("deleteForcePlan turns the in-use 409 into a force retry", () => {
  const plan = deleteForcePlan(IN_USE_409, {
    forcePending: false,
    source: "smart",
    payload: { source: "smart" },
    impact: INVENTORY,
  });

  assert.ok(plan, "the in-use guard must yield a plan");
  // The sentence is the catalog's, with the tally filled in — never the
  // backend's English prose (whose numbers differ from the tally on purpose).
  assert.match(plan.confirmMessage, /smart/);
  assert.match(plan.confirmMessage, /3/);
  assert.match(plan.confirmMessage, /1/);
  assert.ok(
    !plan.confirmMessage.includes("9 credential(s)"),
    `the backend message leaked into ${plan.confirmMessage}`,
  );
  assert.ok(
    !plan.confirmMessage.includes("send force to delete it"),
    `the backend message leaked into ${plan.confirmMessage}`,
  );
  assert.deepEqual(plan.retryPayload, { source: "smart", force: true });
});

test("deleteForcePlan is one-shot and applies to no other failure", () => {
  const context = { source: "smart", payload: { source: "smart" } };

  // Already forced: never build a second prompt (no confirm loop).
  assert.equal(
    deleteForcePlan(IN_USE_409, { ...context, forcePending: true }),
    null,
  );
  // A different 409 (server-side lock) is an error the operator has to fix.
  assert.equal(
    deleteForcePlan(
      { status: 409, data: { error: { code: "virtual_model_locked" } } },
      context,
    ),
    null,
  );
  // Same code but not a 409: not a guard answer at all.
  assert.equal(
    deleteForcePlan(
      { status: 400, data: { error: { code: "virtual_model_in_use" } } },
      context,
    ),
    null,
  );
  assert.equal(deleteForcePlan(undefined, context), null);
  assert.equal(deleteForcePlan({ status: 409, data: {} }, context), null);
});

test("deleteForcePlan keeps the original payload and only adds force", () => {
  const payload = { source: "q3", enabled: false, user_paths: ["/acme"] };
  const plan = deleteForcePlan(IN_USE_409, {
    forcePending: false,
    source: "q3",
    payload,
    impact: null,
  });

  assert.deepEqual(plan.retryPayload, {
    source: "q3",
    enabled: false,
    user_paths: ["/acme"],
    force: true,
  });
  // Without a tally the sentence still names the row instead of leaking prose.
  assert.match(plan.confirmMessage, /q3/);
  // The caller's payload object is left alone.
  assert.equal(payload.force, undefined);
});

test("the models-page row delete walks the force confirm too", async () => {
  const src = squash(await read("../src/pages/models/virtualModels.svelte.js"));

  // It asks the shared plan instead of stopping at the notice, confirms with
  // the plan's sentence and only then resends the same payload with force.
  assert.ok(src.includes("deleteForcePlan("), "row delete asks the shared plan");
  assert.ok(
    src.includes("if (plan && window.confirm(plan.confirmMessage))"),
    "row delete confirms with the plan's sentence",
  );
  assert.ok(
    src.includes("forcePending ? { ...options.payload, force: true } : options.payload"),
    "row delete resends the payload with force after the confirm",
  );
  // The tally the sentence needs is fetched on this entry too.
  assert.ok(
    src.includes("/admin/virtual-models/authorized-by?"),
    "row delete loads the holder tally",
  );
  // Declining the confirm is not a silent no-op: the reason still reaches the
  // operator as a flash.
  assert.ok(
    src.includes("deleteBlockedNotice(source)"),
    "declining the force confirm still explains the block",
  );
});

test("both delete entries share one plan and one inventory fetch", async () => {
  const editor = squash(
    await read("../src/pages/models/virtualModelEditor.svelte.js"),
  );

  // One implementation of the 409 decision and of the inventory fetch, so the
  // two entries cannot disagree about what the guard means.
  assert.ok(editor.includes("deleteForcePlan("), "editor uses the shared plan");
  assert.ok(
    editor.includes("virtualModels.fetchDeleteImpact("),
    "editor reuses the store's inventory fetch",
  );
});

test("the editor's forced retry does not re-ask the first confirm", async () => {
  const editor = squash(
    await read("../src/pages/models/virtualModelEditor.svelte.js"),
  );

  // Accepting the force confirm recurses into deleteVirtualModel; that pass
  // must skip the plain "remove the virtual model?" prompt it already asked,
  // or the operator sees the same question twice for one delete.
  assert.ok(
    editor.includes(
      "if ( !this.vmDeleteForcePending && !window.confirm(m.models_remove_policy_confirm({ source })) )",
    ),
    "the forced pass skips the first confirm",
  );
});
