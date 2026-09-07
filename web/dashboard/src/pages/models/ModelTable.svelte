<script>
  // Grouped models table for the active category — one component that
  // switches its columns per category.
  import Icon from "$lib/components/atoms/Icon.svelte";
  import TableActionButton from "$lib/components/atoms/TableActionButton.svelte";
  import { modelsStore } from "$lib/stores/models.svelte.js";
  import { virtualModels } from "./virtualModels.svelte.js";
  import { virtualModelEditor } from "./virtualModelEditor.svelte.js";
  import { pricingOverrides } from "./pricingOverrides.svelte.js";
  import { rateLimits } from "$pages/rate-limits/rateLimits.svelte.js";
  import {
    hasAccessOverride,
    isGroupExpanded,
    modelOverrideEditButtonClass,
    modelOverrideEditButtonLabel,
  } from "./displayRows.js";
  import AccessToggle from "./AccessToggle.svelte";
  import ModelGlobalActions from "./ModelGlobalActions.svelte";
  import ModelRow from "./ModelRow.svelte";
  import { categoryColumns, categoryColspan } from "./categoryColumns.js";
  import { ChevronDown, ChevronRight, CircleDollarSign, Gauge, Pencil } from "lucide";
  import * as m from "$lib/paraglide/messages.js";

  const category = $derived(modelsStore.activeCategory || "all");
  const columns = $derived(categoryColumns(category));
  const groupColspan = $derived(categoryColspan(category));

  // groups comes from the page as a reactive prop: Svelte 5 does not
  // re-track a child template's direct read of a cross-module singleton's
  // $derived value, so ModelsPage bridges filteredDisplayModelGroups.
  // The remaining reads are bridged for the same cross-module reason.
  // `collapse` bridges the group-collapse sets the same way (issue #14).
  let { groups, collapse } = $props();
  const pricingOverridesAvailable = $derived(
    pricingOverrides.modelPricingOverridesAvailable,
  );
  const rateLimitsEnabled = $derived(rateLimits.rateLimitsEnabled());
  const virtualModelsAvailable = $derived(virtualModels.virtualModelsAvailable);

  function groupExpanded(group) {
    return isGroupExpanded(group.key, collapse.collapsed, collapse.expanded);
  }

  function groupExpanderLabel(group, expanded) {
    return (
      group.display_name +
      ", " +
      (expanded ? m.common_action_collapse() : m.common_action_expand())
    );
  }
</script>

<div class="table-wrapper">
  <table class="data-table">
    <thead>
      <tr>
        <th>{m.models_column_model()}</th>
        {#each columns as col, i (i)}
          <th class={col.class}>
            {#each col.headerLines as line, li (li)}
              {#if li > 0}<br />{/if}{line}
            {/each}
          </th>
        {/each}
        <th class="model-actions-header col-actions"><ModelGlobalActions /></th>
      </tr>
    </thead>
    {#each groups as group (group.key)}
      <tbody>
        <tr class="provider-group-row">
          <td colspan={groupColspan}>
            <div class="provider-group-header">
              <div class="provider-group-meta">
                <div class="provider-group-title">
                  <button
                    type="button"
                    class="provider-group-expander"
                    aria-expanded={groupExpanded(group)}
                    title={groupExpanderLabel(group, groupExpanded(group))}
                    aria-label={groupExpanderLabel(group, groupExpanded(group))}
                    onclick={() => virtualModels.toggleGroupExpanded(group.key)}
                  >
                    <Icon
                      icon={groupExpanded(group) ? ChevronDown : ChevronRight}
                      class="provider-group-expander-svg"
                    />
                  </button>
                  <span class="mono font-size-md">{group.display_name}</span>
                  {#if group.type_label}
                    <span class="provider-group-type">{"(" + group.type_label + ")"}</span>
                  {/if}
                  {#if group.item_count_label}
                    <span class="provider-group-count">{group.item_count_label}</span>
                  {/if}
                </div>
                {#if group.access_summary}
                  <div class="provider-group-summary">{group.access_summary}</div>
                {/if}
              </div>
              <div class="alias-actions-cell model-list-actions">
                {#if group.access.selector}
                  <AccessToggle row={group} />
                {/if}
                {#if pricingOverridesAvailable && group.provider_name}
                  <TableActionButton
                    label={pricingOverrides.modelPricingButtonLabel(m.models_provider_pricing_for({ name: group.display_name }), pricingOverrides.hasProviderPricingOverride(group))}
                    class="table-icon-btn {pricingOverrides.modelPricingButtonClass(pricingOverrides.hasProviderPricingOverride(group))}"
                    onclick={() => pricingOverrides.openProviderPricingOverrideEdit(group)}
                  >
                    <Icon icon={CircleDollarSign} class="table-icon-svg" />
                  </TableActionButton>
                {/if}
                {#if rateLimitsEnabled && group.provider_name}
                  <TableActionButton
                    label={rateLimits.rateLimitGaugeTitle( "provider " + group.display_name, rateLimits.rateLimitGaugeClassForProvider(group), )}
                    class="table-icon-btn {rateLimits.rateLimitGaugeClassForProvider(group)}"
                    onclick={() => rateLimits.openRateLimitInspectorForProvider(group)}
                  >
                    <Icon icon={Gauge} class="table-icon-svg" />
                  </TableActionButton>
                {/if}
                {#if virtualModelsAvailable && group.access.selector}
                  <TableActionButton
                    label={modelOverrideEditButtonLabel(m.models_provider_access_for({ name: group.display_name }), hasAccessOverride(group.access))}
                    class="table-icon-btn {modelOverrideEditButtonClass(hasAccessOverride(group.access))}"
                    onclick={() => virtualModelEditor.openProviderOverrideEdit(group)}
                  >
                    <Icon icon={Pencil} class="table-icon-svg" />
                  </TableActionButton>
                {/if}
              </div>
            </div>
          </td>
        </tr>
        {#if groupExpanded(group)}
          {#each group.rows as row (row.key)}
            <ModelRow {row} {columns} />
          {/each}
        {/if}
      </tbody>
    {/each}
  </table>
</div>

<style>
  .provider-group-row :global(td) {
    background: color-mix(in srgb, var(--accent) 6%, var(--bg));
    padding-top: 12px;
    padding-bottom: 12px;
  }

  .provider-group-row:hover :global(td) {
    background: color-mix(in srgb, var(--accent) 8%, var(--bg));
  }

  .provider-group-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
  }

  .provider-group-meta {
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .provider-group-title {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
  }

  .provider-group-expander {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 22px;
    height: 22px;
    padding: 0;
    background: none;
    border: none;
    border-radius: 4px;
    color: var(--text-muted);
    cursor: pointer;
  }

  .provider-group-expander:hover {
    background: var(--bg-surface-hover);
    color: var(--text);
  }

  .provider-group-expander-svg {
    width: 14px;
    height: 14px;
  }

  .provider-group-type, .provider-group-count, .provider-group-summary {
    color: var(--text-muted);
  }

  .provider-group-type, .provider-group-count {
    font-size: 12px;
  }

  .provider-group-summary {
    font-size: 12px;
  }

  @media (max-width: 768px) {
    .provider-group-header {
        flex-direction: column;
        align-items: flex-start;
      }
  }
</style>
