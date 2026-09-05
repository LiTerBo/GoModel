<script>
  import * as m from "$lib/paraglide/messages.js";
  // Providers page: dashboard-managed provider credentials
  // (/admin/provider-credentials).
  import LoadingState from "$lib/components/molecules/LoadingState.svelte";
  import Icon from "$lib/components/atoms/Icon.svelte";
  import FilterInput from "$lib/components/molecules/FilterInput.svelte";
  import InlineHelpSection from "$lib/components/molecules/InlineHelpSection.svelte";
  import { router } from "$lib/stores/router.svelte.js";
  import { auth } from "$lib/stores/auth.svelte.js";
  import { providersConfig } from "./providersConfig.svelte.js";
  import ProviderCredentialList from "./ProviderCredentialList.svelte";
  import ProviderCredentialEditor from "./ProviderCredentialEditor.svelte";
  import { Plus } from "lucide";

  const PAGE = "providers-config";

  // Re-fetch when the page becomes active or the API key changes.
  $effect(() => {
    void auth.refreshTick;
    if (router.page === PAGE) providersConfig.fetchPage();
  });
</script>

<div>
  <div class="page-header">
    <div>
      <InlineHelpSection copyId="providers-config-help-copy" label={m.providers_help_label()}>
        {#snippet title()}<h2>{m.providers_title()}</h2>{/snippet}
        {#snippet help()}
          {m.providers_help()}
        {/snippet}
      </InlineHelpSection>
    </div>
    <div class="page-header-controls">
      {#if providersConfig.available && !auth.needsAuth}
        <button
          type="button"
          class="btn btn-primary btn-with-icon"
          disabled={providersConfig.formSubmitting}
          onclick={() => providersConfig.openCreate()}
        >
          <Icon icon={Plus} class="form-action-icon" />
          <span>{m.providers_add()}</span>
        </button>
      {/if}
    </div>
  </div>

  {#if !providersConfig.available && !auth.needsAuth}
    <div class="alert alert-warning">{m.providers_unavailable()}</div>
  {/if}
  {#if providersConfig.error && !auth.needsAuth && !providersConfig.formOpen}
    <p class="form-error" role="alert" aria-live="assertive">{providersConfig.error}</p>
  {/if}
  {#if providersConfig.loading && !auth.needsAuth}
    <LoadingState label={m.providers_loading()} />
  {/if}

  {#if (providersConfig.rows.length > 0 || providersConfig.filter) && providersConfig.available && !auth.needsAuth}
    <div class="table-toolbar">
      <div class="table-toolbar-main">
        <FilterInput
          id="provider-credential-filter"
          placeholder={m.providers_filter_placeholder()}
          label={m.providers_filter_label()}
          bind:value={providersConfig.filter}
        />
      </div>
    </div>
  {/if}

  <ProviderCredentialEditor />

  {#if providersConfig.filteredRows.length > 0 && providersConfig.available && !auth.needsAuth}
    <ProviderCredentialList />
  {/if}

  {#if providersConfig.rows.length === 0 && !providersConfig.filter && !providersConfig.loading && !auth.needsAuth && !providersConfig.error && providersConfig.available}
    <p class="empty-state">
      {m.providers_empty()}
    </p>
  {/if}
  {#if providersConfig.rows.length > 0 && providersConfig.filteredRows.length === 0 && providersConfig.filter && !providersConfig.loading && !auth.needsAuth && providersConfig.available}
    <p class="empty-state">{m.providers_no_match()}</p>
  {/if}
</div>
