<script>
  // One model/alias table row.
  import Icon from "$lib/components/atoms/Icon.svelte";
  import { runtimeConfig } from "$lib/stores/runtimeConfig.svelte.js";
  import TableActionButton from "$lib/components/atoms/TableActionButton.svelte";
  import { virtualModels } from "./virtualModels.svelte.js";
  import { virtualModelEditor } from "./virtualModelEditor.svelte.js";
  import { pricingOverrides } from "./pricingOverrides.svelte.js";
  import { rateLimits } from "$pages/rate-limits/rateLimits.svelte.js";
  import {
  aliasRowCanRemove,
  displayRowClass,
  hasAccessOverride,
  modelOverrideEditButtonClass,
  modelOverrideEditButtonLabel,
  rowAnchorID,
  rowIsManaged,
  rowRedirectCanRemove,
} from "./displayRows.js";
import {
  maskingFailsOver,
  maskingRoutingKind,
  maskingRoutingLabel,
} from "./routing.js";
  import AccessToggle from "./AccessToggle.svelte";
  import { modelTest } from "./modelTest.svelte.js";
  import { capabilityErrors } from "./capabilityErrors.svelte.js";
  import {
    panelKey,
    isPanelOpen,
    togglePanel,
    closePanel,
    runnableCaps,
  } from "./modelTestPanel.js";
  import {
    AlertTriangle,
    Box,
    CircleDollarSign,
    Eye,
    FlaskConical,
    Gauge,
    MessageSquare,
    Pencil,
    ShieldCheck,
    Split,
    Trash2,
    Wrench,
  } from "lucide";
  import { capabilityIconStates, capabilityIconTitle } from "./capabilityIcons.js";
  import * as m from "$lib/paraglide/messages.js";

  // columns: the active category's column spec from categoryColumns.js
  // (ModelTable renders the matching <thead> from the same spec).
  let { row, columns } = $props();

  // Bridge cross-module singleton stores through component-level $derived so
  // the template tracks them (Svelte 5 does not establish reactive deps on
  // module-scope object properties read directly in markup).
  const virtualModelsAvailable = $derived(virtualModels.virtualModelsAvailable);
  const rowDeletingKey = $derived(virtualModels.rowDeletingKey);
  const pricingOverridesAvailable = $derived(
    pricingOverrides.modelPricingOverridesAvailable,
  );
  const rateLimitsEnabled = $derived(rateLimits.rateLimitsEnabled());
  const modelTestState = $derived(
    modelTest.state(row.provider_name, row.model?.id),
  );

  const pricing = $derived(pricingOverrides.modelRowPricing(row));
  // Capability provenance (J-1): which verification streams confirmed this
  // real model's capabilities — offline probes ("test") and/or audit-log
  // traffic mining ("observed"). The per-capability icon strip renders this
  // three-way (confirmed/declared/hidden), so no separate badge is needed.
  const runtimeCapError = $derived(
    row.is_alias ? null : capabilityErrors.state(row.provider_name, row.model?.id),
  );
  const hasCapabilityError = $derived(
    Boolean(row.model?.metadata?.capability_error) || Boolean(runtimeCapError),
  );
  // Capability icon strip (phase D): one icon per capability, three states
  // (confirmed / declared / hidden). Aliases have no capability metadata.
  const capabilityIcons = $derived(capabilityIconStates(row));
  const capabilityIconSpec = {
    chat: { icon: MessageSquare },
    embeddings: { icon: Box },
    function_calling: { icon: Wrench },
    vision: { icon: Eye },
  };
  const capabilityIconLabels = {
    capability: (key) =>
      ({
        chat: m.models_cap_icon_chat(),
        embeddings: m.models_cap_icon_embeddings(),
        function_calling: m.models_cap_icon_function_calling(),
        vision: m.models_cap_icon_vision(),
      })[key] ?? key,
    confirmedTest: () => m.models_cap_state_confirmed_test(),
    confirmedObserved: () => m.models_cap_state_confirmed_observed(),
    declared: () => m.models_cap_state_declared(),
  };
  // capabilitySourcesOf re-reads provenance for the title suffix
  // (confirmed states distinguish probe vs traffic verification).
  function capabilityIconTooltip(entry) {
    const source = row.model?.metadata?.capability_sources?.[entry.key];
    return capabilityIconTitle(entry.key, entry.state, capabilityIconLabels, source);
  }

  // Inline test-result panel (phase E): the panel opens when the operator
  // runs a test (or toggles the flask button) and shows one row per probe
  // with its verdict, latency, and detail. State is per-row via a Set of
  // keys — the group-collapse encoding.
  let openPanels = $state(new Set());
  const rowPanelKey = $derived(
    row.is_alias ? null : panelKey(row.provider_name, row.model?.id),
  );
  const panelOpen = $derived(rowPanelKey !== null && isPanelOpen(openPanels, rowPanelKey));
  const testRun = $derived(modelTestState);
  const probeRows = $derived(Array.isArray(testRun?.results) ? testRun.results : []);
  const confirmableCaps = $derived(runnableCaps(testRun?.results));
  const canConfirm = $derived(
    !row.is_alias &&
      !testRun?.running &&
      !testRun?.confirming &&
      probeRows.length > 0 &&
      !testRun?.confirmed,
  );
  const probeLabels = {
    chat: () => m.models_cap_probe_chat(),
    embeddings: () => m.models_cap_probe_embeddings(),
    function_calling: () => m.models_cap_probe_function_calling(),
  };
  // Elapsed-seconds counter while a run is in flight (E-5): one interval per
  // open+running panel, torn down when the run completes or the panel closes.
  let elapsedSeconds = $state(0);
  $effect(() => {
    if (!panelOpen || !testRun?.running) {
      elapsedSeconds = 0;
      return;
    }
    const startedAt = Date.now();
    const timer = setInterval(() => {
      elapsedSeconds = Math.round((Date.now() - startedAt) / 1000);
    }, 1000);
    return () => clearInterval(timer);
  });
  function toggleResultPanel() {
    if (rowPanelKey === null) return;
    openPanels = togglePanel(openPanels, rowPanelKey);
  }
  function closeResultPanel() {
    if (rowPanelKey === null) return;
    openPanels = closePanel(openPanels, rowPanelKey);
  }
  async function runAndOpen() {
    toggleResultPanel();
    if (rowPanelKey !== null && !testRun?.running) {
      await modelTest.runTest(row.provider_name, row.model.id);
    }
  }
  async function confirmFromPanel() {
    if (rowPanelKey === null) return;
    const outcome = await modelTest.confirmFromProbes(row.provider_name, row.model.id);
    if (outcome?.ok !== false) {
      closeResultPanel();
    }
  }
  const configuredSlowdown = $derived(
    row.is_alias
      ? row.alias && row.alias.slowdown
      : row.masking_alias && row.masking_alias.slowdown != null
        ? row.masking_alias.slowdown
        : row.access && row.access.override && row.access.override.slowdown,
  );
  // What the virtual model over this real model does with its requests: the
  // icon, the secondary line, and the edit/remove labels all follow it.
  const globalFailover = $derived(runtimeConfig.booleanFlag("FAILOVER_ENABLED", true));
  const routingKind = $derived(
    row.masking_alias ? maskingRoutingKind(row.masking_alias, globalFailover) : "",
  );
  const routingPrefix = $derived(
    routingKind === "failover"
      ? m.models_falls_back_to()
      : routingKind === "balanced"
        ? m.models_balanced_with()
        : m.models_redirects_to(),
  );
  const routingTitle = $derived.by(() => {
    if (routingKind === "failover") return m.models_failover_title();
    if (routingKind === "balanced") {
      if (!globalFailover) return m.models_balanced_title_failover_global_off();
      return maskingFailsOver(row.masking_alias, globalFailover)
        ? m.models_balanced_title()
        : m.models_balanced_title_failover_off();
    }
    return m.models_redirect();
  });
  const routingEditLabel = $derived(
    routingKind === "failover"
      ? m.models_edit_failover({ name: row.display_name })
      : routingKind === "balanced"
        ? m.models_edit_balancing({ name: row.display_name })
        : m.models_edit_redirect({ name: row.display_name }),
  );
  const routingRemoveLabel = $derived(
    routingKind === "failover"
      ? m.models_remove_failover({ model: row.display_name })
      : routingKind === "balanced"
        ? m.models_remove_balancing({ model: row.display_name })
        : m.models_remove_redirect({ model: row.display_name }),
  );
  const routingRemovingLabel = $derived(
    routingKind === "failover"
      ? m.models_removing_failover({ model: row.display_name })
      : routingKind === "balanced"
        ? m.models_removing_balancing({ model: row.display_name })
        : m.models_removing_redirect({ model: row.display_name }),
  );

  const slowdown = $derived(
    Number(configuredSlowdown == null ? 0 : configuredSlowdown),
  );
</script>

<tr id={rowAnchorID(row) || undefined} class={displayRowClass(row)}>
  <td>
    <div class="model-name-cell">
      <div class="model-name-primary">
        <span class="mono font-size-md">{row.display_name}</span>
        {#if row.is_alias}
          <span class="model-kind-icon" role="img" aria-label={m.models_virtual_model()} title={m.models_virtual_model()}>
            <svg class="model-kind-icon-svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <path d="m12 3-1.9 5.8a2 2 0 0 1-1.3 1.3L3 12l5.8 1.9a2 2 0 0 1 1.3 1.3L12 21l1.9-5.8a2 2 0 0 1 1.3-1.3L21 12l-5.8-1.9a2 2 0 0 1-1.3-1.3L12 3Z"></path>
            </svg>
          </span>
        {/if}
        {#if !row.is_alias && row.masking_alias}
          <span class="model-kind-icon" role="img" aria-label={routingTitle} title={routingTitle}>
            {#if routingKind === "failover"}
              <Icon icon={ShieldCheck} class="model-kind-icon-svg" />
            {:else if routingKind === "balanced"}
              <Icon icon={Split} class="model-kind-icon-svg" />
            {:else}
              <svg class="model-kind-icon-svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                <circle cx="6" cy="19" r="3"></circle>
                <path d="M9 19h8.5a3.5 3.5 0 0 0 0-7h-11a3.5 3.5 0 0 1 0-7H15"></path>
                <circle cx="18" cy="5" r="3"></circle>
              </svg>
            {/if}
          </span>
        {/if}
        {#if rowIsManaged(row)}
          <span class="alias-kind-badge" title={m.models_managed_config()}>{m.models_config()}</span>
        {/if}
        {#if capabilityIcons.length > 0}
          <span class="capability-icon-row">
            {#each capabilityIcons as entry (entry.key)}
              {@const spec = capabilityIconSpec[entry.key]}
              <span
                class="capability-icon capability-icon-{entry.state}"
                role="img"
                aria-label={capabilityIconTooltip(entry)}
                title={capabilityIconTooltip(entry)}
              >
                <Icon icon={spec.icon} class="model-kind-icon-svg" />
                {#if entry.state === "confirmed"}
                  <svg class="capability-icon-check" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                    <path d="M20 6 9 17l-5-5"></path>
                  </svg>
                {/if}
              </span>
            {/each}
          </span>
        {/if}
        {#if hasCapabilityError}
          <span
            class="alias-kind-badge model-warn-badge"
            role="img"
            aria-label={m.models_cap_warn_title()}
            title={runtimeCapError
              ? `${m.models_cap_warn_title()} (${runtimeCapError.latest_kind}, ${runtimeCapError.occurrences})`
              : m.models_cap_warn_title()}
          >
            <Icon icon={AlertTriangle} class="model-kind-icon-svg" />
            {m.models_cap_warn()}
          </span>
        {/if}
      </div>
      {#if row.is_alias}
        <div class="model-name-secondary">
          {m.models_targets()} <span class="mono font-size-md">{row.secondary_name}</span>
        </div>
      {/if}
      {#if !row.is_alias && row.masking_alias}
        <div class="model-name-secondary">
          {routingPrefix} <span class="mono font-size-md">{maskingRoutingLabel(row.masking_alias, routingKind)}</span>
          {#if virtualModelsAvailable && rowRedirectCanRemove(row)}
            <button
              type="button"
              class="model-redirect-remove-btn mono"
              aria-label={rowDeletingKey === row.key ? routingRemovingLabel : routingRemoveLabel}
              title={rowDeletingKey === row.key ? routingRemovingLabel : routingRemoveLabel}
              disabled={Boolean(rowDeletingKey)}
              onclick={() => virtualModels.removeRedirectRow(row)}
            >[{m.models_remove()}]</button>
          {/if}
        </div>
      {/if}
      {#if slowdown > 0}
        <div class="model-name-secondary">
          {m.models_slowdown()} <span class="mono font-size-md">{m.models_inference_time({ value: slowdown })}</span>
        </div>
      {/if}
    </div>
  </td>
  {#each columns as col, i (i)}
    {@const hint = col.hint ? col.hint(row, pricing) : ""}
    <td class={col.class} title={hint || undefined}>
      {col.value(row, pricing)}{#if hint}<span class="price-hint" aria-hidden="true">*</span><span class="price-hint-text">{hint}</span>{/if}
    </td>
  {/each}
  <td class="model-row-actions col-actions">
    {#if row.is_alias}
      <div class="alias-actions-cell model-list-actions">
        <AccessToggle {row} />
        {#if virtualModelsAvailable && aliasRowCanRemove(row)}
          <TableActionButton
            label={rowDeletingKey === row.key
              ? m.models_removing_alias({ name: row.alias.name })
              : m.models_remove_alias({ name: row.alias.name })}
            class="table-action-btn-danger table-icon-btn"
            onclick={() => virtualModels.removeAliasRow(row)}
            disabled={Boolean(rowDeletingKey)}
          >
            <Icon icon={Trash2} class="table-icon-svg" />
          </TableActionButton>
        {/if}
        {#if virtualModelsAvailable}
          <TableActionButton
            label={m.models_edit_alias({ name: row.alias.name })}
            class="table-icon-btn table-action-btn-active"
            onclick={() => virtualModelEditor.openVirtualModelEditAlias(row.alias)}
          >
            <Icon icon={Pencil} class="table-icon-svg" />
          </TableActionButton>
        {/if}
      </div>
    {:else}
      <div class="alias-actions-cell model-list-actions">
        <AccessToggle {row} />
        {#if pricingOverridesAvailable}
          <TableActionButton
            label={pricingOverrides.modelPricingButtonLabel(m.models_model_pricing_for({ name: row.display_name }), pricingOverrides.hasModelPricingOverride(row))}
            class="table-icon-btn {pricingOverrides.modelPricingButtonClass(pricingOverrides.hasModelPricingOverride(row))}"
            onclick={() => pricingOverrides.openModelPricingOverrideEdit(row)}
          >
            <Icon icon={CircleDollarSign} class="table-icon-svg" />
          </TableActionButton>
        {/if}
        {#if rateLimitsEnabled && rateLimits.rateLimitInspectorModelID(row)}
          <TableActionButton
            label={rateLimits.rateLimitGaugeTitle(row.display_name, rateLimits.rateLimitGaugeClassForModel(row))}
            class="table-icon-btn {rateLimits.rateLimitGaugeClassForModel(row)}"
            onclick={() => rateLimits.openRateLimitInspectorForModel(row)}
          >
            <Icon icon={Gauge} class="table-icon-svg" />
          </TableActionButton>
        {/if}
        {#if row.provider_name && row.model?.id}
          <TableActionButton
            label={modelTestState?.running
              ? m.models_cap_test_running()
              : m.models_cap_test_action()}
            title={m.models_cap_test_action_title()}
            class="table-icon-btn {panelOpen ? 'table-action-btn-active' : ''}"
            disabled={Boolean(modelTestState?.running)}
            onclick={runAndOpen}
          >
            <Icon icon={FlaskConical} class="table-icon-svg" />
          </TableActionButton>
        {/if}
        {#if virtualModelsAvailable && row.masking_alias && row.masking_alias.name}
          <TableActionButton
            label={routingEditLabel}
            class="table-icon-btn table-action-btn-active"
            onclick={() => virtualModelEditor.openVirtualModelEditAlias(row.masking_alias)}
          >
            <Icon icon={Pencil} class="table-icon-svg" />
          </TableActionButton>
        {/if}
        {#if virtualModelsAvailable && !row.masking_alias}
          <TableActionButton
            label={modelOverrideEditButtonLabel(m.models_model_settings_for({ name: row.display_name }), hasAccessOverride(row.access))}
            class="table-icon-btn {modelOverrideEditButtonClass(hasAccessOverride(row.access))}"
            onclick={() => virtualModelEditor.openVirtualModelEditModel(row)}
          >
            <Icon icon={Pencil} class="table-icon-svg" />
          </TableActionButton>
        {/if}
      </div>
    {/if}
  </td>
</tr>
{#if panelOpen}
  <tr class="capability-test-panel-row">
    <td colspan={99}>
      <div class="capability-test-panel">
        <div class="capability-test-panel-head">
          <span class="capability-test-panel-title">{m.models_cap_panel_title()}</span>
          {#if testRun?.running}
            <span class="capability-test-panel-running">
              {m.models_cap_test_running()} {elapsedSeconds > 0 ? `${elapsedSeconds}s` : ""}
            </span>
          {:else if testRun?.error}
            <span class="capability-test-panel-error">{testRun.error}</span>
          {:else if testRun?.confirmed}
            <span class="capability-test-panel-confirmed">✓ {m.models_cap_panel_confirmed()}</span>
          {/if}
          <button
            type="button"
            class="capability-test-panel-close"
            aria-label={m.common_action_close()}
            onclick={closeResultPanel}
          >×</button>
        </div>
        {#if probeRows.length === 0 && !testRun?.running}
          <div class="capability-test-panel-empty">{m.models_cap_panel_empty()}</div>
        {:else}
          <ul class="capability-test-probes">
            {#each probeRows as probe (probe.probe)}
              <li class="capability-test-probe capability-test-probe-{probe.verdict}">
                <span class="capability-test-probe-name">{probe.probe in probeLabels ? probeLabels[probe.probe]() : probe.probe}</span>
                <span class="capability-test-probe-verdict">
                  {probe.verdict === "pass" ? `✓ ${m.models_cap_panel_verdict_pass()}` : `△ ${m.models_cap_panel_verdict_inconclusive()}`}
                </span>
                {#if probe.latency_ms != null}
                  <span class="capability-test-probe-latency mono">{probe.latency_ms}ms</span>
                {/if}
                {#if probe.detail}
                  <span class="capability-test-probe-detail">{probe.detail}</span>
                {/if}
              </li>
            {/each}
          </ul>
        {/if}
        {#if canConfirm}
          <div class="capability-test-panel-actions">
            <button
              type="button"
              class="btn btn-primary capability-test-confirm-btn"
              disabled={testRun?.confirming || confirmableCaps.length === 0}
              onclick={confirmFromPanel}
            >
              {testRun?.confirming ? m.models_cap_panel_confirming() : m.models_cap_panel_confirm()}
            </button>
          </div>
        {/if}
      </div>
    </td>
  </tr>
{/if}

<style>
  .model-name-cell {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .model-name-primary {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
    align-items: center;
  }

  .model-name-secondary {
    font-size: 12px;
    color: var(--text-muted);
  }

  .model-redirect-remove-btn {
    appearance: none;
    margin-left: 4px;
    padding: 0;
    border: 0;
    background: none;
    color: var(--danger);
    font-size: 11px;
    cursor: pointer;
  }

  .model-redirect-remove-btn:hover:not(:disabled) {
    text-decoration: underline;
  }

  .model-redirect-remove-btn:disabled {
    opacity: 0.45;
    cursor: default;
  }

  .model-kind-icon {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 24px;
    height: 24px;
    flex: 0 0 24px;
    border: 1px solid color-mix(in srgb, var(--accent) 55%, var(--border));
    border-radius: 999px;
    background: var(--bg);
    color: var(--accent);
  }

  /* :global so the rule also reaches the SVG rendered by the Icon component. */
  .model-kind-icon :global(.model-kind-icon-svg) {
    width: 14px;
    height: 14px;
  }

  /* Capability icon strip (phase D): one icon per capability with three
     provenance states. Compact and icon-only per the UI conventions; the
     tooltip carries the capability name and verification wording. */
  .capability-icon-row {
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }

  .capability-icon {
    position: relative;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 20px;
    height: 20px;
    flex: 0 0 20px;
    border-radius: 999px;
  }

  .capability-icon :global(.model-kind-icon-svg) {
    width: 12px;
    height: 12px;
  }

  /* Confirmed: accent-filled, plus a tiny corner checkmark. */
  .capability-icon-confirmed {
    background: color-mix(in srgb, var(--accent) 18%, var(--bg));
    border: 1px solid var(--accent);
    color: var(--accent);
  }

  .capability-icon-confirmed :global(.capability-icon-check) {
    position: absolute;
    right: -3px;
    bottom: -3px;
    width: 9px;
    height: 9px;
    padding: 1px;
    border-radius: 999px;
    background: var(--accent);
    color: var(--bg);
  }

  /* Declared: outlined and dimmed — present but not verified. */
  .capability-icon-declared {
    border: 1px solid color-mix(in srgb, var(--text-muted) 45%, transparent);
    color: var(--text-muted);
    opacity: 0.75;
  }

  /* Inline test-result panel (phase E): a full-width row under the model,
     mirroring the provider-group banding. */
  .capability-test-panel-row > td {
    padding: 0 12px 12px;
    background: color-mix(in srgb, var(--accent) 4%, var(--bg));
  }

  .capability-test-panel {
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 10px 12px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .capability-test-panel-head {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .capability-test-panel-title {
    font-weight: 600;
    font-size: 12px;
  }

  .capability-test-panel-running {
    font-size: 12px;
    color: var(--accent);
  }

  .capability-test-panel-error {
    font-size: 12px;
    color: var(--danger);
  }

  .capability-test-panel-confirmed {
    font-size: 12px;
    color: var(--success, var(--accent));
  }

  .capability-test-panel-close {
    appearance: none;
    margin-left: auto;
    border: 0;
    background: none;
    color: var(--text-muted);
    font-size: 16px;
    line-height: 1;
    cursor: pointer;
    padding: 0 4px;
  }

  .capability-test-panel-close:hover {
    color: var(--foreground, var(--text));
  }

  .capability-test-panel-empty {
    font-size: 12px;
    color: var(--text-muted);
  }

  .capability-test-probes {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .capability-test-probe {
    display: flex;
    align-items: baseline;
    gap: 10px;
    font-size: 12px;
    flex-wrap: wrap;
  }

  .capability-test-probe-name {
    min-width: 90px;
    font-weight: 500;
  }

  .capability-test-probe-pass .capability-test-probe-verdict {
    color: var(--success, var(--accent));
  }

  .capability-test-probe-inconclusive .capability-test-probe-verdict {
    color: var(--warning, var(--text-muted));
  }

  .capability-test-probe-latency {
    color: var(--text-muted);
  }

  .capability-test-probe-detail {
    color: var(--text-muted);
    font-size: 11px;
  }

  .capability-test-panel-actions {
    display: flex;
    justify-content: flex-end;
  }

  .capability-test-confirm-btn {
    font-size: 12px;
    padding: 4px 12px;
  }

  .model-row-actions {
    text-align: right;
    width: 170px;
  }

  @media (max-width: 768px) {
    .model-name-primary {
        flex-direction: column;
        align-items: flex-start;
      }

    .model-row-actions {
        width: auto;
      }
  }
</style>
