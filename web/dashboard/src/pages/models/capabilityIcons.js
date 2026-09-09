// Capability icon strip (phase D): pure logic deciding which capability
// icons a model row shows and in which state. No Svelte, no $lib — relative
// imports only, so node:test can load it directly (the displayRows.js
// pattern).
//
// Three states, per the design decision Q6=A:
//   confirmed — a verification stream (offline probe "test" or traffic
//               mining "observed") positively confirmed the capability
//               (highlight + checkmark badge).
//   declared  — the capability flag is set by a static source (registry,
//               discovered, config, heuristic); shown outlined/dimmed.
//   hidden    — the capability is absent or explicitly false. Probes never
//               strip capabilities, so an explicit false is a confirmed
//               unsupported and there is nothing to advertise.
//
// The icon set mirrors the probe surface (chat / embeddings /
// function_calling) plus vision (Q4=B). Image/audio/video needs are carried
// by the category tabs, not the icon strip.

// Paraglide message functions, injected by the caller (ModelRow passes the
// `m` namespace) so this module stays free of import-shape assumptions.
// titleOf(capabilityKey, state) -> string.

export const CAPABILITY_ORDER = [
  { key: "chat", category: "text_generation" },
  { key: "embeddings", category: "embedding" },
  { key: "function_calling", capability: "function_calling" },
  { key: "vision", capability: "vision" },
];

const CONFIRMED_SOURCES = new Set(["test", "observed"]);

// Declared sources are every non-verification provenance the backend writes
// (core.CapSrc*): anything that set the flag without a verification stream.
// Unknown sources still count as declared — the flag exists, the strip shows
// it, only the title degrades to the generic declared wording.

function metadataOf(row) {
  if (!row || row.is_alias) return null;
  return row.model?.metadata ?? null;
}

function sourcesOf(metadata) {
  return metadata?.capability_sources ?? {};
}

// stateOf resolves one capability key to its strip state. "chat" and
// "embeddings" have no dedicated boolean flags on the backend — they ride
// the model's categories (text_generation / embedding) — so a category match
// counts as declared unless a capability_sources entry confirms it.
export function stateOf(metadata, capabilityKey) {
  if (!metadata) return "hidden";
  const sources = sourcesOf(metadata);
  const source = sources[capabilityKey];
  if (source && CONFIRMED_SOURCES.has(source)) return "confirmed";

  const spec = CAPABILITY_ORDER.find((c) => c.key === capabilityKey);
  if (spec?.capability) {
    return metadata.capabilities?.[spec.capability] ? "declared" : "hidden";
  }
  // Category-backed capability: the probe confirmation key ("chat" /
  // "embeddings") or the category presence.
  const categories = metadata.categories ?? [];
  if (source && metadata.capabilities?.[capabilityKey]) return "declared";
  if (spec?.category && categories.includes(spec.category)) return "declared";
  return "hidden";
}

// capabilityIconStates returns the visible icon descriptors in display
// order: [{ key, state }]. Hidden capabilities are omitted entirely.
export function capabilityIconStates(row) {
  const metadata = metadataOf(row);
  if (!metadata) return [];
  const states = [];
  for (const spec of CAPABILITY_ORDER) {
    const state = stateOf(metadata, spec.key);
    if (state !== "hidden") {
      states.push({ key: spec.key, state });
    }
  }
  return states;
}

// capabilityIconTitle composes the tooltip: capability name + provenance
// wording, e.g. "函数调用 · 实测验证". `labels` = {
//   capability: (capabilityKey) => name,
//   confirmedTest: () => probe-confirmed wording,
//   confirmedObserved: () => traffic-confirmed wording,
//   declared: () => static-declaration wording,
// } — shaped by the ModelRow caller from the paraglide namespace.
export function capabilityIconTitle(capabilityKey, state, labels, source) {
  const name = labels.capability(capabilityKey);
  const suffix =
    state === "confirmed"
      ? source === "observed"
        ? labels.confirmedObserved()
        : labels.confirmedTest()
      : labels.declared();
  return suffix ? `${name} · ${suffix}` : name;
}
