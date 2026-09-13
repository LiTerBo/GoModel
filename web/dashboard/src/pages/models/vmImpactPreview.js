// The alias-evolution guardrail preview: pure logic for the impact block in
// the virtual-model editor and the delete guard's 409 handling. No Svelte, so
// the node:test suite exercises it directly; relative imports keep it loadable
// outside Vite.

import * as m from "../../lib/paraglide/messages.js";

// parseAuthorizedByResponse shapes GET /admin/virtual-models/authorized-by's
// payload for the editor. Unknown verdicts ride along (they render in the
// catch-all group) but never inflate the summary.
export function parseAuthorizedByResponse(body) {
  const source = body && typeof body.source === "string" ? body.source : "";
  const oldTargets = Array.isArray(body && body.old_targets)
    ? body.old_targets.map(String)
    : [];
  const newTargets = Array.isArray(body && body.new_targets)
    ? body.new_targets.map(String)
    : [];
  const grants = (Array.isArray(body && body.grants) ? body.grants : []).map(
    (grant) => ({
      kind: String((grant && grant.kind) || ""),
      id: String((grant && grant.id) || ""),
      label: String((grant && grant.label) || (grant && grant.id) || ""),
      change: String((grant && grant.change) || ""),
      matchedBy: String((grant && grant.matched_by) || ""),
      matched: Array.isArray(grant && grant.matched)
        ? grant.matched.map(String)
        : [],
    }),
  );
  const raw = (body && body.summary) || {};
  return {
    source,
    oldTargets,
    newTargets,
    grants,
    summary: {
      follow: Number(raw.follow) || 0,
      potential: Number(raw.potential) || 0,
      unrestricted: Number(raw.unrestricted) || 0,
    },
  };
}

// The three buckets the backend can report, in presentation order. "none" is
// implicit: a grant the backend does not flag is not shown at all.
const IMPACT_GROUPS = ["follow", "potential", "unrestricted"];

// groupImpactGrants buckets grants by verdict, keeping the server's order
// inside each bucket. Empty groups are omitted so a template can iterate the
// object directly.
export function groupImpactGrants(grants) {
  const groups = {};
  for (const grant of Array.isArray(grants) ? grants : []) {
    const key = IMPACT_GROUPS.includes(grant.change) ? grant.change : "potential";
    if (!IMPACT_GROUPS.includes(grant.change)) {
      // An unknown verdict would silently re-caption itself; keep unknowns
      // out of the summary-driven groups entirely instead.
      continue;
    }
    (groups[key] = groups[key] || []).push(grant);
  }
  return groups;
}

// impactPreviewLine is the one-line verdict caption. Wording is part of the
// design (D2): "follow" is a fact the key holds today, "potential" must stay
// hedged because the coarse preview cannot prove ±1, and "unrestricted" is a
// coverage fact of the key.
export function impactPreviewLine(change) {
  switch (change) {
    case "follow":
      return m.vm_impact_follow_caption();
    case "potential":
      return m.vm_impact_potential_caption();
    case "unrestricted":
      return m.vm_impact_unrestricted_caption();
    default:
      return "";
  }
}

// shouldPreviewImpact decides whether the editor shows the impact block at
// all: only rows whose saving changes a pointing (redirects and load
// balancers) have something to preview; access policies do not.
export function shouldPreviewImpact({ mode, isRedirect, source }) {
  return Boolean(isRedirect && String(source || "").trim());
}

// deleteBlockedMessage maps a failed DELETE to the message the editor shows.
// The 409 virtual_model_in_use answer is a force-confirm prompt, not an
// error: it names the holders and points at the force action. Anything else
// falls through to the store's generic failure message.
export function deleteBlockedMessage(result, fallback) {
  if (!result || result.status !== 409) {
    return fallback();
  }
  const code =
    result.body &&
    result.body.error &&
    typeof result.body.error.code === "string"
      ? result.body.error.code
      : "";
  if (code !== "virtual_model_in_use") {
    return fallback();
  }
  const detail =
    result.body && result.body.error
      ? String(result.body.error.message || "")
      : "";
  return m.vm_delete_blocked_confirm({ detail });
}
