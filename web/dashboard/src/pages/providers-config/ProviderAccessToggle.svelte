<script>
  // Provider-wide availability switch: icon-only (the .alias-toggle track and
  // thumb, no caption), matching the Models page's provider-group switch. One
  // flip switches every model the provider serves, through the provider-scoped
  // access policy the Models page writes from its own switches.
  import * as m from "$lib/paraglide/messages.js";
  import { providersConfig } from "./providersConfig.svelte.js";

  let { row } = $props();

  // Bridge the cross-module singleton store read through a component-level
  // $derived so the template tracks it (Svelte 5 does not establish reactive
  // deps on module-scope object properties read directly in markup).
  const access = $derived(providersConfig.providerAccessFor(row.name));
  const toggling = $derived(providersConfig.accessTogglingName === row.name);
  const available = $derived(providersConfig.virtualModelsAvailable);
  const label = $derived(
    access.effective_enabled
      ? m.providers_access_disable_all({ name: row.name })
      : m.providers_access_enable_all({ name: row.name }),
  );
</script>

<button
  type="button"
  class="alias-toggle"
  class:enabled={access.effective_enabled}
  disabled={toggling || !available}
  aria-label={label}
  title={label}
  onclick={() => providersConfig.toggleProviderAccess(row.name)}
>
  <span class="alias-toggle-track"><span class="alias-toggle-thumb"></span></span>
</button>
