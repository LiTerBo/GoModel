// Virtual models (redirects/aliases, load balancers, access policies) list
// state for the Models page: fetching, display rows, and the row actions.
// The editor form lives in virtualModelEditor.svelte.js.

import { errorMessage, getJSON, sendJSON } from "$lib/api/client.js";
import { flash } from "$lib/stores/flash.svelte.js";
import * as m from "$lib/paraglide/messages.js";
import { modelsStore } from "$lib/stores/models.svelte.js";
import {
  areAllGroupsExpanded,
  buildDisplayModels,
  buildGlobalScopeRow,
  filterDisplayModels,
  findModelOverrideView,
  groupDisplayModels,
  isGroupExpanded,
  rowAccessSelector,
  rowIsManaged,
  rowToggleBlocked,
  toggleAllGroups,
  toggleGroupOverride,
} from "./displayRows.js";
import {
  GLOBAL_OVERRIDE_SELECTOR,
  modelKeys,
  normalizedAliasName,
  qualifiedModelName,
} from "./modelIdentity.js";
import { computeRenderStep, initialRenderStep } from "./renderBatching.js";
import { modelAccessStateClass, splitVirtualModelViews } from "./routing.js";
import { buildAliasTogglePayload, buildModelTogglePayload } from "./vmForm.js";
import {
  apiErrorCode,
  deleteBlockedNotice,
  deleteForcePlan,
  parseAuthorizedByResponse,
  virtualModelErrorText,
} from "./vmImpactPreview.js";

class VirtualModelsStore {
  virtualModelsAvailable = $state(true);
  aliases = $state([]);
  modelOverrideViews = $state([]);
  modelRenderLimit = $state(0);
  modelRenderBatchSize = 75;
  modelsRendering = $state(false);
  #renderGeneration = 0;
  #aliasesLoaded = false;
  aliasLoading = $state(false);
  // Load failures only; mutation feedback goes through the flash store.
  aliasError = $state("");
  rowTogglingKey = $state("");
  rowDeletingKey = $state("");

  // ---- Group collapse (issue #14) ----
  // Two explicit key sets: collapsedGroups forces groups shut, expandedGroups
  // forces them open; keys absent from both follow defaultGroupExpanded (only
  // the virtual-model group starts open). Set objects are swapped, never
  // mutated, so $derived readers re-run. Session-only by design.
  collapsedGroups = $state(new Set());
  expandedGroups = $state(new Set());

  isGroupExpanded(key) {
    return isGroupExpanded(key, this.collapsedGroups, this.expandedGroups);
  }

  allGroupsExpanded() {
    return areAllGroupsExpanded(
      this.filteredDisplayModelGroups,
      this.collapsedGroups,
      this.expandedGroups,
    );
  }

  toggleGroupExpanded(key) {
    const next = toggleGroupOverride(
      key,
      this.collapsedGroups,
      this.expandedGroups,
      !this.isGroupExpanded(key),
    );
    this.collapsedGroups = next.collapsedGroups;
    this.expandedGroups = next.expandedGroups;
  }

  toggleAllGroupsExpanded(expand) {
    const next = toggleAllGroups(this.filteredDisplayModelGroups, expand);
    this.collapsedGroups = next.collapsedGroups;
    this.expandedGroups = next.expandedGroups;
  }

  // ---- Display rows (derived from the shared model inventory) ----

  displayModels = $derived(
    buildDisplayModels({
      models: modelsStore.models,
      aliases: this.aliases,
      virtualModelsAvailable: this.virtualModelsAvailable,
      activeCategory: modelsStore.activeCategory,
    }),
  );

  displayModelGroups = $derived(
    groupDisplayModels(
      this.displayModels,
      modelsStore.models,
      this.modelOverrideViews,
    ),
  );

  filteredDisplayModels = $derived(
    filterDisplayModels(this.displayModels, modelsStore.filter),
  );

  filteredDisplayModelGroups = $derived.by(() => {
    const filtered = this.filteredDisplayModels;
    const limit = Math.max(
      0,
      Math.min(Number(this.modelRenderLimit || 0), filtered.length),
    );
    if (!modelsStore.filter && limit >= this.displayModels.length) {
      return this.displayModelGroups;
    }
    return groupDisplayModels(
      filtered.slice(0, limit),
      modelsStore.models,
      this.modelOverrideViews,
    );
  });

  // globalScopeRow exposes the global "/" scope as a toggle row, so the global
  // level reuses the same enable/restrict/disable switch as models, aliases,
  // and provider groups.
  globalScopeRow = $derived(
    buildGlobalScopeRow(modelsStore.models, this.modelOverrideViews),
  );

  modelsBusy() {
    return Boolean(modelsStore.loading || this.modelsRendering);
  }

  modelLoadingText() {
    if (modelsStore.loading) {
      return this.displayModels.length > 0
        ? m.models_refreshing()
        : m.models_loading();
    }
    const total = this.filteredDisplayModels.length;
    const visible = Math.min(Number(this.modelRenderLimit || 0), total);
    return m.models_rendering({ visible, total });
  }

  // ---- Incremental render batching (loader paints between batches) ----
  //
  // The Models page owns the lifecycle: an $effect calls
  // restartModelRendering(total) whenever the visible row set changes and
  // stopModelRendering() on teardown. Batches grow the window between
  // animation frames; the generation counter cancels a stale run when a
  // newer restart (or a stop) supersedes it.

  restartModelRendering(total) {
    const generation = ++this.#renderGeneration;
    const step = initialRenderStep(this.modelRenderBatchSize, total);
    this.modelRenderLimit = step.limit;
    this.modelsRendering = step.rendering;
    if (step.rendering) {
      this.#scheduleRenderBatch(generation);
    }
  }

  stopModelRendering() {
    this.#renderGeneration++;
    this.modelsRendering = false;
  }

  #scheduleRenderBatch(generation) {
    const run = () => {
      if (generation !== this.#renderGeneration) {
        return;
      }
      const step = computeRenderStep(
        this.modelRenderLimit,
        this.modelRenderBatchSize,
        this.filteredDisplayModels.length,
      );
      this.modelRenderLimit = step.limit;
      this.modelsRendering = step.rendering;
      if (step.rendering) {
        this.#scheduleRenderBatch(generation);
      }
    };
    if (typeof requestAnimationFrame === "function") {
      requestAnimationFrame(() => setTimeout(run, 0));
    } else {
      setTimeout(run, 0);
    }
  }

  // ---- Fetch ----

  async fetchVirtualModels() {
    this.aliasLoading = true;
    this.aliasError = "";
    try {
      const result = await getJSON("/admin/virtual-models", {
        label: "virtual models",
      });
      if (result.status === 503) {
        this.virtualModelsAvailable = false;
        this.aliases = [];
        this.modelOverrideViews = [];
        return;
      }
      if (result.stale) {
        return;
      }
      this.virtualModelsAvailable = true;
      if (!result.ok) {
        this.aliases = [];
        this.modelOverrideViews = [];
        return;
      }
      const { aliases, policies } = splitVirtualModelViews(result.data);
      this.aliases = aliases;
      this.modelOverrideViews = policies;
    } catch (e) {
      console.error("Failed to fetch virtual models:", e);
      this.aliases = [];
      this.modelOverrideViews = [];
      this.aliasError = m.models_load_failed();
    } finally {
      this.aliasLoading = false;
    }
  }

  // ensureAliasesLoaded fetches the list at most once for pages that only need
  // the names (the allowlist pickers on the API Keys and Users pages).
  async ensureAliasesLoaded() {
    if (this.#aliasesLoaded || this.aliasLoading) {
      return;
    }
    await this.fetchVirtualModels();
    this.#aliasesLoaded = true;
  }

  // ---- Lookups ----

  qualifiedModelName(model) {
    return qualifiedModelName(model);
  }

  findModelOverrideView(selector) {
    return findModelOverrideView(this.modelOverrideViews, selector);
  }

  hasGlobalModelOverride() {
    return Boolean(this.findModelOverrideView(GLOBAL_OVERRIDE_SELECTOR));
  }

  findExistingAliasByName(name) {
    const normalizedName = normalizedAliasName(name);
    if (!normalizedName) {
      return null;
    }
    for (const alias of this.aliases) {
      if (normalizedAliasName(alias && alias.name) === normalizedName) {
        return alias;
      }
    }
    return null;
  }

  findConcreteModelByName(name) {
    const normalizedName = normalizedAliasName(name);
    if (!normalizedName) {
      return null;
    }
    for (const model of modelsStore.models) {
      if (modelKeys(model).has(normalizedName)) {
        return model;
      }
    }
    return null;
  }

  // ---- Row enable/disable toggle (real models, aliases, groups, global) ----

  rowToggleEnabled(row) {
    if (!row) {
      return false;
    }
    if (row.is_alias) {
      return row.alias && row.alias.enabled !== false;
    }
    return Boolean(row.access && row.access.effective_enabled !== false);
  }

  rowToggleLabel(row) {
    if (this.rowTogglingKey && this.rowTogglingKey === row.key) {
      return m.models_updating();
    }
    if (this.rowToggleRestricted(row)) {
      return m.models_restricted();
    }
    return this.rowToggleEnabled(row)
      ? m.models_enabled()
      : m.models_disabled();
  }

  rowToggleRestricted(row) {
    return (
      Boolean(row) &&
      !row.is_alias &&
      modelAccessStateClass(row.access, this.virtualModelsAvailable) ===
        "is-restricted"
    );
  }

  // rowToggleBlocked: a provider whose provider-scoped policy is off takes its
  // whole model set off the shelf, so the per-model switch has nothing to
  // change; the provider group row keeps its own switch (it owns that policy).
  rowToggleBlocked(row) {
    return rowToggleBlocked(row, this.modelOverrideViews);
  }

  rowToggleAriaLabel(row) {
    if (!row) {
      return "";
    }
    if (this.rowToggleBlocked(row)) {
      // The provider is paused: say why this switch cannot be used instead of
      // naming an action that would not change anything.
      return m.models_toggle_provider_paused();
    }
    let subject;
    if (row.is_alias) {
      subject = m.models_alias_subject({
        name: String((row.alias && row.alias.name) || ""),
      });
    } else {
      subject = String(
        row.display_name ||
          (row.access && row.access.selector) ||
          m.models_model_subject(),
      );
    }
    return this.rowToggleEnabled(row)
      ? m.models_disable_action({ subject: subject.trim() })
      : m.models_enable_action({ subject: subject.trim() });
  }

  // virtualModelFailureText resolves a failed row write to a localized sentence:
  // the in-use guard (reported as a notice — the row switch has no dialog and no
  // holder tally), a server-side lock, or the caller's generic fallback. The
  // backend message stays English (AGENTS.md), so it is only the last resort.
  virtualModelFailureText(result, source, fallback) {
    if (apiErrorCode(result) === "virtual_model_in_use") {
      return deleteBlockedNotice(source);
    }
    return virtualModelErrorText(result) || errorMessage(result, fallback);
  }

  async toggleRowEnabled(row) {
    if (!this.virtualModelsAvailable) {
      return;
    }
    if (!row || this.rowTogglingKey === row.key) {
      return;
    }
    if (this.rowToggleBlocked(row)) {
      // The disabled switch already reflects this; the guard keeps a stale
      // click (or a programmatic call) from writing a policy that cannot act.
      flash.success(m.models_toggle_provider_paused());
      return;
    }
    if (rowIsManaged(row)) {
      flash.success(m.models_managed_read_only());
      return;
    }
    if (row.is_alias) {
      await this.toggleAliasRow(row);
      return;
    }
    await this.toggleModelRow(row);
  }

  async toggleAliasRow(row) {
    const alias = row.alias;
    if (!alias || !alias.name) {
      return;
    }

    this.rowTogglingKey = row.key;

    const payload = buildAliasTogglePayload(alias);
    try {
      const result = await sendJSON("/admin/virtual-models", "PUT", payload, {
        label: "alias state",
      });
      if (result.status === 503) {
        this.virtualModelsAvailable = false;
        flash.error(m.models_unavailable());
        return;
      }
      if (result.stale) {
        return;
      }
      if (!result.ok) {
        flash.error(
          result.status === 401
            ? m.common_authentication_required()
            : this.virtualModelFailureText(
                result,
                alias.name,
                m.models_alias_update_failed(),
              ),
        );
        return;
      }

      flash.success(
        payload.enabled ? m.models_alias_enabled() : m.models_alias_disabled(),
      );
      void this.fetchVirtualModels();
    } catch (e) {
      console.error("Failed to toggle alias state:", e);
      flash.error(m.models_alias_update_failed());
    } finally {
      this.rowTogglingKey = "";
    }
  }

  async toggleModelRow(row) {
    const selector = rowAccessSelector(row);
    if (!selector) {
      return;
    }
    const existingPolicy = this.findModelOverrideView(selector);
    const { method, payload, desired } = buildModelTogglePayload(
      selector,
      existingPolicy,
      row.access || {},
    );

    this.rowTogglingKey = row.key;

    try {
      const result = await sendJSON("/admin/virtual-models", method, payload, {
        label: "model access",
      });
      if (result.status === 503) {
        this.virtualModelsAvailable = false;
        flash.error(m.models_unavailable());
        return;
      }
      if (!(method === "DELETE" && result.status === 404)) {
        if (result.stale) {
          return;
        }
        if (!result.ok) {
          flash.error(
            result.status === 401
              ? m.common_authentication_required()
              : this.virtualModelFailureText(
                  result,
                  selector,
                  m.models_access_update_failed(),
                ),
          );
          return;
        }
      }

      flash.success(
        desired ? m.models_model_enabled() : m.models_model_disabled(),
      );
      void Promise.all([modelsStore.fetchModels(), this.fetchVirtualModels()]);
    } catch (e) {
      console.error("Failed to toggle model access:", e);
      flash.error(m.models_access_update_failed());
    } finally {
      this.rowTogglingKey = "";
    }
  }

  // ---- Row deletes ----

  async removeAliasRow(row) {
    if (!(
      row &&
      row.is_alias &&
      row.alias &&
      row.alias.name &&
      !row.alias.managed
    )) {
      return;
    }
    if (this.rowDeletingKey) {
      return;
    }
    const source = String(row.alias.name || "").trim();
    if (!source) {
      return;
    }
    await this.mutateVirtualModelRow({
      rowKey: row.key,
      confirmMessage: m.models_remove_alias_confirm({ source }),
      method: "DELETE",
      payload: { source },
      operation: "virtual model",
      failureMessage: m.models_remove_failed(),
      notice: m.models_removed(),
      ignoreNotFound: true,
    });
  }

  async removeRedirectRow(row) {
    const alias = row && row.masking_alias;
    if (!(row && !row.is_alias && alias && alias.name && !alias.managed)) {
      return;
    }
    if (this.rowDeletingKey) {
      return;
    }
    const source = String(alias.name || "").trim();
    if (!source) {
      return;
    }
    await this.mutateVirtualModelRow({
      rowKey: row.key,
      confirmMessage: m.models_remove_redirect_confirm({ source }),
      method: "PUT",
      payload: {
        source,
        user_paths: Array.isArray(alias.user_paths) ? alias.user_paths : [],
        description: String(alias.description || "").trim(),
        enabled: alias.enabled !== false,
        ...(alias.slowdown != null ? { slowdown: Number(alias.slowdown) } : {}),
      },
      operation: "virtual model redirect",
      failureMessage: m.models_redirect_remove_failed(),
      notice: m.models_redirect_removed(),
    });
  }

  async mutateVirtualModelRow(options) {
    if (this.rowDeletingKey) {
      return;
    }
    if (!window.confirm(options.confirmMessage)) {
      return;
    }

    this.rowDeletingKey = options.rowKey;

    const source = String(
      (options.payload && options.payload.source) || "",
    ).trim();

    try {
      let forcePending = false;
      // Two sends at most: the plain attempt, then the forced retry the in-use
      // guard asks for. Every other outcome ends here with an error flash.
      for (let attempt = 0; attempt < 2; attempt += 1) {
        const result = await sendJSON(
          "/admin/virtual-models",
          options.method,
          forcePending ? { ...options.payload, force: true } : options.payload,
          {
            label: options.operation,
          },
        );
        if (result.status === 503) {
          this.virtualModelsAvailable = false;
          flash.error(m.models_unavailable());
          return;
        }
        if (!(options.ignoreNotFound && result.status === 404)) {
          if (result.stale) {
            return;
          }
          if (!result.ok) {
            const plan = await this.deleteGuardPlan(result, {
              forcePending,
              source,
              payload: options.payload,
            });
            if (plan && window.confirm(plan.confirmMessage)) {
              forcePending = true;
              continue;
            }
            flash.error(
              result.status === 401
                ? m.common_authentication_required()
                : plan
                  ? deleteBlockedNotice(source)
                  : this.virtualModelFailureText(
                      result,
                      source,
                      options.failureMessage,
                    ),
            );
            return;
          }
        }
        this.virtualModelsAvailable = true;

        flash.success(options.notice);
        void Promise.all([
          modelsStore.fetchModels(),
          this.fetchVirtualModels(),
        ]);
        return;
      }
    } catch (e) {
      console.error(options.failureMessage, e);
      flash.error(options.failureMessage);
    } finally {
      this.rowDeletingKey = "";
    }
  }

  // deleteGuardPlan is the shared guard decision plus the tally it needs. The
  // inventory is fetched only when the answer really is the in-use guard, so a
  // plain failure costs no extra request; null means "render this failure as an
  // error", a plan means "confirm, then resend with force".
  async deleteGuardPlan(result, { forcePending, source, payload }) {
    if (forcePending || result.status !== 409) {
      return null;
    }
    if (apiErrorCode(result) !== "virtual_model_in_use") {
      return null;
    }
    return deleteForcePlan(result, {
      forcePending,
      source,
      payload,
      impact: await this.fetchDeleteImpact(source),
    });
  }

  // fetchDeleteImpact loads a stored row's holder inventory for the delete
  // guard; no new_targets, so the backend previews the definition it will
  // delete — the same set its 409 count comes from. null when unavailable, so
  // callers degrade to the count-free sentence instead of guessing.
  async fetchDeleteImpact(source) {
    const name = String(source || "").trim();
    if (!name) {
      return null;
    }
    try {
      const params = new URLSearchParams({ source: name });
      const result = await getJSON(
        "/admin/virtual-models/authorized-by?" + params.toString(),
        { label: "delete impact" },
      );
      if (!result.ok || result.stale) {
        return null;
      }
      return parseAuthorizedByResponse(result.data);
    } catch {
      return null;
    }
  }
}

export const virtualModels = new VirtualModelsStore();
