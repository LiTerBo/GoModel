<script>
  // Enable/restrict/disable switch shared by model rows, alias rows, provider
  // groups, and the global scope. Icon-only (phase F): the state text is
  // dropped — the track/thumb pair plus the aria-label/title carry the
  // semantics, matching the compact icon-action convention of the actions
  // column.
  import { virtualModels } from "./virtualModels.svelte.js";

  let { row } = $props();

  // Bridge cross-module singleton store reads through component-level $derived
  // so the template tracks them (Svelte 5 does not establish reactive deps on
  // module-scope object properties read directly in markup).
  const rowToggleEnabled = $derived(virtualModels.rowToggleEnabled(row));
  const rowToggleRestricted = $derived(virtualModels.rowToggleRestricted(row));
  const rowToggleAriaLabel = $derived(virtualModels.rowToggleAriaLabel(row));
  const rowTogglingKey = $derived(virtualModels.rowTogglingKey);
  const virtualModelsAvailable = $derived(virtualModels.virtualModelsAvailable);
  // The provider of this row may itself be paused: then this switch has nothing
  // to change, so it renders disabled and its aria/title explains why.
  const rowBlocked = $derived(virtualModels.rowToggleBlocked(row));
</script>

<button
  type="button"
  class="alias-toggle"
  class:enabled={rowToggleEnabled}
  class:restricted={rowToggleRestricted}
  disabled={rowTogglingKey === row.key || !virtualModelsAvailable || rowBlocked}
  aria-label={rowToggleAriaLabel}
  title={rowToggleAriaLabel}
  onclick={() => virtualModels.toggleRowEnabled(row)}
>
  <span class="alias-toggle-track"><span class="alias-toggle-thumb"></span></span>
</button>
