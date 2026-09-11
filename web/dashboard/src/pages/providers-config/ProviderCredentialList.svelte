<script>
  import * as m from "$lib/paraglide/messages.js";
  // Provider credential table. Managed rows (declared in config.yaml or env
  // vars) show a Config badge and expose no edit/delete actions — but every
  // row can re-fetch its model list, so the actions column is always shown.
  // The Enabled column is the provider-wide model availability switch.
  import TableActionButton from "$lib/components/atoms/TableActionButton.svelte";
  import Icon from "$lib/components/atoms/Icon.svelte";
  import { timezone } from "$lib/stores/timezone.svelte.js";
  import { providersConfig } from "./providersConfig.svelte.js";
  import ProviderAccessToggle from "./ProviderAccessToggle.svelte";
  import {
    providerCredentialAuthLabel,
    providerModelsCell,
  } from "./providersConfigLogic.js";
  import { Pencil, RefreshCw, X } from "lucide";

  // Bound here rather than passed as a reference: the store's formatter reads
  // `this`, so a bare reference would lose its receiver.
  const formatFetchedAt = (value) => timezone.formatTimestamp(value);
</script>

<div class="table-wrapper">
  <table class="data-table">
    <thead>
      <tr>
        <th>{m.providers_name()}</th>
        <th>{m.providers_type()}</th>
        <th>{m.overview_base_url()}</th>
        <th>{m.providers_auth()}</th>
        <th>{m.providers_models()}</th>
        <th title={m.providers_serving_column_hint()}>{m.providers_serving_column()}</th>
        <th>{m.providers_updated()}</th>
        <th class="col-actions">{m.providers_actions()}</th>
      </tr>
    </thead>
    <tbody>
      {#each providersConfig.filteredRows as row (row.name)}
        <tr>
          <td>
            <span class="font-size-md">{row.name}</span>
            {#if row.managed}
              <span
                class="alias-kind-badge"
                title={m.providers_managed()}
                >{m.common_config()}</span>
            {/if}
          </td>
          <td><span class="budget-source mono">{row.type}</span></td>
          <td class="mono font-size-md" title={row.base_url || ""}>{row.base_url || "—"}</td>
          <td>{providerCredentialAuthLabel(row)}</td>
          <td>{providerModelsCell(row, formatFetchedAt)}</td>
          <td>
            <!-- The Enabled column is an availability switch for every model
                 this provider serves (a provider-scoped access policy), not the
                 credential's own enabled flag, which the editor owns. -->
            <ProviderAccessToggle {row} />
          </td>
          <td>{timezone.formatTimestamp(row.updated_at)}</td>
          <td class="col-actions">
            <div class="alias-actions-cell model-list-actions">
              <TableActionButton
                label={providersConfig.refreshingName === row.name
                  ? m.providers_refreshing_action({ name: row.name })
                  : m.providers_refresh_action({ name: row.name })}
                class="table-icon-btn"
                onclick={() => providersConfig.refreshModels(row.name)}
                disabled={Boolean(providersConfig.refreshingName)}
              >
                <Icon icon={RefreshCw} class="table-icon-svg" />
              </TableActionButton>
              {#if !row.managed}
                <TableActionButton
                  label={m.providers_edit_action({ name: row.name })}
                  class="table-icon-btn"
                  onclick={() => providersConfig.openEdit(row)}
                >
                  <Icon icon={Pencil} class="table-icon-svg" />
                </TableActionButton>
                <TableActionButton
                  label={providersConfig.deletingName === row.name
                    ? m.providers_deleting_action({ name: row.name })
                    : m.providers_delete_action({ name: row.name })}
                  class="table-action-btn-danger table-icon-btn"
                  onclick={() => providersConfig.requestDelete(row.name)}
                  disabled={providersConfig.deletingName === row.name}
                >
                  <Icon icon={X} class="table-icon-svg" />
                </TableActionButton>
              {/if}
            </div>
          </td>
        </tr>
      {/each}
    </tbody>
  </table>
</div>
