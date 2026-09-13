// Admin/API error extraction and machine-code localization. Kept free of
// Svelte-runtime imports so pure page-logic modules (and their node:test
// suites) can use it directly; $lib/api/client.js re-exports both helpers for
// component/store code.
//
// The gateway answers in English on every surface (AGENTS.md). What the
// dashboard renders instead is a sentence from this module's catalog, chosen by
// the stable {error:{code}} the backend attaches; the English message is kept
// only where it carries specifics the sentence cannot (counts, the offending
// selector, store text). A code with no entry falls back to the English
// message unchanged, so an unmapped code is never a regression.

// Relative import, not $lib: the node:test suites load this file directly and
// cannot resolve Vite's aliases.
import * as m from "../paraglide/messages.js";

// CODE_TEXT maps each backend machine code the dashboard can receive to its
// console sentence. Entries whose sentence names contextual data read it from
// the second argument (apiErrorText(result, { source })) and stand down ("")
// when the caller cannot supply it, so the English message still shows.
// `detail: true` appends the server message after the sentence.
const CODE_TEXT = {
  // --- request lifecycle ---
  request_canceled: { text: () => m.apierr_request_canceled() },
  request_timeout: { text: () => m.apierr_request_timeout() },

  // --- model access ---
  model_access_denied: { text: () => m.apierr_model_access_denied() },
  model_not_found: { text: () => m.apierr_model_not_found(), detail: true },

  // --- rate limits and budgets (the server message carries the numbers) ---
  rate_limit_exceeded: { text: () => m.apierr_rate_limit_exceeded(), detail: true },
  rate_limit_check_failed: { text: () => m.apierr_rate_limit_check_failed(), detail: true },
  rate_limit_not_found: { text: () => m.apierr_rate_limit_not_found() },
  budget_exceeded: { text: () => m.apierr_budget_exceeded(), detail: true },
  budget_check_failed: { text: () => m.apierr_budget_check_failed(), detail: true },
  budget_not_found: { text: () => m.apierr_budget_not_found() },
  usage_status_failed: { text: () => m.apierr_usage_status_failed(), detail: true },

  // --- upstream provider ---
  // The fallback code a failed provider response carries; the provider's own
  // message rides along as the detail, which is the part worth reading.
  provider_error: { text: () => m.apierr_provider_error(), detail: true },

  // --- deployment, entitlements, identity ---
  feature_unavailable: { text: () => m.apierr_feature_unavailable(), detail: true },
  quota_templates_not_entitled: { text: () => m.apierr_quota_templates_not_entitled() },
  extension_authentication_failed: { text: () => m.apierr_extension_authentication_failed(), detail: true },
  dashboard_access_denied: { text: () => m.apierr_dashboard_access_denied() },
  user_managed: { text: () => m.apierr_user_managed(), detail: true },
  runtime_setting_not_found: { text: () => m.apierr_runtime_setting_not_found() },
  auth_key_issue_failed: { text: () => m.apierr_auth_key_issue_failed() },
  metadata_max_properties_exceeded: { text: () => m.apierr_metadata_max_properties_exceeded(), detail: true },
  unsupported_response_operation: { text: () => m.apierr_unsupported_response_operation(), detail: true },

  // --- virtual models (the sentences name the row, so they need `source`) ---
  virtual_model_locked: { text: () => m.vm_lock_change_blocked() },
  virtual_model_in_use: {
    text: (params) => (params.source ? m.vm_delete_blocked_notice({ source: params.source }) : ""),
  },
  virtual_model_kind_change: {
    text: (params) => (params.source ? m.vm_kind_change_blocked({ source: params.source }) : ""),
  },
};

// coveredErrorCodes lists every code this module can render, so the i18n suite
// can hold the backend inventory and this table together.
export const coveredErrorCodes = Object.freeze(Object.keys(CODE_TEXT));

// errorPayloadData accepts both shapes the callers hold: the getJSON/sendJSON
// envelope {ok, stale, status, data, res} and an already-unwrapped payload.
function errorPayloadData(result) {
  if (!result || typeof result !== "object") return null;
  if (result.data && typeof result.data === "object") return result.data;
  return result;
}

// apiErrorCode returns the machine-readable code of a failed call, "" when the
// answer carries none. The envelope matters: the code rides in data.error.code —
// reading a `body` property matched nothing.
export function apiErrorCode(result) {
  const data = errorPayloadData(result);
  const error = data ? data.error : null;
  if (!error || typeof error !== "object") {
    return "";
  }
  return typeof error.code === "string" ? error.code : "";
}

// serverMessage is the English sentence the backend sent in a getJSON/sendJSON
// envelope: data.message or data.error, with data.error.message last.
function serverMessage(result) {
  const data = errorPayloadData(result);
  if (!data) return "";
  const error = data.error;
  const candidates = [
    data.message,
    typeof error === "string" ? error : null,
    error && typeof error === "object" ? error.message : null,
  ];
  for (const msg of candidates) {
    if (typeof msg === "string" && msg.trim()) return msg.trim();
  }
  return "";
}

// payloadMessage is the narrow shape a raw payload is allowed to carry:
// {error:{message}} and nothing else, so a caller that holds one cannot be
// handed a stray top-level field by mistake.
function payloadMessage(data) {
  const error = data && typeof data === "object" ? data.error : null;
  const message = error && typeof error === "object" ? error.message : null;
  return typeof message === "string" && message.trim() ? message.trim() : "";
}

// apiErrorText renders the console sentence for a failed call, "" when the code
// has no entry (or needs a param the caller did not supply) — callers then show
// the English message. Deliberately never returns a fallback of its own so the
// English text stays reachable.
export function apiErrorText(result, params = {}) {
  const code = apiErrorCode(result);
  const entry = code ? CODE_TEXT[code] : null;
  if (!entry) return "";
  const sentence = entry.text(params || {});
  if (!sentence) return "";
  const detail = entry.detail ? serverMessage(result) : "";
  return detail && detail !== sentence ? `${sentence} ${detail}` : sentence;
}

// errorMessage renders the failure of a getJSON/sendJSON result envelope: the
// console sentence when the code is mapped, the English message otherwise, and
// the caller's fallback when the answer carried neither.
export function errorMessage(result, fallback, params = {}) {
  return apiErrorText(result, params) || serverMessage(result) || fallback;
}

// errorPayloadMessage does the same for a raw payload ({error:{message}}), which
// is the shape the playground reads straight off the gateway's /v1 answers. It
// keeps the narrow extraction: only {error:{message}} counts as a message.
export function errorPayloadMessage(data, fallback, params = {}) {
  return apiErrorText(data, params) || payloadMessage(data) || fallback;
}

// errorPayloadProvider reads {error:{provider}} — the upstream provider a
// gateway error originated from, "" when the gateway raised it itself.
export function errorPayloadProvider(data) {
  const error = data && typeof data === "object" ? data.error : null;
  return error && typeof error === "object" && typeof error.provider === "string"
    ? error.provider.trim()
    : "";
}

// isGatewayAuthError tells a rejected gateway credential (the dashboard key is
// wrong or lacks access — reopen the key dialog) from an upstream provider
// rejecting its own key (report it like any other request error). A 401 with no
// readable payload is treated as the gateway's.
export function isGatewayAuthError(data) {
  const error = data && typeof data === "object" ? data.error : null;
  if (!error || typeof error !== "object" || Array.isArray(error)) return true;
  return String(error.type || "") === "authentication_error" && !errorPayloadProvider(data);
}
