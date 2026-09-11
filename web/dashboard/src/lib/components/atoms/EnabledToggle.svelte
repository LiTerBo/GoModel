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
  {onclick}
>
  <span class="alias-toggle-track"><span class="alias-toggle-thumb"></span></span>
  <span>{text ?? (enabled ? m.common_enabled() : m.common_disabled())}</span>
</button>
