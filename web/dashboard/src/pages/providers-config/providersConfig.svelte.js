// State module for the Providers (provider credentials) page: networking +
// editor state on Svelte 5 runes.
//
// Endpoints:
//   GET    /admin/provider-credentials          — list rows
//   GET    /admin/provider-credentials/types    — types + their credential forms
//   PUT    /admin/provider-credentials          — upsert one row
//   DELETE /admin/provider-credentials/{name}   — delete one row
//   POST   /admin/providers/{name}/models/refresh — re-fetch one model list
//   GET    /admin/virtual-models                — access policies (switch state)
//   PUT    /admin/virtual-models                — write a provider-wide policy
//   DELETE /admin/virtual-models                — drop it again

import { errorPayloadMessage, getJSON, sendJSON } from "$lib/api/client.js";
import { loadAdminList, sendAdminMutation } from "$lib/api/adminCrud.js";
import { confirmDialog } from "$lib/stores/confirm.svelte.js";
import { flash } from "$lib/stores/flash.svelte.js";
import * as m from "$lib/paraglide/messages.js";
import { modelsStore } from "$lib/stores/models.svelte.js";
import { runtimeConfig } from "$lib/stores/runtimeConfig.svelte.js";
import {
  defaultProviderCredentialForm,
  filterProviderCredentials,
  mergeProviderRuntime,
  providerAccessState,
  providerAccessToggleRequest,
  providerCredentialFormFields,
  providerCredentialRowToForm,
  providerCredentialSchema,
  providerModelsRefreshPath,
  providerModelsRefreshSummary,
  resetProviderCredentialFields,
  validateProviderCredentialForm,
  buildProviderCredentialPayload,
} from "./providersConfigLogic.js";
import { Trash2 } from "lucide";

class ProvidersConfigState {
  rows = $state([]);
  available = $state(true);
  loading = $state(false);
  // Load failures, and the editor errors that belong to no single field;
  // anything blamed on one field goes to fieldErrors, and successful
  // mutations go through the flash store.
  error = $state("");
  filter = $state("");

  formOpen = $state(false);
  formSubmitting = $state(false);
  formMode = $state("create");
  advancedOpen = $state(false);
  form = $state(defaultProviderCredentialForm());
  // Per-field validation messages keyed by credential field name, from local
  // checks or from a rejected save's `param`. focusField names the input the
  // editor should move to; it is cleared once the editor has done so.
  fieldErrors = $state({});
  focusField = $state("");

  deletingName = $state("");
  deleteSubmitting = $state(false);
  // Name of the provider whose model list is being re-fetched, empty when no
  // refresh is in flight.
  refreshingName = $state("");

  // Provider-wide availability. Turning a provider off is an access policy on
  // the provider scope — a virtual-model row whose source is "<provider>/" —
  // which is the policy the Models page writes from its provider-group and
  // per-model Enabled switches. It is independent of the credential's own
  // enabled flag: a switched-off provider stays registered and keeps listing
  // its models, they are just not routable.
  virtualModels = $state([]);
  virtualModelsAvailable = $state(true);
  accessTogglingName = $state("");

  // Credential schemas: one per constructible provider type, naming the
  // fields that type accepts.
  types = $state([]);
  typesLoaded = $state(false);

  // Provider rows from GET /admin/providers/status. Discovery facts (how many
  // models a provider serves, when they were last fetched) belong to the
  // registry, not to the credential row, so they are fetched alongside and
  // merged into the rows for display.
  providerRuntime = $state([]);

  #controller = null;

  // rowsWithRuntime are the credential rows carrying their provider's
  // discovery facts, when the status response has any.
  get rowsWithRuntime() {
    return mergeProviderRuntime(this.rows, this.providerRuntime);
  }

  get filteredRows() {
    return filterProviderCredentials(this.rowsWithRuntime, this.filter);
  }

  // runtimeFor reports one provider's discovery facts, or null while the
  // status response has nothing for it.
  runtimeFor(name) {
    const target = String(name || "").trim();
    if (!target) {
      return null;
    }
    const provider = (this.providerRuntime || []).find(
      (item) => String((item && item.name) || "").trim() === target,
    );
    const runtime = provider && provider.runtime;
    if (!runtime) {
      return null;
    }
    return {
      discovered_model_count: Number(runtime.discovered_model_count) || 0,
      last_model_fetch_at: String(runtime.last_model_fetch_at || ""),
    };
  }

  // schema is the selected type's credential form, or null while the schemas
  // load (the editor then shows every field rather than none).
  get schema() {
    return providerCredentialSchema(this.types, this.form.type);
  }

  // providerAccessFor is the availability switch state for one provider row:
  // whether its models are currently available, and whether the policy behind
  // that is this page's to change.
  providerAccessFor(name) {
    return providerAccessState(
      this.virtualModels,
      name,
      runtimeConfig.modelsEnabledByDefault(),
    );
  }

  // formFields are the credential fields to render. Until a type is picked
  // there is nothing to show: which fields exist is the type's answer.
  get formFields() {
    if (!String(this.form.type || "").trim()) {
      return { primary: [], advanced: [] };
    }
    const schema = this.schema;
    return providerCredentialFormFields(schema, schema && schema.default_base_url);
  }

  // Schema load failures stay silent: the editor falls back to showing every
  // field, and the list request already reports availability problems.
  async fetchTypes() {
    const outcome = await loadAdminList("/admin/provider-credentials/types", {
      label: "provider credential types",
    });
    if (outcome.status !== "ok") {
      return;
    }
    this.types = outcome.items;
    this.typesLoaded = true;
  }

  async fetchPage() {
    if (this.#controller) this.#controller.abort();
    const controller = new AbortController();
    this.#controller = controller;
    this.loading = true;
    this.error = "";
    try {
      const outcome = await loadAdminList("/admin/provider-credentials", {
        label: "provider credentials",
        errorFallback: m.providers_load_failed(),
        unavailableStatuses: [503, 404],
        options: { signal: controller.signal },
      });
      if (outcome.status === "stale" || controller.signal.aborted) {
        return;
      }
      if (outcome.status === "unavailable") {
        this.available = false;
        this.rows = [];
        return;
      }
      if (outcome.status === "error") {
        // A gateway response proves the endpoint exists; a network failure
        // (result === null) leaves the availability flag as-is.
        if (outcome.result) {
          this.available = true;
        }
        this.rows = [];
        this.error = outcome.error;
        return;
      }
      this.available = true;
      this.rows = outcome.items;
      if (!this.typesLoaded) {
        await this.fetchTypes();
      }
      void this.#fetchProviderRuntime();
      void this.#fetchVirtualModels();
    } finally {
      if (this.#controller === controller) {
        this.#controller = null;
        this.loading = false;
      }
    }
  }

  // #fetchProviderRuntime loads the registry's per-provider discovery facts.
  // A failure leaves the rows without them rather than failing the page: the
  // model list itself is served by the credentials endpoint.
  async #fetchProviderRuntime() {
    try {
      const result = await getJSON("/admin/providers/status", {
        label: "provider status",
      });
      if (result.stale || !result.ok) {
        return;
      }
      const providers =
        result.data && Array.isArray(result.data.providers) ? result.data.providers : [];
      this.providerRuntime = providers;
    } catch (e) {
      console.error("Failed to fetch provider status:", e);
    }
  }

  // #fetchVirtualModels loads the access policies the availability switches
  // read. A 503 means this deployment has the virtual-model feature off: the
  // switches render disabled rather than claiming a state nothing enforces.
  async #fetchVirtualModels() {
    try {
      const result = await getJSON("/admin/virtual-models", {
        label: "virtual models",
      });
      if (result.stale) {
        return;
      }
      if (result.status === 503) {
        this.virtualModelsAvailable = false;
        this.virtualModels = [];
        return;
      }
      if (!result.ok) {
        return;
      }
      this.virtualModelsAvailable = true;
      this.virtualModels = Array.isArray(result.data) ? result.data : [];
    } catch (e) {
      console.error("Failed to fetch virtual models:", e);
    }
  }

  // toggleProviderAccess switches every model a provider serves on or off. One
  // provider-scoped access policy does it: the same row, written the same way,
  // as the Models page's provider-group and per-model Enabled switches, so both
  // pages agree on what "off" means. A policy declared in configuration is not
  // this page's to change, and says so instead of failing.
  async toggleProviderAccess(name) {
    const provider = String(name || "").trim();
    if (!provider || this.accessTogglingName) {
      return;
    }
    // The deployment default decides how a policy-less provider reads, and
    // whether re-enabling may simply drop the policy again.
    await runtimeConfig.ensureLoaded();
    const request = providerAccessToggleRequest(
      this.virtualModels,
      provider,
      runtimeConfig.modelsEnabledByDefault(),
    );
    if (!request) {
      if (this.providerAccessFor(provider).managed) {
        flash.success(m.providers_access_managed_read_only({ name: provider }));
      }
      return;
    }

    this.accessTogglingName = provider;
    try {
      const result = await sendJSON(
        "/admin/virtual-models",
        request.method,
        request.payload,
        { label: "provider model availability" },
      );
      if (result.status === 503) {
        this.virtualModelsAvailable = false;
        flash.error(m.providers_access_unavailable());
        return;
      }
      // Dropping a policy that is already gone is the outcome we wanted.
      if (!(request.method === "DELETE" && result.status === 404)) {
        if (result.stale) {
          return;
        }
        if (!result.ok) {
          flash.error(
            result.status === 401
              ? m.common_authentication_required()
              : errorPayloadMessage(result.data, m.providers_access_update_failed()),
          );
          return;
        }
      }
      flash.success(
        request.desired
          ? m.providers_access_enabled({ name: provider })
          : m.providers_access_disabled({ name: provider }),
      );
      // The models list carries each model's effective availability, and the
      // switches read the policy list back.
      modelsStore.fetchModels();
      void this.#fetchVirtualModels();
    } catch (e) {
      console.error("Failed to toggle provider model availability:", e);
      flash.error(m.providers_access_update_failed());
    } finally {
      this.accessTogglingName = "";
    }
  }

  // refreshModels re-fetches one provider's model inventory on demand
  // (POST /admin/providers/{name}/models/refresh), so a model the upstream
  // listing just started serving shows up without waiting out the background
  // refresh interval.
  async refreshModels(name) {
    const provider = String(name || "").trim();
    if (!provider || this.refreshingName) {
      return;
    }
    this.refreshingName = provider;
    try {
      const result = await sendJSON(providerModelsRefreshPath(provider), "POST", undefined, {
        label: "refresh provider models",
      });
      if (result.stale) {
        return;
      }
      if (!result.ok) {
        flash.error(m.providers_refresh_failed({ name: provider }));
        return;
      }
      flash.success(providerModelsRefreshSummary(result.data));
      this.#refreshInventory();
    } catch (e) {
      console.error("Failed to refresh provider models:", e);
      flash.error(m.providers_refresh_failed({ name: provider }));
    } finally {
      this.refreshingName = "";
    }
  }

  // #resetForm puts the editor in a known state: no leftover values, errors,
  // or open disclosure from the provider edited before this one.
  #resetForm(mode, form) {
    this.formMode = mode;
    this.form = form;
    this.advancedOpen = false;
    this.error = "";
    this.fieldErrors = {};
    this.focusField = "";
  }

  openCreate() {
    this.#resetForm("create", defaultProviderCredentialForm());
    this.formOpen = true;
    if (!this.typesLoaded) {
      this.fetchTypes();
    }
  }

  openEdit(row) {
    if (!row || row.managed) {
      return;
    }
    this.#resetForm("edit", providerCredentialRowToForm(row));
    this.formOpen = true;
    if (!this.typesLoaded) {
      this.fetchTypes();
    }
  }

  closeForm() {
    this.formOpen = false;
    this.#resetForm("create", defaultProviderCredentialForm());
  }

  // selectType reacts to a change of provider type: the form's shape changes
  // with it, so complaints and values belonging to the previous type go, and a
  // type that authenticates with a key opens with a row ready to paste into
  // rather than an "Add key" button the operator has to find first.
  //
  // Only creating clears: an existing provider's type is immutable, and there
  // the payload's echo of unrendered values is what keeps a stored setting the
  // form cannot show from being wiped.
  selectType() {
    this.fieldErrors = {};
    const fields = this.formFields;
    if (this.formMode === "create") {
      this.form = resetProviderCredentialFields(this.form, fields);
    }
    const apiKeys = fields.primary.find((field) => field.name === "api_keys");
    if (apiKeys && apiKeys.required && this.form.api_keys.length === 0) {
      this.form.api_keys = [{ value: "" }];
    }
  }

  // clearFieldError drops a field's message as soon as the operator edits it,
  // so a stale complaint never outlives the value it was about.
  clearFieldError(field) {
    if (this.fieldErrors[field] === undefined) {
      return;
    }
    const { [field]: _cleared, ...rest } = this.fieldErrors;
    this.fieldErrors = rest;
  }

  addApiKeyRow() {
    this.form.api_keys.push({ value: "" });
    this.clearFieldError("api_keys");
  }

  removeApiKeyRow(index) {
    this.form.api_keys.splice(index, 1);
    this.clearFieldError("api_keys");
  }

  // #reportSaveError routes a rejected save to the input that caused it. The
  // gateway names the offending credential field in `error.param`, using the
  // same names as the schema, so a server-side rule (which field combinations
  // actually authenticate) reads like a local validation error.
  #reportSaveError(data) {
    const message = errorPayloadMessage(data, m.providers_save_failed());
    const param = String(
      (data && data.error && typeof data.error === "object" && data.error.param) || "",
    ).trim();
    if (param && this.#isFormField(param)) {
      this.fieldErrors = { ...this.fieldErrors, [param]: message };
      this.error = "";
      this.#revealInvalidFields();
      return;
    }
    this.error = message;
  }

  #isFormField(name) {
    if (name === "name" || name === "type") {
      return true;
    }
    const { primary, advanced } = this.formFields;
    return [...primary, ...advanced].some((field) => field.name === name);
  }

  // #revealInvalidFields makes sure a complaint is visible: a field folded
  // into "Advanced settings" would otherwise be flagged behind a closed
  // disclosure.
  #revealInvalidFields() {
    const invalid = Object.keys(this.fieldErrors);
    if (invalid.length === 0) {
      return;
    }
    const { primary, advanced } = this.formFields;
    if (advanced.some((field) => invalid.includes(field.name))) {
      this.advancedOpen = true;
    }
    const order = ["type", "name", ...primary.map((f) => f.name), ...advanced.map((f) => f.name)];
    this.focusField = order.find((name) => invalid.includes(name)) || invalid[0];
  }

  // Hot-registering a provider changes the live model inventory, so refresh
  // the shared models store after every successful upsert/delete — and re-read
  // the per-provider discovery facts, which a new or removed provider changes.
  #refreshInventory() {
    modelsStore.fetchModels();
    modelsStore.fetchCategories();
    void this.#fetchProviderRuntime();
    void this.#fetchVirtualModels();
  }

  async submitForm() {
    const schema = this.schema;
    const fieldErrors = validateProviderCredentialForm(
      this.form,
      this.formMode,
      this.rows,
      schema,
    );
    if (Object.keys(fieldErrors).length > 0) {
      this.fieldErrors = fieldErrors;
      this.error = "";
      this.#revealInvalidFields();
      return;
    }

    const payload = buildProviderCredentialPayload(this.form, schema);
    this.error = "";
    this.fieldErrors = {};
    this.formSubmitting = true;
    try {
      const outcome = await sendAdminMutation("/admin/provider-credentials", "PUT", payload, {
        label: "save provider credential",
        errorFallback: m.providers_save_failed(),
        unavailableMessage: m.providers_unavailable(),
      });
      if (outcome.status === "stale") {
        return;
      }
      if (outcome.status === "unavailable") {
        this.available = false;
        this.error = outcome.error;
        return;
      }
      if (outcome.status === "error") {
        // Non-401 gateway rejections carry `error.param`, so route them to
        // the offending field; everything else lands in the form error slot.
        if (outcome.result && outcome.result.status !== 401) {
          this.#reportSaveError(outcome.result.data);
        } else {
          this.error = outcome.error;
        }
        return;
      }

      flash.success(m.providers_saved({ name: payload.name }));
      this.closeForm();
      this.#refreshInventory();
      void this.fetchPage();
    } finally {
      this.formSubmitting = false;
    }
  }

  async performDelete(name) {
    this.deleteSubmitting = true;
    this.deletingName = name;
    try {
      const outcome = await sendAdminMutation(
        "/admin/provider-credentials/" + encodeURIComponent(name),
        "DELETE",
        undefined,
        {
          label: "delete provider credential",
          errorFallback: m.providers_delete_failed(),
          unavailableMessage: m.providers_unavailable(),
        },
      );
      if (outcome.status === "stale") {
        return;
      }
      if (outcome.status === "unavailable") {
        this.available = false;
        confirmDialog.error = outcome.error;
        return;
      }
      if (outcome.status === "error") {
        confirmDialog.error = outcome.error;
        return;
      }

      flash.success(m.providers_deleted({ name }));
      confirmDialog.close();
      if (this.formOpen && this.form.name === name) {
        this.closeForm();
      }
      this.#refreshInventory();
      void this.fetchPage();
    } finally {
      this.deleteSubmitting = false;
      this.deletingName = "";
    }
  }

  // requestDelete drives the shared typed-confirmation dialog rather than
  // window.confirm: deleting a provider credential can silently break every
  // request routed to it, so the operator must type the provider name back.
  requestDelete(name) {
    const target = String(name || "").trim();
    if (!target || this.deleteSubmitting) {
      return;
    }
    const row = (this.rows || []).find(
      (item) => String((item && item.name) || "").trim() === target,
    );
    if (row && row.managed) {
      return;
    }
    confirmDialog.open({
      title: m.providers_delete_title(),
      titleId: "providerCredentialDeleteDialogTitle",
      inputId: "provider-credential-delete-confirmation",
      message: m.providers_delete_message({ name: target }),
      requiredText: target,
      confirmLabel: m.providers_delete_title(),
      icon: Trash2,
      dialogClass: "budget-reset-dialog",
      onConfirm: () => this.performDelete(target),
    });
  }
}

export const providersConfig = new ProvidersConfigState();
