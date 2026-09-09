// Inline test-result panel (phase E): pure state and mapping helpers for the
// per-row expansion and the confirm action. No Svelte, no $lib — relative
// imports only, so node:test can load it directly (the capabilityIcons.js
// pattern).

// panelKey mirrors the store's "provider/model" run key.
export function panelKey(provider, model) {
  return `${provider}/${model}`;
}

// Panel-open state: one Set of keys, replaced on every mutation (Svelte 5
// $state reassignment, mirroring the group-collapse encoding of issue #14).
export function isPanelOpen(openPanels, key) {
  return Boolean(openPanels && openPanels.has(key));
}

export function togglePanel(openPanels, key) {
  const next = new Set(openPanels || []);
  if (next.has(key)) {
    next.delete(key);
  } else {
    next.add(key);
  }
  return next;
}

export function closePanel(openPanels, key) {
  const next = new Set(openPanels || []);
  next.delete(key);
  return next;
}

// CAPABILITY_FOR_PROBE maps a backend probe name to the capability key the
// icon strip (capabilityIcons.js) uses. The chat probe has no boolean flag
// of its own — the routable capability is the text_generation category — so
// the confirmed icon after a successful chat probe lights up on that key.
const CAPABILITY_FOR_PROBE = {
  chat: "text_generation",
  embeddings: "embedding",
  function_calling: "function_calling",
};

// runnableCaps converts probe results into confirmable capabilities: one
// entry per known probe, pass=true for pass verdicts. Inconclusive probes
// map to pass=false, which records an explicitly confirmed unsupported —
// only because the operator pressed the button; probes alone never do
// (the backend refuses otherwise).
export function runnableCaps(results) {
  const rows = Array.isArray(results) ? results : [];
  const caps = [];
  for (const result of rows) {
    const capability = CAPABILITY_FOR_PROBE[result?.probe];
    if (!capability) continue;
    caps.push({ key: capability, pass: result.verdict === "pass" });
  }
  return caps;
}

// probeLabels maps probe names to i18n message functions; ModelRow builds it
// from the paraglide namespace (explicit literal keys — the i18n usage guard
// rejects dynamic lookups).
export function probeLabel(probe, labels) {
  const label = labels?.[probe];
  return label ? label() : probe;
}
