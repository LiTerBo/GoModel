// Model capability testing (J-2): trigger offline probes per model and read
// results. Global-scope endpoints; the store keeps the last run per model so
// the models table can show freshness.

import { getJSON, sendJSON } from "$lib/api/client.js";
import { runnableCaps } from "./modelTestPanel.js";

class ModelTestStore {
  // selector "provider/model" -> { running, results, error, confirming, confirmed }
  runs = $state({});

  async runTest(provider, model, probes) {
    const key = `${provider}/${model}`;
    this.runs = { ...this.runs, [key]: { ...(this.runs[key] ?? {}), running: true, error: null } };
    try {
      const result = await sendJSON("/admin/models/test", "POST", {
        provider,
        model,
        ...(probes ? { probes } : {}),
      });
      if (result.ok) {
        this.runs = { ...this.runs, [key]: { running: false, results: result.data, error: null } };
      } else {
        this.runs = { ...this.runs, [key]: { running: false, error: result.error ?? "request failed" } };
      }
    } catch (e) {
      this.runs = { ...this.runs, [key]: { running: false, error: String(e) } };
    }
    return this.runs[key];
  }

  // confirm persists the probe verdicts as operator-confirmed capabilities.
  // caps = { [capabilityKey]: boolean } as produced by runnableCaps(); source
  // is "test" (probe confirmation) or "observed" (traffic confirmation). The
  // backend writes the durable store first, then the in-memory registry, so
  // the confirmation survives restarts.
  async confirm(provider, model, caps, source = "test") {
    const key = `${provider}/${model}`;
    this.runs = {
      ...this.runs,
      [key]: { ...(this.runs[key] ?? {}), confirming: true, confirmed: null, error: null },
    };
    try {
      const result = await sendJSON("/admin/models/capabilities", "PUT", {
        provider,
        model,
        capabilities: caps,
      });
      if (result.ok) {
        this.runs = { ...this.runs, [key]: { ...(this.runs[key] ?? {}), confirming: false, confirmed: source } };
      } else {
        this.runs = {
          ...this.runs,
          [key]: { ...(this.runs[key] ?? {}), confirming: false, error: result.error ?? "confirm failed" },
        };
      }
    } catch (e) {
      this.runs = {
        ...this.runs,
        [key]: { ...(this.runs[key] ?? {}), confirming: false, error: String(e) },
      };
    }
    return this.runs[key];
  }

  // confirmFromProbes is the panel's one-click action: map the last probe
  // run's verdicts to capabilities and persist them with the probe source.
  async confirmFromProbes(provider, model) {
    const key = `${provider}/${model}`;
    const run = this.runs[key];
    const caps = {};
    for (const { key: capability, pass } of runnableCaps(run?.results)) {
      caps[capability] = pass;
    }
    if (Object.keys(caps).length === 0) {
      return { ok: false, error: "no probe results to confirm" };
    }
    return this.confirm(provider, model, caps, "test");
  }

  // confirmObserved persists a traffic-observation suggestion (W2b path):
  // same two-write endpoint, but the observed-capabilities route stamps the
  // "observed" source so provenance stays distinguishable.
  async confirmObserved(provider, model, caps) {
    const key = `${provider}/${model}`;
    this.runs = {
      ...this.runs,
      [key]: { ...(this.runs[key] ?? {}), confirming: true, confirmed: null, error: null },
    };
    try {
      const result = await sendJSON("/admin/models/observed-capabilities", "PUT", {
        provider,
        model,
        capabilities: caps,
      });
      if (result.ok) {
        this.runs = { ...this.runs, [key]: { ...(this.runs[key] ?? {}), confirming: false, confirmed: "observed" } };
      } else {
        this.runs = {
          ...this.runs,
          [key]: { ...(this.runs[key] ?? {}), confirming: false, error: result.error ?? "confirm failed" },
        };
      }
    } catch (e) {
      this.runs = {
        ...this.runs,
        [key]: { ...(this.runs[key] ?? {}), confirming: false, error: String(e) },
      };
    }
    return this.runs[key];
  }

  async fetchResults() {
    const result = await getJSON("/admin/models/test-results");
    return result.ok ? result.data : {};
  }

  state(provider, model) {
    return this.runs[`${provider}/${model}`] ?? null;
  }
}

export const modelTest = new ModelTestStore();
