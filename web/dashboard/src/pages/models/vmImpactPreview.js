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

// apiErrorCode returns the machine-readable code of a failed admin call, ""
// when the answer carries none. The envelope matters: getJSON/sendJSON answer
// {ok, stale, status, data, res}, so the code rides in data.error.code —
// reading a `body` property matched nothing.
export function apiErrorCode(result) {
  const error = result && result.data ? result.data.error : null;
  if (!error || typeof error !== "object") {
    return "";
  }
  return typeof error.code === "string" ? error.code : "";
}

// virtualModelErrorText renders a failed virtual-model write from its machine
// code, "" when the code has no catalog entry yet — callers then show the
// server message, which stays English for every backend surface (AGENTS.md:
// localization is the frontend's job, structure is the backend's). source is
// only needed by the codes whose sentence names the row.
export function virtualModelErrorText(result, source) {
  const code = apiErrorCode(result);
  if (code === "virtual_model_locked") {
    return m.vm_lock_change_blocked();
  }
  if (code === "virtual_model_kind_change") {
    return kindChangeBlockedText(source);
  }
  return "";
}

// kindChangeBlockedText renders the redirect-takeover guard: the write aimed at
// a selector that a virtual model owns. Saying which model owns it is what
// tells the operator to edit the redirect instead of the model row.
export function kindChangeBlockedText(source) {
  return m.vm_kind_change_blocked({ source: String(source || "").trim() });
}

// impactHolderCounts tallies the holders an impact payload reports, mirroring
// the backend's "reachable by N credential(s) and M user path(s)" guard count:
// every reported grant counts once, whatever its verdict (follow, potential or
// unrestricted).
export function impactHolderCounts(impact) {
  const counts = { credentials: 0, userPaths: 0 };
  const grants = impact && Array.isArray(impact.grants) ? impact.grants : [];
  for (const grant of grants) {
    if (grant && grant.kind === "credential") {
      counts.credentials += 1;
    } else if (grant && grant.kind === "user_path") {
      counts.userPaths += 1;
    }
  }
  return counts;
}

// deleteBlockedConfirm is the force-confirm sentence shown for a 409
// virtual_model_in_use: the backend names the holders in permanently English
// prose, so the sentence is built from the catalog plus the holder tally of
// GET /admin/virtual-models/authorized-by (the inventory the endpoint itself
// points at). An unavailable tally degrades to the count-free sentence instead
// of leaking that prose.
export function deleteBlockedConfirm(source, impact) {
  const name = String(source || "").trim();
  if (!impact) {
    return m.vm_delete_blocked_confirm_unknown({ source: name });
  }
  const counts = impactHolderCounts(impact);
  return m.vm_delete_blocked_confirm({
    source: name,
    credentials: counts.credentials,
    userPaths: counts.userPaths,
  });
}

// deleteBlockedNotice is the non-interactive counterpart of the confirm: the
// models-page row switch reports the same block as a flash, where counts are
// not at hand (no dialog, no preview fetch).
export function deleteBlockedNotice(source) {
  return m.vm_delete_blocked_notice({ source: String(source || "").trim() });
}

// deleteForcePlan decides the delete guard's interactive step: a 409
// virtual_model_in_use that has not been forced yet yields the force-confirm
// sentence plus the payload to resend, anything else yields null so the caller
// renders it as an error. Both delete entries — the models-page row action and
// the editor — consume this one decision; the row used to stop at the notice
// alone, which left the page it was clicked on with no way to force.
export function deleteForcePlan(result, { forcePending, source, impact, payload }) {
  if (forcePending) {
    return null;
  }
  if (!result || result.status !== 409) {
    return null;
  }
  if (apiErrorCode(result) !== "virtual_model_in_use") {
    return null;
  }
  const name = String(source || "").trim();
  return {
    confirmMessage: deleteBlockedConfirm(name, impact),
    retryPayload: { ...(payload || {}), source: name, force: true },
  };
}
