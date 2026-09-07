<script>
  // Global-scope action buttons shown in every table's actions header.
  import Icon from "$lib/components/atoms/Icon.svelte";
  import TableActionButton from "$lib/components/atoms/TableActionButton.svelte";
  import { virtualModels } from "./virtualModels.svelte.js";
  import { virtualModelEditor } from "./virtualModelEditor.svelte.js";
  import { pricingOverrides } from "./pricingOverrides.svelte.js";
  import {
  modelOverrideEditButtonClass,
  modelOverrideEditButtonLabel,
} from "./displayRows.js";
  import AccessToggle from "./AccessToggle.svelte";
  import { CircleDollarSign, Pencil } from "lucide";
  import * as m from "$lib/paraglide/messages.js";

  // Bridge cross-module singleton store reads through component-level $derived
  // so the template tracks them (Svelte 5 does not establish reactive deps on
  // module-scope object properties read directly in markup).
  const virtualModelsAvailable = $derived(virtualModels.virtualModelsAvailable);
  const pricingOverridesAvailable = $derived(
    pricingOverrides.modelPricingOverridesAvailable,
  );
  const globalScopeRow = $derived(virtualModels.globalScopeRow);
  const hasGlobalModelOverride = $derived(virtualModels.hasGlobalModelOverride());
</script>

<div class="alias-actions-cell model-list-actions">
  {#if virtualModelsAvailable}
    <AccessToggle row={globalScopeRow} />
  {/if}
  {#if pricingOverridesAvailable}
    <TableActionButton
      label={pricingOverrides.modelPricingButtonLabel(m.models_global_pricing(), pricingOverrides.hasGlobalPricingOverride())}
      class="table-icon-btn {pricingOverrides.modelPricingButtonClass(pricingOverrides.hasGlobalPricingOverride())}"
      onclick={() => pricingOverrides.openGlobalPricingOverrideEdit()}
    >
      <Icon icon={CircleDollarSign} class="table-icon-svg" />
    </TableActionButton>
  {/if}
  {#if virtualModelsAvailable}
    <TableActionButton
      label={modelOverrideEditButtonLabel(m.models_global_access(), hasGlobalModelOverride)}
      class="table-icon-btn {modelOverrideEditButtonClass(hasGlobalModelOverride)}"
      onclick={() => virtualModelEditor.openGlobalModelOverrideEdit()}
    >
      <Icon icon={Pencil} class="table-icon-svg" />
    </TableActionButton>
  {/if}
</div>
