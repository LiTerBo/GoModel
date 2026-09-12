<script>
  // Pricing override editor (EditorDialog shell).
  import TableActionButton from "$lib/components/atoms/TableActionButton.svelte";
  import EditorDialog from "$lib/components/organisms/EditorDialog.svelte";
  import FormField from "$lib/components/molecules/FormField.svelte";
  import Icon from "$lib/components/atoms/Icon.svelte";
  import { formatPriceFine } from "$lib/utils/format.js";
  import { pricingOverrides } from "./pricingOverrides.svelte.js";
  import { Plus, X } from "lucide";
  import * as m from "$lib/paraglide/messages.js";

  const po = pricingOverrides;
</script>

<EditorDialog
  open={po.modelPricingOverrideFormOpen}
  ariaLabel={m.models_pricing_editor()}
  dialogClass="model-pricing-editor"
  error={po.modelPricingOverrideError}
  submitting={po.modelPricingOverrideSubmitting}
  submitLabel={m.models_pricing_save()}
  onclose={() => po.closeModelPricingOverrideForm()}
  onsubmit={() => po.submitModelPricingOverrideForm()}
>
  {#snippet header()}
    <p class="form-kicker">{m.models_pricing_override()}</p>
    <h3>{po.modelPricingOverrideFormDisplayName || po.modelPricingOverrideForm.selector || m.models_pricing()}</h3>
  {/snippet}

  <div class="form-grid">
    <FormField id="model-pricing-override-selector" label={m.models_selector()}>
      <input
        id="model-pricing-override-selector"
        type="text"
        class="mono"
        bind:value={po.modelPricingOverrideForm.selector}
        disabled
      />
    </FormField>

    {#if po.modelPricingOverrideFormScopeOptions.length > 1}
      <FormField id="model-pricing-override-scope" label={m.models_scope()}>
        <select
          id="model-pricing-override-scope"
          class="form-select"
          bind:value={po.modelPricingOverrideFormScope}
          onchange={() => po.setModelPricingOverrideScope(po.modelPricingOverrideFormScope)}
        >
          {#each po.modelPricingOverrideFormScopeOptions as option (option.value)}
            <option value={option.value}>{option.label}</option>
          {/each}
        </select>
      </FormField>
    {/if}
  </div>

  <p class="form-hint">
    {m.models_pricing_help()}
  </p>

  <div class="pricing-override-rows">
    {#each po.modelPricingOverrideRows as row (row.id)}
      <div class="pricing-override-row">
        <div class="form-field pricing-override-type-field">
          <label class="form-field-label" for={"pricing-type-" + row.id}>{m.models_price_type()}</label>
          <select
            id={"pricing-type-" + row.id}
            class="form-select"
            bind:value={row.field}
            data-modal-autofocus
          >
            {#each po.availablePricingFieldOptions(row) as option (option.value)}
              <option value={option.value}>{option.group + " - " + option.label}</option>
            {/each}
          </select>
        </div>
        <div class="form-field pricing-override-value-field">
          <label class="form-field-label" for={"pricing-value-" + row.id}>{m.models_usd_value()}</label>
          <input
            id={"pricing-value-" + row.id}
            type="number"
            step="any"
            min="0"
            inputmode="decimal"
            bind:value={row.value}
          />
        </div>
        <TableActionButton
          label={m.models_remove_price({ price: po.pricingFieldLabel(row.field) })}
          class="table-action-btn-danger table-icon-btn pricing-override-remove-row"
          onclick={() => po.removeModelPricingOverrideRow(row)}
        >
          <Icon icon={X} class="table-icon-svg" />
        </TableActionButton>
      </div>
    {/each}
  </div>

  <div class="pricing-override-row-actions">
    <button
      type="button"
      class="btn btn-with-icon"
      onclick={() => po.addModelPricingOverrideRow()}
    >
      <Icon icon={Plus} class="form-action-icon" />
      <span>{m.models_add_price()}</span>
    </button>
  </div>

  {#if po.modelPricingOverrideFormTimeWindows.length > 0}
    <div class="time-windows-section">
      <div class="time-windows-header">
        <h4>{m.models_tw_section()}</h4>
        <button type="button" class="btn btn-with-icon" onclick={() => po.addTimeWindow()}>
          <Icon icon={Plus} class="form-action-icon" />
          <span>{m.models_tw_add_window()}</span>
        </button>
      </div>
      {#each po.modelPricingOverrideFormTimeWindows as window, wi}
        <div class="time-window-card">
          <div class="time-window-header-row">
            <FormField label={m.models_tw_label_field()}>
              <input type="text" class="mono" placeholder="off_peak" bind:value={window.label} />
            </FormField>
            <button type="button" class="table-action-btn-danger table-icon-btn" aria-label={m.models_tw_remove_window()} onclick={() => po.removeTimeWindow(wi)}>
              <Icon icon={X} class="table-icon-svg" />
            </button>
          </div>

          <div class="time-window-ranges">
            <div class="tw-range-header">
              <span>{m.models_tw_ranges()}</span>
              <button type="button" class="btn btn-with-icon" onclick={() => po.addTimeRange(window)}>
                <Icon icon={Plus} class="form-action-icon" />
                <span>{m.models_tw_add_range()}</span>
              </button>
            </div>
            {#each window.utc_ranges as range, ri}
              <div class="tw-range-row">
                <div class="tw-day-picker">
                  {#each ["mon", "tue", "wed", "thu", "fri", "sat", "sun"] as day}
                    <label class="tw-day-label">
                      <input type="checkbox" checked={range.days.includes(day)} onchange={() => po.toggleTimeRangeDay(range, day)} />
                      <span>{day.slice(0, 2)}</span>
                    </label>
                  {/each}
                </div>
                <input type="text" class="mono tw-time-input" aria-label={m.models_tw_start()} placeholder="00:00" maxlength="5" bind:value={range.start} />
                <input type="text" class="mono tw-time-input" aria-label={m.models_tw_end()} placeholder="24:00" maxlength="5" bind:value={range.end} />
                <button type="button" class="table-action-btn-danger table-icon-btn" aria-label={m.models_tw_remove_window()} onclick={() => po.removeTimeRange(window, ri)}>
                  <Icon icon={X} class="table-icon-svg" />
                </button>
              </div>
            {/each}
          </div>

          <div class="time-window-rates">
            <span class="tw-rates-label">{m.models_tw_window_rates()}</span>
            <div class="tw-rate-fields">
              <FormField label={m.models_price_input()}>
                <input type="number" step="any" min="0" class="mono" placeholder="0" bind:value={window.pricing.input_per_mtok} />
              </FormField>
              <FormField label={m.models_price_output()}>
                <input type="number" step="any" min="0" class="mono" placeholder="0" bind:value={window.pricing.output_per_mtok} />
              </FormField>
              <FormField label={m.models_price_cached_input()}>
                <input type="number" step="any" min="0" class="mono" placeholder="0" bind:value={window.pricing.cached_input_per_mtok} />
              </FormField>
            </div>
          </div>
        </div>
      {/each}
    </div>
  {:else}
    <div class="pricing-override-row-actions">
      <button type="button" class="btn btn-with-icon" onclick={() => po.addTimeWindow()}>
        <Icon icon={Plus} class="form-action-icon" />
        <span>{m.models_tw_add_window()}</span>
      </button>
    </div>
  {/if}

  {#if po.modelPricingOverrideFormPreservedTiers.length > 0}
    <div class="pricing-override-tier-note">
      {m.models_tiered_pricing_help()}
    </div>
  {/if}

  <div class="pricing-preview">
    <div class="pricing-preview-header">
      <span>{m.models_price_type()}</span>
      <span>USD</span>
      <span>{m.models_source_column()}</span>
    </div>
    {#if po.modelPricingEffectivePreviewRows().length === 0}
      <div class="pricing-preview-row pricing-preview-row-empty">{m.models_no_pricing()}</div>
    {/if}
    {#each po.modelPricingEffectivePreviewRows() as row (row.field)}
      <div class="pricing-preview-row">
        <span>{row.label}</span>
        <span class="mono">
          {row.value === null || row.value === undefined ? "-" : formatPriceFine(Number(row.value))}
        </span>
        <span>{row.source}</span>
      </div>
    {/each}
  </div>

  {#snippet extraActions()}
    {#if po.modelPricingOverrideFormHasExistingOverride}
      <button
        type="button"
        class="btn btn-danger-outline"
        disabled={po.modelPricingOverrideSubmitting}
        onclick={() => po.deleteModelPricingOverride()}
      >
        {m.models_remove_override()}
      </button>
    {/if}
  {/snippet}
</EditorDialog>

<style>
.pricing-override-rows {
    display: grid;
    gap: 12px;
  }

.pricing-override-row {
    display: grid;
    grid-template-columns: minmax(220px, 1fr) minmax(130px, 180px) 32px;
    gap: 12px;
    align-items: end;
  }

.pricing-override-row-actions {
    display: flex;
    justify-content: flex-start;
  }

.pricing-override-tier-note {
    padding: 10px 12px;
    border: 1px solid var(--border);
    border-radius: 6px;
    background: var(--bg);
    color: var(--text-muted);
    font-size: 13px;
  }

.pricing-preview {
    border: 1px solid var(--border);
    border-radius: 6px;
    overflow: hidden;
  }

.pricing-preview-header, .pricing-preview-row {
    display: grid;
    grid-template-columns: minmax(150px, 1fr) minmax(90px, auto) minmax(130px, 0.8fr);
    gap: 12px;
    align-items: center;
    padding: 10px 12px;
  }

.pricing-preview-header {
    background: var(--bg);
    color: var(--text-muted);
    font-size: 12px;
    font-weight: 600;
    text-transform: uppercase;
  }

.pricing-preview-row {
    border-top: 1px solid var(--border);
    font-size: 13px;
  }

.pricing-preview-row-empty {
    grid-template-columns: 1fr;
    color: var(--text-muted);
  }

.time-windows-section {
    display: grid;
    gap: 16px;
    margin-top: 16px;
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 12px;
    background: var(--bg);
  }
.time-windows-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
.time-windows-header h4 {
    margin: 0;
    font-size: 14px;
    font-weight: 600;
    color: var(--foreground);
  }
.time-window-card {
    display: grid;
    gap: 12px;
    padding: 12px;
    border: 1px solid var(--border);
    border-radius: 6px;
    background: var(--card);
  }
.time-window-header-row {
    display: grid;
    grid-template-columns: 1fr auto;
    gap: 8px;
    align-items: end;
  }
.time-window-ranges {
    display: grid;
    gap: 8px;
  }
.tw-range-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 12px;
    font-weight: 600;
    color: var(--muted-foreground);
    text-transform: uppercase;
  }
.tw-range-row {
    display: grid;
    grid-template-columns: auto 72px 72px 32px;
    gap: 8px;
    align-items: center;
  }
.tw-day-picker {
    display: flex;
    gap: 4px;
  }
.tw-day-label {
    display: flex;
    align-items: center;
    gap: 2px;
    font-size: 11px;
    cursor: pointer;
  }
.tw-day-label input {
    margin: 0;
  }
.tw-time-input {
    width: 100%;
    text-align: center;
    font-size: 12px;
    padding: 4px 6px;
  }
.time-window-rates {
    display: grid;
    gap: 6px;
  }
.tw-rates-label {
    font-size: 12px;
    font-weight: 600;
    color: var(--muted-foreground);
    text-transform: uppercase;
  }
.tw-rate-fields {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 8px;
  }

@media (max-width: 768px) {
  .pricing-override-row, .pricing-preview-header, .pricing-preview-row {
          grid-template-columns: 1fr;
        }
}
</style>
