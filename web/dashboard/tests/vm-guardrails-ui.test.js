import test from "node:test";
import assert from "node:assert/strict";

// T13-F RED: pure logic for the alias-evolution guardrail UI. The editor gets
// an impact preview grouped by verdict, a lock toggle, and a delete guard that
// turns the backend's 409 into a force confirm; the allowed-models pickers
// badge the alias entries that follow the name. All of these decisions are
// pure functions so node:test can pin them down without a DOM.

import {
  groupImpactGrants,
  impactPreviewLine,
  shouldPreviewImpact,
} from "../src/pages/models/vmImpactPreview.js";
import {
  aliasBadge,
  aliasHoverTitle,
  applyLockFields,
  buildImpactPreviewTargets,
  lockToggleHelp,
} from "../src/pages/models/vmForm.js";
import { buildVirtualModelSavePayload } from "../src/pages/models/vmForm.js";
import { parseAuthorizedByResponse } from "../src/pages/models/vmImpactPreview.js";
import { buildAliasAnnotate } from "../src/pages/auth-keys/authKeysLogic.js";

// --- parseAuthorizedByResponse --------------------------------------------

test("parseAuthorizedByResponse keeps the server's ordering and summary", () => {
  const parsed = parseAuthorizedByResponse({
    source: "smart",
    old_targets: ["openai/gpt-4o"],
    new_targets: ["openai/gpt-4o-mini"],
    grants: [
      {
        kind: "credential",
        id: "7",
        label: "hms",
        change: "follow",
        matched_by: "name",
        matched: ["openai/gpt-4o", "openai/gpt-4o-mini"],
      },
      {
        kind: "user_path",
        id: "/acme",
        label: "/acme",
        change: "potential",
        matched_by: "target",
        matched: ["openai/gpt-4o"],
      },
    ],
    summary: { follow: 1, potential: 1, unrestricted: 0 },
  });
  assert.equal(parsed.source, "smart");
  assert.deepEqual(parsed.oldTargets, ["openai/gpt-4o"]);
  assert.deepEqual(parsed.newTargets, ["openai/gpt-4o-mini"]);
  assert.equal(parsed.grants.length, 2);
  assert.deepEqual(parsed.summary, { follow: 1, potential: 1, unrestricted: 0 });
  // Unknown verdicts do not crash the preview; they render in the catch-all.
  const weird = parseAuthorizedByResponse({
    grants: [{ kind: "credential", id: "1", label: "x", change: "future" }],
  });
  assert.equal(weird.grants.length, 1);
  assert.equal(weird.summary.follow + weird.summary.potential + weird.summary.unrestricted, 0);
});

test("parseAuthorizedByResponse tolerates a missing body", () => {
  const parsed = parseAuthorizedByResponse(null);
  assert.deepEqual(parsed.grants, []);
  assert.deepEqual(parsed.summary, { follow: 0, potential: 0, unrestricted: 0 });
});

// --- groupImpactGrants ------------------------------------------------------

test("groupImpactGrants buckets grants into follow, potential, unrestricted", () => {
  const groups = groupImpactGrants([
    { kind: "user_path", label: "/b", change: "potential" },
    { kind: "credential", label: "a", change: "follow" },
    { kind: "credential", label: "c", change: "unrestricted" },
    { kind: "user_path", label: "/a", change: "potential" },
  ]);
  assert.deepEqual(
    groups.follow.map((g) => g.label),
    ["a"],
  );
  assert.deepEqual(
    groups.potential.map((g) => g.label),
    ["/b", "/a"],
  );
  assert.deepEqual(
    groups.unrestricted.map((g) => g.label),
    ["c"],
  );
  // Groups without members are omitted so the template can iterate directly.
  assert.equal(groups.none, undefined);
});

// --- impactPreviewLine ------------------------------------------------------

test("impactPreviewLine says possibly affected, never will lose", () => {
  // D2 wording constraint: coarse-grained preview, so the copy must stay
  // hedged ("possibly affected") for potential, while follow is a fact.
  assert.match(impactPreviewLine("follow"), /follows|跟随/);
  assert.match(impactPreviewLine("potential"), /[Pp]ossibl|可能/);
});

// --- shouldPreviewImpact ----------------------------------------------------

test("shouldPreviewImpact only previews redirect edits with a source", () => {
  assert.equal(shouldPreviewImpact({ mode: "edit", isRedirect: true, source: "smart" }), true);
  assert.equal(shouldPreviewImpact({ mode: "create", isRedirect: true, source: "new" }), true);
  // Access policies carry no pointing to change.
  assert.equal(shouldPreviewImpact({ mode: "edit", isRedirect: false, source: "smart" }), false);
  assert.equal(shouldPreviewImpact({ mode: "edit", isRedirect: true, source: "" }), false);
});

// --- buildImpactPreviewTargets ----------------------------------------------

test("buildImpactPreviewTargets mirrors the save payload's target list", () => {
  // Single plain alias: back-compat target_model, no targets array.
  const plain = buildVirtualModelSavePayload(
    {
      ...{ source: "smart", target_model: "openai/gpt-4o", strategy: "round_robin" },
      targets: [],
    },
    "",
    "create",
  );
  assert.deepEqual(buildImpactPreviewTargets(plain.payload), ["openai/gpt-4o"]);

  // Load-balanced: the payload carries targets in order.
  const balanced = buildVirtualModelSavePayload(
    {
      source: "lb",
      target_model: "openai/gpt-4o",
      targets: [
        { provider: "openai", model: "gpt-4o-mini", weight: 1 },
      ],
      strategy: "round_robin",
    },
    "",
    "create",
  );
  assert.deepEqual(buildImpactPreviewTargets(balanced.payload), [
    "openai/gpt-4o",
    "openai/gpt-4o-mini",
  ]);
});

// --- lockedSubmitGesture (D11) ----------------------------------------------

test("applyLockFields keeps the lock when the operator does not touch it", () => {
  // Stored state is true, form unchanged → omitted → backend preserves lock.
  const untouched = { locked: true, unlockRequested: false };
  const p1 = {};
  applyLockFields(p1, untouched, true);
  assert.equal(p1.locked, undefined);
  assert.equal(p1.unlock, undefined);

  // UnlockRequested=true → unlock gesturing.
  const p2 = {};
  applyLockFields(p2, { locked: true, unlockRequested: true }, true);
  assert.equal(p2.unlock, true);

  // Toggle off: stored was locked, form is unlocked → explicit locked: false.
  const p3 = {};
  applyLockFields(p3, { locked: false, unlockRequested: false }, true);
  assert.equal(p3.locked, false);

  // Toggle on: stored was unlocked, form is locked → explicit locked: true.
  const p4 = {};
  applyLockFields(p4, { locked: true, unlockRequested: false }, false);
  assert.equal(p4.locked, true);

  // Fallback (no originalLocked): toggle-off still works.
  const p5 = {};
  applyLockFields(p5, { locked: false, unlockRequested: false });
  assert.equal(p5.locked, false);

  // Fallback (no originalLocked): toggle-on is skipped (needs originalLocked).
  const p6 = {};
  applyLockFields(p6, { locked: true, unlockRequested: false });
  assert.equal(p6.locked, undefined);
});

test("lockToggleHelp explains what the lock freezes", () => {
  assert.ok(lockToggleHelp().length > 0);
});

// --- delete guard: 409 -> force confirm --------------------------------------

// The 409 handling itself lives in the store (network code); here the pure
// decision is which message to show for which status.
import { deleteBlockedMessage } from "../src/pages/models/vmImpactPreview.js";

test("deleteBlockedMessage keys off the 409 code", () => {
  const blocked = deleteBlockedMessage(
    { status: 409, body: { error: { code: "virtual_model_in_use", message: "2 credential(s)" } } },
    () => "fallback",
  );
  assert.notEqual(blocked, "fallback");
  assert.match(blocked, /force|强制/);
  assert.equal(deleteBlockedMessage({ status: 400, body: {} }, () => "fallback"), "fallback");
});

// --- alias badge (D4) ---------------------------------------------------------

test("aliasBadge marks alias values with the follow tag", () => {
  const aliases = [
    { name: "smart", targets: [{ model: "openai/gpt-4o" }] },
    { name: "lb", targets: [{ model: "openai/gpt-4o" }, { model: "groq/llama" }] },
  ];
  assert.deepEqual(aliasBadge("smart", aliases), { follows: true });
  assert.deepEqual(aliasBadge("lb", aliases), { follows: true });
  assert.equal(aliasBadge("openai/gpt-4o", aliases), null);
  assert.equal(aliasBadge("openai/*", aliases), null);
  // Degraded (aliases not loaded yet): no badge, no crash.
  assert.equal(aliasBadge("smart", []), null);
  assert.equal(aliasBadge("smart", null), null);
});

test("aliasHoverTitle shows the current pointing for aliases only", () => {
  const aliases = [
    { name: "smart", targets: [{ provider: "openai", model: "gpt-4o" }] },
  ];
  assert.match(aliasHoverTitle("smart", aliases), /openai\/gpt-4o/);
  assert.equal(aliasHoverTitle("openai/gpt-4o", aliases), "");
  assert.equal(aliasHoverTitle("smart", []), "");
});

test("buildAliasAnnotate marks alias values in the option list", () => {
  const aliases = [
    { name: "smart", targets: [{ model: "openai/gpt-4o" }, { model: "openai/gpt-4o-mini" }] },
    { name: "fast", targets: [{ model: "anthropic/claude-3-haiku" }] },
  ];
  const annotate = buildAliasAnnotate(aliases);
  assert.ok(annotate, "factory returns a function when aliases are loaded");

  const smart = annotate({ value: "smart" });
  assert.equal(smart.text, "follows alias");

  const concrete = annotate({ value: "openai/*" });
  assert.equal(concrete, null, "concrete selectors (wildcard) are not annotated");

  const noAliases = buildAliasAnnotate(null);
  assert.equal(noAliases, undefined, "returns undefined when aliases are null");
});
