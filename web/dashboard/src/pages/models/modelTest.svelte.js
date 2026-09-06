// Model capability testing (J-2): trigger offline probes per model and read
// results. Global-scope endpoints; the store keeps the last run per model so
// the models table can show freshness.

import { getJSON, sendJSON } from "$lib/api/client.js";

class ModelTestStore {
  // selector "provider/model" -> { running, results, error }
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

  async fetchResults() {
    const result = await getJSON("/admin/models/test-results");
    return result.ok ? result.data : {};
  }

  state(provider, model) {
    return this.runs[`${provider}/${model}`] ?? null;
  }
}

export const modelTest = new ModelTestStore();
