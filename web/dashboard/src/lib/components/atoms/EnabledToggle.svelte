<script>
  // Enabled/Disabled slide toggle shared by editors (MCP server, provider
  // credential, failover rule, virtual model) and list rows. Renders the
  // .alias-toggle track/thumb pair from the global sheet.
  //
  // Props:
  //   enabled    — current state (parent flips it in onclick)
  //   label      — what is being toggled, for the aria-label ("MCP server")
  //   disabled   — read-only (config-managed rows)
  //   restricted — the "enabled but user-path-scoped" amber state
  //   onclick    — toggle handler
  //   text       — visible caption override; defaults to Enabled/Disabled
  //   ariaLabel  — full aria override for callers whose action wording differs
  //                from "Enable/Disable {label}" (e.g. Start/Stop serving)
  import * as m from "$lib/paraglide/messages.js";

  let {
    enabled = false,
    label = "",
    disabled = false,
    restricted = false,
    onclick,
    text,
    ariaLabel = "",
  } = $props();

  function onToggle(event) {
    onclick?.(event);
    // The toggle flips form state programmatically; dispatch a bubbling
    // change so enclosing forms (the editor dialog's unsaved-changes guard)
    // treat flips like native field edits.
    event.currentTarget.dispatchEvent(new Event("change", { bubbles: true }));
  }
</script>

<button
  type="button"
  class="alias-toggle"
  class:enabled
  class:restricted
  {disabled}
  aria-label={ariaLabel ||
    (enabled
      ? m.common_action_disable({ subject: label })
      : m.common_action_enable({ subject: label }))}
  onclick={onToggle}
>
  <span class="alias-toggle-track"><span class="alias-toggle-thumb"></span></span>
  <span>{text ?? (enabled ? m.common_enabled() : m.common_disabled())}</span>
</button>
