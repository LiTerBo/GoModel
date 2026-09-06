// Runtime capability-error rollups (K-3): the models page reads the admin
// aggregate to light the WARN badge on models that recently served a
// type/capability mismatch. Data comes from GET /admin/models/capability-errors.

import { getJSON } from "$lib/api/client.js";

class CapabilityErrorsStore {
  // "provider/model" -> { latest_kind, last_seen, occurrences }
  byModel = $state({});
  loaded = $state(false);

  async refresh() {
    const result = await getJSON("/admin/models/capability-errors");
    if (result.ok) {
      const next = {};
      for (const row of result.data?.capability_errors ?? []) {
        if (!row.provider || !row.model) continue;
        next[`${row.provider}/${row.model}`] = row;
      }
      this.byModel = next;
      this.loaded = true;
    }
  }

  // state returns the rollup for a row, or null when the model is clean.
  state(provider, model) {
    if (!provider || !model) return null;
    return this.byModel[`${provider}/${model}`] ?? null;
  }
}

export const capabilityErrors = new CapabilityErrorsStore();
