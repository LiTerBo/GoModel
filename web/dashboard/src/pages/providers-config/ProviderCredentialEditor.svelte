<script>
  // Provider credential editor modal (create + edit), built on the shared
  // EditorDialog shell. Name and Type are immutable once a provider exists;
  // API keys and service-account secrets round-trip as "***********" masks
  // that preserve the stored value.
  //
  // Everything below Name is rendered from the selected type's credential
  // schema, so an operator only ever sees the fields that type actually uses.
  import EnabledToggle from "$lib/components/atoms/EnabledToggle.svelte";
  import Icon from "$lib/components/atoms/Icon.svelte";
  import EditorDialog from "$lib/components/organisms/EditorDialog.svelte";
  import ProviderCredentialField from "./ProviderCredentialField.svelte";
  import { providersConfig } from "./providersConfig.svelte.js";
  import {
    providerCredentialTypeOptions,
    providerDiscoveryState,
    providerServingToggleState,
    suggestProviderCredentialName,
  } from "./providersConfigLogic.js";
  import { timezone } from "$lib/stores/timezone.svelte.js";
  import { RefreshCw } from "lucide";
  import * as m from "$lib/paraglide/messages.js";

  const typeOptions = $derived(
    providerCredentialTypeOptions(providersConfig.types, providersConfig.form.type),
  );
  const fields = $derived(providersConfig.formFields);
  const nameError = $derived(providersConfig.fieldErrors.name || "");
  const typeError = $derived(providersConfig.fieldErrors.type || "");

  // Bound here rather than passed as a reference: the store's formatter reads
  // `this`, so a bare reference would lose its receiver.
  const formatFetchedAt = (value) => timezone.formatTimestamp(value);

  // The stored row is what "currently in effect" describes: the form may hold
  // an unsaved model list, and only a save changes what the gateway serves.
  const storedRow = $derived(
    providersConfig.rows.find(
      (row) =>
        String((row && row.name) || "").trim() ===
        String(providersConfig.form.name || "").trim(),
    ) || null,
  );
  const discoveryState = $derived(
    providersConfig.formMode === "edit"
      ? providerDiscoveryState(
          storedRow,
          providersConfig.runtimeFor(storedRow && storedRow.name),
          formatFetchedAt,
        )
      : null,
  );

  // Registration and serving are independent fields: the switch above writes the
  // credential's enabled flag (registration = the models enter the list at all),
  // the switch below writes the provider-scoped access policy (serving = the
  // listed models answer requests). Registration follows this dialog's own
  // switch, so what the operator sees is what gates the serving switch.
  const servingAccess = $derived(
    providersConfig.providerAccessFor(providersConfig.form.name),
  );
  const servingToggle = $derived(
    providerServingToggleState(servingAccess, {
      mode: providersConfig.formMode,
      registered: providersConfig.form.enabled,
      available: providersConfig.virtualModelsAvailable,
    }),
  );

  // onTypeChange resets the Name field to a fresh suggestion whenever the
  // Type selection changes while creating a provider (Type is immutable once
  // a provider exists, so this never runs in edit mode). The operator can
  // still edit the suggested name before saving.
  function onTypeChange() {
    providersConfig.selectType();
    if (providersConfig.formMode !== "create") {
      return;
    }
    providersConfig.form.name = suggestProviderCredentialName(
      providersConfig.rows,
      providersConfig.form.type,
    );
  }

  // A rejected save points at the field that caused it; move there so the
  // message is not left off-screen or inside a section the operator has to
  // find on their own.
  $effect(() => {
    const target = providersConfig.focusField;
    if (!target) {
      return;
    }
    providersConfig.focusField = "";
    const element = document.getElementById("provider-credential-" + target);
    if (element) {
      element.scrollIntoView({ block: "center" });
      element.focus({ preventScroll: true });
    }
  });
</script>

<EditorDialog
  open={providersConfig.formOpen}
  title={providersConfig.formMode === "edit" ? m.providers_edit() : m.providers_add()}
  ariaLabel={m.providers_editor()}
  error={providersConfig.error}
  submitting={providersConfig.formSubmitting}
  novalidate
  onclose={() => providersConfig.closeForm()}
  onsubmit={() => providersConfig.submitForm()}
>
  {#snippet headerHint()}
    <p class="form-hint">{m.providers_identity_help()}</p>
  {/snippet}

  <div class="form-field">
    <label class="form-field-label" for="provider-credential-type">
      {m.providers_type()}<span class="form-field-required" aria-hidden="true">*</span>
    </label>
    <select
      id="provider-credential-type"
      class="form-select"
      bind:value={providersConfig.form.type}
      disabled={providersConfig.formMode === "edit"}
      aria-invalid={typeError ? "true" : undefined}
      aria-describedby={typeError ? "provider-credential-type-error" : "provider-credential-type-hint"}
      onchange={onTypeChange}
      data-modal-autofocus
    >
      <option value="" disabled>{m.providers_select_type()}</option>
      {#each typeOptions as type (type)}
        <option value={type}>{type}</option>
      {/each}
    </select>
    {#if typeError}
      <small class="form-field-error" id="provider-credential-type-error" role="alert">{typeError}</small>
    {:else}
      {#if providersConfig.form.type === "openai-compatible"}
        <small class="form-hint" id="provider-credential-type-hint">{m.providers_type_openai_compatible_hint()}</small>
      {:else}
        <small class="form-hint" id="provider-credential-type-hint">{m.providers_type_help()}</small>
      {/if}
    {/if}
  </div>

  <div class="form-field">
    <label class="form-field-label" for="provider-credential-name">
      {m.providers_name()}<span class="form-field-required" aria-hidden="true">*</span>
    </label>
    <input
      id="provider-credential-name"
      type="text"
      class="mono"
      placeholder="my-openai"
      bind:value={providersConfig.form.name}
      disabled={providersConfig.formMode === "edit"}
      aria-invalid={nameError ? "true" : undefined}
      aria-describedby={nameError ? "provider-credential-name-error" : "provider-credential-name-hint"}
      oninput={() => providersConfig.clearFieldError("name")}
    />
    {#if nameError}
      <small class="form-field-error" id="provider-credential-name-error" role="alert">{nameError}</small>
    {:else if providersConfig.formMode === "create"}
      <small class="form-hint" id="provider-credential-name-hint">{m.providers_name_create_help()}</small>
    {:else}
      <small class="form-hint" id="provider-credential-name-hint">{m.providers_name_edit_help()}</small>
    {/if}
  </div>

  {#if !providersConfig.form.type}
    <p class="form-hint">{m.providers_pick_type()}</p>
  {/if}

  {#each fields.primary as field (field.name)}
    <ProviderCredentialField {field} />
  {/each}

  {#if discoveryState}
    <div class="form-field">
      <span class="form-field-label">{m.providers_discovery_state()}</span>
      <p class="form-hint" id="provider-credential-discovery-state">{discoveryState}</p>
      <button
        type="button"
        class="btn btn-with-icon"
        disabled={Boolean(providersConfig.refreshingName)}
        aria-describedby="provider-credential-discovery-state"
        onclick={() => providersConfig.refreshModels(providersConfig.form.name)}
      >
        <Icon icon={RefreshCw} class="form-action-icon" />
        <span>{m.providers_refresh_action({ name: providersConfig.form.name })}</span>
      </button>
      <small class="form-hint">{m.providers_refresh_hint()}</small>
    </div>
  {/if}

  <div class="form-field">
    <span class="form-field-label">{m.providers_registration_status()}</span>
    <div class="vm-status-row">
      <div class="vm-status-toggle">
        <EnabledToggle
          enabled={providersConfig.form.enabled}
          label={m.providers_provider_toggle()}
          text={providersConfig.form.enabled
            ? m.providers_registration_on()
            : m.providers_registration_off()}
          onclick={() => (providersConfig.form.enabled = !providersConfig.form.enabled)}
        />
      </div>
    </div>
    <small class="form-hint">{m.providers_registration_hint()}</small>
  </div>

  {#if servingToggle.visible}
    <div class="form-field">
      <span class="form-field-label">{m.providers_serving_status()}</span>
      <div class="vm-status-row">
        <div class="vm-status-toggle">
          <EnabledToggle
            enabled={servingAccess.effective_enabled}
            label={m.providers_serving_status()}
            text={servingAccess.effective_enabled
              ? m.providers_serving_on()
              : m.providers_serving_off()}
            disabled={servingToggle.disabled}
            ariaLabel={servingAccess.effective_enabled
              ? m.providers_serving_stop_action({ name: providersConfig.form.name })
              : m.providers_serving_start_action({ name: providersConfig.form.name })}
            onclick={() => providersConfig.toggleProviderAccess(providersConfig.form.name)}
          />
        </div>
      </div>
      <small class="form-hint">{m.providers_serving_hint()}</small>
      <small class="form-hint">{m.providers_serving_exception_hint()}</small>
      {#if servingToggle.reason === "unregistered"}
        <small class="form-hint" role="alert">{m.providers_serving_blocked_hint()}</small>
      {:else if servingToggle.reason === "managed"}
        <small class="form-hint" role="alert"
          >{m.providers_serving_managed_read_only({ name: providersConfig.form.name })}</small
        >
      {:else if servingToggle.reason === "unavailable"}
        <small class="form-hint" role="alert">{m.providers_serving_unavailable()}</small>
      {/if}
    </div>
  {/if}

  {#if fields.advanced.length > 0}
    <details
      class="mcp-server-advanced"
      open={providersConfig.advancedOpen}
      ontoggle={(event) => (providersConfig.advancedOpen = event.currentTarget.open)}
    >
      <summary>
        <span class="mcp-server-advanced-summary-copy">
          <span class="mcp-server-advanced-title">{m.providers_advanced()}</span>
          <span class="form-hint">{fields.advanced.map((field) => field.label).join(", ")}</span>
        </span>
      </summary>

      <div class="mcp-server-advanced-fields">
        {#each fields.advanced as field (field.name)}
          <ProviderCredentialField {field} />
        {/each}
      </div>
    </details>
  {/if}
</EditorDialog>
