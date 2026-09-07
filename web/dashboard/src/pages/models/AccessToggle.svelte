<script>
  // Enable/restrict/disable switch shared by model rows, alias rows, provider
  // groups, and the global scope.
  import { virtualModels } from "./virtualModels.svelte.js";

  let { row } = $props();

  // Bridge cross-module singleton store reads through component-level $derived
  // so the template tracks them (Svelte 5 does not establish reactive deps on
  // module-scope object properties read directly in markup).
  const rowToggleEnabled = $derived(virtualModels.rowToggleEnabled(row));
  const rowToggleRestricted = $derived(virtualModels.rowToggleRestricted(row));
  const rowToggleAriaLabel = $derived(virtualModels.rowToggleAriaLabel(row));
  const rowToggleLabel = $derived(virtualModels.rowToggleLabel(row));
  const rowTogglingKey = $derived(virtualModels.rowTogglingKey);
  const virtualModelsAvailable = $derived(virtualModels.virtualModelsAvailable);
</script>

<button
  type="button"
  class="alias-toggle"
  class:enabled={rowToggleEnabled}
  class:restricted={rowToggleRestricted}
  disabled={rowTogglingKey === row.key || !virtualModelsAvailable}
  aria-label={rowToggleAriaLabel}
  onclick={() => virtualModels.toggleRowEnabled(row)}
>
  <span class="alias-toggle-track"><span class="alias-toggle-thumb"></span></span>
  <span>{rowToggleLabel}</span>
</button>
