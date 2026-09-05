---
version: alpha
name: gomodel-dashboard-design-blue
description: "Design language of the GoModel Dashboard — an AI gateway management panel. Cool-toned, dark-first UI: pure black canvas #000000 with layered dark-gray surfaces, a single blue accent #0066cc for primary actions, and a four-stop semantic status palette (green/blue/orange/red) mapped to provider/rate-limit states. Theme parity via data-theme + CSS custom properties with a 3-state toggle (light/system/dark). System font stacks at restrained 400-700 weights. Signature components: the collapsible resizable sidebar (240↔60 px), the live token-throughput chart panel, and the provider-configuration editor."

colors:
  # canvas & surfaces (cool, dark-first)
  bg: "#000000"
  surface: "#272729"
  surface-hover: "#2a2a2c"
  border: "#333333"
  text: "#ffffff"
  text-muted: "#7a7a7a"
  # brand accent (blue)
  accent: "#0066cc"
  accent-hover: "#0071e3"
  # semantic status (shared across themes)
  green: "#34d399"
  red: "#ef4444"
  orange: "#f59e0b"
  blue: "#2997ff"
  info: "#2997ff"
  # light theme overrides
  light-bg: "#f5f5f7"
  light-surface: "#ffffff"
  light-surface-hover: "#fafafc"
  light-border: "#e0e0e0"
  light-text: "#1d1d1f"
  light-text-muted: "#7a7a7a"
  light-accent: "#0066cc"
  light-accent-hover: "#0071e3"
  light-info: "#0066cc"
  light-warning: "#d97706"
  light-danger: "#dc2626"
  # token throughput colours (AI gateway specific, blue-toned)
  token-input: "#4a7fc7"
  token-output: "#7aadf0"
  token-prompt: "color-mix(in srgb, var(--info) 60%, transparent)"
  token-local: "color-mix(in srgb, var(--info) 20%, transparent)"
  # cache meter aliases (reuse token colours)
  cache-meter-uncached: "var(--token-input)"
  cache-meter-local: "var(--token-local)"
  cache-meter-prompt: "var(--token-prompt)"
  # calendar heatmap ramp (11 levels, blue-derived)
  cal-level-0-dark: "#161b22"
  cal-level-0-light: "#ebedf0"
  # alias row backgrounds
  alias-row-valid-bg: "color-mix(in srgb, var(--bg-surface-hover) 86%, #fff 14%)"
  alias-row-valid-bg-hover: "color-mix(in srgb, var(--bg-surface-hover) 72%, #fff 28%)"

typography:
  font-stack:
    fontFamily: "\"Inter\", -apple-system, BlinkMacSystemFont, \"Segoe UI\", Roboto, sans-serif"
  text-xs:
    fontFamily: "inherit"
    fontSize: 10px
    fontWeight: 600
    lineHeight: 1.4
  text-sm:
    fontFamily: "inherit"
    fontSize: 12px
    fontWeight: 400
    lineHeight: 1.5
  text-sm-13:
    fontFamily: "inherit"
    fontSize: 13px
    fontWeight: 400
    lineHeight: 1.5
  text-base:
    fontFamily: "inherit"
    fontSize: 14px
    fontWeight: 400
    lineHeight: 1.5
  text-base-500:
    fontFamily: "inherit"
    fontSize: 14px
    fontWeight: 500
    lineHeight: 1.5
  text-lg:
    fontFamily: "inherit"
    fontSize: 18px
    fontWeight: 700
    lineHeight: 1.3
  text-xl:
    fontFamily: "inherit"
    fontSize: 22px
    fontWeight: 700
    lineHeight: 1.2
  text-xxl:
    fontFamily: "inherit"
    fontSize: 28px
    fontWeight: 700
    lineHeight: 1.1
  caption:
    fontFamily: "inherit"
    fontSize: 12px
    fontWeight: 600
    lineHeight: 1.4
  button:
    fontFamily: "inherit"
    fontSize: 13px
    fontWeight: 400
    lineHeight: 1.4
  button-strong:
    fontFamily: "inherit"
    fontSize: 13px
    fontWeight: 600
    lineHeight: 1.4
  uppercase-label:
    fontFamily: "inherit"
    fontSize: 12px
    fontWeight: 600
    lineHeight: 1.4
    letterSpacing: 0.5px
    textTransform: uppercase

rounded:
  sm: 6px
  md: 8px
  lg: 10px
  full: 9999px

spacing:
  xxs: 2px
  xs: 4px
  sm: 8px
  md: 12px
  lg: 16px
  xl: 20px
  2xl: 24px
  3xl: 32px

layout:
  sidebar-w: 240px
  sidebar-collapsed: 60px
  sidebar-header-h: 70px
  nav-item-h: 40px
  nav-item-padding: "8px 12px"
  nav-gap: 4px
  footer-padding: 16px
  icon-md: 18px
  icon-sm: 14px
  toolbar-icon: 16px
  bp-mobile: 768px
  bp-phone: 520px
  content-max-w: 1336px
  transition-fast: 150ms
  transition-normal: 200ms
  transition-slow: 250ms

elevation:
  level-0: "none"
  level-1: "1px solid {colors.border}"
  level-2: "0 2px 8px rgba(0,0,0,0.18)"
  level-3: "0 10px 22px rgba(0,102,204,0.18)"
  level-4: "0 10px 22px color-mix(in srgb, var(--danger) 20%, transparent)"

space-tokens:
  page: 32px
  block: 24px
  card: 20px
  cell: 16px
  stack: 24px
  grid: 16px
  page-mobile: 20px
  page-phone: 12px
  block-phone: 14px
  card-phone: 14px
  cell-phone: 10px
  stack-phone: 16px
  grid-phone: 10px

components:
  sidebar:
    width: "{layout.sidebar-w}"
    collapsedWidth: "{layout.sidebar-collapsed}"
    backgroundColor: "{colors.surface}"
    borderColor: "{colors.border}"
    navItemHeight: "{layout.nav-item-h}"
    navItemPadding: "{layout.nav-item-padding}"
    navItemRadius: "{rounded.md}"
    navGap: "{layout.nav-gap}"
    navItemFont: "{typography.text-base-500}"
    activeBackground: "{colors.accent}"
    activeTextColor: "#ffffff"
    footerPadding: "{layout.footer-padding}"
    transition: "{layout.transition-normal}"
    resizeHandle: "7px wide, stickied to the right edge"
  sidebar-header:
    padding: "20px"
    borderColor: "{colors.border}"
    logoSize: "28px"
    logoColor: "{colors.accent}"
    typography: "{typography.text-lg}"
  btn:
    padding: "6px 16px"
    backgroundColor: "{colors.bg}"
    textColor: "{colors.text}"
    borderColor: "{colors.border}"
    typography: "{typography.button}"
    rounded: "{rounded.sm}"
  btn-primary:
    backgroundColor: "{colors.accent}"
    textColor: "#ffffff"
    typography: "{typography.button-strong}"
    rounded: "{rounded.sm}"
    padding: "6px 16px"
    shadow: "{elevation.level-3}"
  btn-danger:
    backgroundColor: "{colors.red}"
    textColor: "#ffffff"
    typography: "{typography.button-strong}"
    rounded: "{rounded.sm}"
    padding: "6px 16px"
    shadow: "{elevation.level-4}"
  btn-danger-outline:
    color: "{colors.red}"
    borderColor: "color-mix(in srgb, {colors.red} 55%, var(--border))"
    backgroundColor: "transparent"
    typography: "{typography.button-strong}"
  btn-with-icon:
    gap: "8px"
    iconSize: "16px"
  card:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.text}"
    borderColor: "{colors.border}"
    typography: "{typography.text-sm}"
    rounded: "{rounded.md}"
    padding: "var(--space-card)"
  card-label:
    typography: "{typography.uppercase-label}"
    color: "{colors.text-muted}"
    marginBottom: "8px"
  card-value:
    fontSize: "28px"
    fontWeight: 700
    letterSpacing: "-0.5px"
  modal:
    backgroundColor: "{colors.surface}"
    borderColor: "{colors.border}"
    rounded: "{rounded.md}"
    width: "560px"
    maxHeight: "80vh"
  input:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.text}"
    borderColor: "{colors.border}"
    typography: "{typography.text-sm-13}"
    rounded: "{rounded.md}"
    padding: "8px 10px"
    focusBorder: "{colors.accent}"
    minHeight: "38px"
  badge:
    typography: "{typography.text-xs}"
    rounded: "{rounded.lg}"
    padding: "2px 8px"
    textTransform: "uppercase"
    letterSpacing: "0.5px"
    backgroundColor: "{colors.accent}"
    textColor: "#ffffff"
  provider-badge:
    typography: "{typography.text-sm}"
    fontWeight: 500
    rounded: "12px"
    padding: "2px 10px"
    backgroundColor: "{colors.bg}"
    borderColor: "{colors.border}"
  status-badge:
    active: "color-mix(in srgb, {colors.green} 12%, var(--bg)) bg, {colors.green} text"
    inactive: "color-mix(in srgb, {colors.red} 10%, var(--bg)) bg, {colors.red} text"
    rounded: "{rounded.full}"
    typography: "{typography.text-sm}"
    fontWeight: 600
    padding: "2px 10px"
  table:
    wrapperBackground: "{colors.surface}"
    wrapperBorder: "1px solid {colors.border}"
    wrapperRadius: "{rounded.md}"
    headerBackground: "{colors.bg}"
    headerFont: "{typography.uppercase-label}"
    headerPadding: "12px var(--space-cell)"
    cellPadding: "10px var(--space-cell)"
    cellBorder: "1px solid {colors.border}"
    cellFont: "{typography.text-base}"
    hoverBackground: "{colors.surface-hover}"
    compactFont: "13px"
  theme-toggle:
    pillBackground: "{colors.bg}"
    pillBorder: "1px solid {colors.border}"
    pillRadius: "6px"
    pillPadding: "2px"
    buttonWidth: "28px"
    buttonHeight: "24px"
    buttonRadius: "4px"
    activeBackground: "{colors.accent}"
    activeTextColor: "#ffffff"
    mobileWidth: "36px"
    mobileHeight: "36px"
    iconSize: "14px"
    mobileIconSize: "16px"
  loading-spinner:
    width: "16px"
    height: "16px"
    border: "2px solid {colors.border}"
    borderTop: "2px solid {colors.accent}"
    borderWidth: "2px"
    animation: "loading-spin 0.8s linear infinite"
  chart-container:
    backgroundColor: "{colors.surface}"
    border: "1px solid {colors.border}"
    rounded: "{rounded.md}"
    padding: "var(--space-block)"
  chart-grid: "{colors.border}"
  chart-text: "{colors.text-muted}"
  chart-day-marker: "{colors.text}"
  chart-tooltip-bg: "{colors.surface}"
  chart-tooltip-border: "{colors.border}"
  chart-tooltip-text: "{colors.text}"
  workflow-node:
    padding: "8px 12px"
    rounded: "{rounded.md}"
    border: "1px solid {colors.border}"
    backgroundColor: "{colors.surface}"
    minWidth: "72px"
  alias-toggle:
    padding: "6px 10px"
    rounded: "{rounded.sm}"
    border: "1px solid {colors.border}"
    backgroundColor: "{colors.bg}"
    trackWidth: "34px"
    trackHeight: "18px"
    trackRadius: "6px"
    thumbSize: "16px"
    thumbRadius: "6px"
    enabledBorder: "color-mix(in srgb, {colors.green} 50%, var(--border))"
---
## Overview

GoModel Dashboard is a **B/S web application** for managing an AI gateway — monitoring token throughput, configuring providers, managing models, setting budgets/rate limits, and inspecting audit logs. This document defines the design language of the dashboard UI.

The surface reads as a professional operations tool: **cool-toned cards on a pure black canvas** (`{colors.bg}` `#000000`) for the default dark theme, separated by `{colors.border}` `#333333` hairlines. Around this neutral-cool base, the brand layers a **single blue accent** `{colors.accent}` `#0066cc` reserved for primary actions, active states, links, and progress fills; a four-stop semantic status palette maps to gateway states: green `{colors.green}` (healthy/success), blue `{colors.blue}` (processing/active), orange `{colors.orange}` (warning/rate-limited), red `{colors.red}` (error/down).

Type uses `Inter` with system fallbacks (`-apple-system` / `BlinkMacSystemFont` / `Segoe UI` / `Roboto`) at restrained weights — 400 for body, 500 for navigation items, 600 for buttons and labels, 700 for page headers and statistics values. The UI guarantees **WCAG 2.1 AA contrast (≥ 4.5:1)** — white text `{colors.text}` `#ffffff` on dark surfaces for dark theme, dark text `{colors.light-text}` `#1d1d1f` on white cards for light theme.

The shape system is unified: `{rounded.md}` `8px` is the single radius used for cards, inputs, modals, and nav items; `{rounded.sm}` `6px` is reserved for buttons; `{rounded.full}` for circular elements. The `{rounded.lg}` `10px` variant is used for badges. Elevation is minimal — `{elevation.level-1}` hairline borders are the default chrome, with `{elevation.level-3}` blue-tinted shadow for the primary CTA button.

**Key Characteristics:**
- A **cool-toned, dark-first canvas** — pure black surfaces with dark-gray borders; a single blue accent for every primary action.
- The brand's signature is the **collapsible, resizable sidebar** (240↔60 px) with a three-zone hierarchy (header / primary nav / footer for theme toggle + API key).
- **Live token-throughput charting**: Chart.js driven by a `tick` versioning system that rebuilds the canvas when the theme changes, reading colours from CSS custom properties.
- **Three-state theme toggle** (light / system / dark) with localStorage persistence, placed in the sidebar footer.
- The Spacing system is responsive: spacing tokens scale across three breakpoints (desktop 32px → tablet 20px → phone 12px) so components carry no spacing media queries.
- **B/S responsive**: sidebar collapses to icon mode, auto-breakpoints at 768px (tablet) and 520px (phone).

## Colors

### Brand & Accent
- **Accent Blue** (`{colors.accent}` — `#0066cc`): The brand's single conversion colour — primary CTAs, active states, links, focus rings, progress fills, logo colour.
- **Accent Hover** (`{colors.accent-hover}` — `#0071e3`): Hover state for primary buttons and active nav items.

### Surface (Dark Theme — Default)
- **Canvas** (`{colors.bg}` — `#000000`): The default page background (dark, pure black).
- **Surface** (`{colors.surface}` — `#272729`): Cards, panels, sidebar, modals, table wrappers, chart containers.
- **Surface Hover** (`{colors.surface-hover}` — `#2a2a2c`): Table row hover, nav item hover, button hover, dropdown items.
- **Border** (`{colors.border}` — `#333333`): 1 px solid borders — card chrome, input borders, dividers, table row separators.

### Surface (Light Theme — Explicit & System)
- **Canvas** (`{colors.light-bg}` — `#f5f5f7`): The light page background, a cool gray.
- **Surface** (`{colors.light-surface}` — `#ffffff`): White cards, panels, modals.
- **Surface Hover** (`{colors.light-surface-hover}` — `#fafafc`): Light hover state.
- **Border** (`{colors.light-border}` — `#e0e0e0`): Light hairline borders.

### Text
- **Text** (`{colors.text}` — `#ffffff`): Default text and headings (dark theme). Light theme: `{colors.light-text}` `#1d1d1f`.
- **Text Muted** (`{colors.text-muted}` — `#7a7a7a`): Secondary text — meta, captions, placeholders, disabled, table headers. Light theme: `{colors.light-text-muted}` `#7a7a7a`.

### Semantic Status
- **Success Green** (`{colors.green}` — `#34d399`): Healthy states, active models, successful operations, active auth-key status badges.
- **Info Blue** (`{colors.blue}` — `#2997ff`): Processing states, prompting cache indicator, calendar heatmap base. Light theme: `{colors.light-info}` `#0066cc`.
- **Warning Orange** (`{colors.orange}` — `#f59e0b`): Rate-limit approaching, warning states, notification dot. Light theme: `{colors.light-warning}` `#d97706`.
- **Error Red** (`{colors.red}` — `#ef4444`): Error states, exceeded limits, deactivated keys, form errors. Light theme: `{colors.light-danger}` `#dc2626`.

### Categorical Palette (Label Chips)

A deterministic 11-colour palette for usage labels, auth-key labels, and Chart.js series. Each label is assigned a colour via `labelColor()` (djb2 hash), so the same label keeps one colour across the dashboard. Colours are applied as `--label-color` and mixed with the background at 14 % for chip tinting.

| # | Colour | Value | Use |
|---|--------|-------|-----|
| 1 | Token-input blue | `#4a7fc7` | Paid input tokens, period weekly |
| 2 | Token-output blue | `#7aadf0` | Paid output tokens |
| 3 | Teal | `#2aa8a0` | Period hourly bars |
| 4 | Muted purple | `#8a6fc4` | Period custom bars |
| 5 | Classic blue | `#3b82f6` | General label |
| 6 | Desaturated teal | `#5a9f9e` | General label |
| 7 | Steel blue | `#6b8eae` | General label |
| 8 | Muted lavender | `#9b7ea4` | General label |
| 9 | Medium blue | `#4e8dc9` | General label |
| 10 | Sky blue | `#6699cc` | General label |
| 11 | Periwinkle | `#7a8ae0` | General label |

### Budget Period Colours

Budget progress bars and period labels use a 5-colour cool palette, one per period type. Each colour is applied three ways: fill (solid, under white text), track (22 % mix with `--bg`), and label (12 % mix background + 62 % mix border + 34 % mix text).

| Period | Colour | Fill | Use |
|--------|--------|------|-----|
| Monthly | Slate blue | `#3d5170` | Conservative, long-range budget |
| Weekly | Token-input blue | `#4a7fc7` | Matching the paid throughput token |
| Daily | Strong blue | `#2370c2` | Most frequent period, prominent |
| Hourly | Teal | `#2a8b9a` | Distinct hue, short-range |
| Custom | Muted purple | `#6b5ba8` | Dashed border, distinct hue |

### Rate-Limit Scope Colours

Rate-limit scopes use a 2-colour cool palette to distinguish tab and group headers:

| Scope | Colour | Value |
|-------|--------|-------|
| Provider | Info blue | `{colors.info}` / `#2997ff` |
| Model | Token-input blue | `#4a7fc7` |

### Token Throughput (AI Gateway Specific)
Token throughput colours are designed to communicate cost at a glance:

| Token | Colour | Value | Semantics |
|-------|--------|-------|-----------|
| Input | `{colors.token-input}` | `#4a7fc7` | Paid input tokens — solid darker blue |
| Output | `{colors.token-output}` | `#7aadf0` | Paid output tokens — lighter blue |
| Prompt Cache | `{colors.token-prompt}` | blue, 60% opacity | Cheap (prompt cache hit) — translucent blue |
| Local Cache | `{colors.token-local}` | blue, 20% opacity | Free (local cache hit) — most transparent |

The cache meter reuses these colours: `--cache-meter-uncached` = `--token-input` (paid), `--cache-meter-local` = `--token-local` (free), `--cache-meter-prompt` = `--token-prompt` (almost free).

### Calendar Heatmap
11-level ramp (`--cal-level-0` to `--cal-level-10`) derived from the info blue:
- **Dark theme**: climbs from `#161b22` (dark navy, level 0) through `--info` `#2997ff` (level 8) to a bright tint (level 10).
- **Light theme**: climbs from `#ebedf0` (pale gray, level 0) through `--info` `#0066cc` (level 7) to a deep navy (level 10).

## Typography

### Font Family
**`'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif`** — system-first stack with Inter as the primary face. Weights 400 / 500 / 600 / 700 are present; body never heavier than 400, navigation at 500, buttons and labels at 600, page headers and statistics values at 700.

### Hierarchy

| Token | Size | Weight | Line Height | Use |
|-------|------|--------|-------------|-----|
| `{typography.text-xs}` | 10px | 600 | 1.4 | Badge labels, version numbers. |
| `{typography.text-sm}` | 12px | 400 | 1.5 | Table headers, secondary text, sidebar footer controls. |
| `{typography.text-sm-13}` | 13px | 400 | 1.5 | Body text, form inputs, general buttons. |
| `{typography.text-base}` | 14px | 400 | 1.5 | Table body cells, sections, paragraphs. |
| `{typography.text-base-500}` | 14px | 500 | 1.5 | Sidebar navigation items. |
| `{typography.text-lg}` | 18px | 700 | 1.3 | Sidebar header (app name). |
| `{typography.text-xl}` | 22px | 700 | 1.2 | Page headers. |
| `{typography.text-xxl}` | 28px | 700 | 1.1 | Card statistics values. |
| `{typography.caption}` | 12px | 600 | 1.4 | Small labels, tooltips. |
| `{typography.button}` | 13px | 400 | 1.4 | Base button labels. |
| `{typography.button-strong}` | 13px | 600 | 1.4 | Primary and danger button labels. |
| `{typography.uppercase-label}` | 12px | 600 | 1.4 | Table headers, card labels (uppercase, 0.5px letter-spacing). |

### Principles
- **Weight ceiling at 700**, body at 400 — restrained, operations-tool tone.
- **Inter as primary face** with system fallbacks; no proprietary faces.
- **Uppercase labels** (table headers, card labels) use 12px weight 600 with 0.5px letter-spacing — never full caps for body text.
- **Monospace** reserved for model names, pricing, and code: `'SF Mono', Menlo, Consolas, monospace`.

## Layout

### Spacing System
The spacing scale is defined as CSS custom properties in `base.css` and is **responsive by design** — components read `var(--space-*)` tokens, and the values change at breakpoints:

| Token | Desktop | Tablet (≤768px) | Phone (≤520px) |
|-------|---------|-----------------|----------------|
| `--space-page` | 32px | 20px | 12px |
| `--space-block` | 24px | 18px | 14px |
| `--space-card` | 20px | 20px | 14px |
| `--space-cell` | 16px | 16px | 10px |
| `--space-stack` | 24px | 20px | 16px |
| `--space-grid` | 16px | 16px | 10px |

### Layout Tokens

| Token | Value | Use |
|-------|-------|-----|
| `{layout.sidebar-w}` | 240 px | Sidebar expanded width |
| `{layout.sidebar-collapsed}` | 60 px | Sidebar collapsed (icon-only) |
| `{layout.nav-item-h}` | ~40 px | Nav item (8px top + 14px font + 8px bottom + 10px gap) |
| `{layout.icon-md}` | 18 px | Nav icons |
| `{layout.icon-sm}` | 14 px | Table row action icons, auxiliary icons |
| `{layout.toolbar-icon}` | 16 px | Toolbar button icons |
| `{layout.content-max-w}` | 1336 px | Content area max-width (centred) |

### Responsive Strategy

| Name | Width | Key Changes |
|------|-------|-------------|
| Desktop | > 768 px | Fixed 240 px sidebar (collapsible to 60 px) |
| Tablet | ≤ 768 px | Sidebar 60 px auto-collapsed, page spacing 20 px |
| Phone | ≤ 520 px | Sidebar 60 px, page spacing 12 px, cards narrow |

#### Touch Targets
- Sidebar nav items at ~40 px height (with 8px top/bottom padding × 14px font).
- Buttons ≥ 32 px height, icon-only buttons ≥ 32 px.
- Inputs at minimum 38 px height.

#### Collapsing Strategy
- Sidebar: 240 px at desktop, 60 px icon-only when collapsed (chevron toggle, localStorage persistence), auto-collapse not enforced (user-controlled via resize handle).
- The sidebar has a draggable resize handle (separator role, keyboard accessible with ArrowLeft/ArrowRight).
- Collapsed state hides nav labels, shows only icons; footer shrinks to centered theme-toggle button.

## Elevation & Depth

| Level | Treatment | Use |
|-------|-----------|-----|
| Level 0 — Flat | No shadow, no border. | Page backgrounds, full-bleed areas. |
| Level 1 — Hairline | 1 px solid `{colors.border}`. | Default card chrome, inputs, tables, sidebar. |
| Level 2 — Soft Drop | `{elevation.level-2}` — `0 2px 8px rgba(0,0,0,0.18)`. | Loading state pills, floating elements. |
| Level 3 — Blue Accent Lift | `{elevation.level-3}` — `0 10px 22px rgba(0,102,204,0.18)`. | Primary CTA button (`btn-primary`). |
| Level 4 — Danger Lift | `{elevation.level-4}` — `0 10px 22px color-mix(in srgb, red, 20%, transparent)`. | Danger button (`btn-danger`). |

### Decorative Depth
Depth comes from the layered cool surfaces and the blue accent — no gradients, no photography, no background patterns. The sole decorative flourish is the `color-mix()` accent tint on the primary CTA shadow.

## Shapes

### Border Radius Scale

The system uses a deliberately narrow radius palette:

| Token | Value | Use |
|-------|-------|-----|
| `{rounded.sm}` | 6 px | Buttons, toggles, table action buttons, icon-only buttons. |
| `{rounded.md}` | 8 px | Cards, inputs, modals, nav items, chart containers, table wrappers, workflow nodes. |
| `{rounded.lg}` | 10 px | Badges, sidebar footer buttons. |
| `{rounded.full}` | 9999 px | Status badges only (auth-key active/inactive). |

> **Note**: `{rounded.md}` 8px is the canonical radius used for the majority of components. 6px is reserved for buttons only. 10px is for circular badges. The single-radius approach (`--radius: 8px`) is an intentional simplification over a multi-tier radius system.

## Components

### Buttons

**`btn`** — the base button class.
- Padding `{components.btn.padding}` (6 px 16 px), background `{components.btn.backgroundColor}`, text `{components.btn.textColor}`, 1 px solid `{components.btn.borderColor}`, `{typography.button}`, `{rounded.sm}`.

**`btn-primary`** — the canonical blue CTA.
- Background `{colors.accent}`, text white, `{typography.button-strong}` (13 px 600), `{rounded.sm}`, `{elevation.level-3}` blue-tinted shadow. Hover darkens via `color-mix(in srgb, var(--accent) 90%, #fff 10%)`.

**`btn-danger`** — destructive action.
- Background `{colors.red}`, text white, `{typography.button-strong}`, `{elevation.level-4}` red-tinted shadow.

**`btn-danger-outline`** — outline destructive variant.
- Transparent background, `{colors.red}` text and border (mixed 55% with `{colors.border}`), `{typography.button-strong}`.

**`btn-with-icon`** — button with icon and label.
- `display: inline-flex`, `align-items: center`, gap 8 px, icon at 16 px.

**`table-action-btn`** — compact table action button.
- Padding 6 px 12 px, 12 px, `{rounded.sm}`, 500 weight.

**`table-icon-btn`** — icon-only table action button.
- 32 px × 32 px square, `{rounded.sm}`, no padding, icon at 14 px.

### Badges

**`badge`** — the canonical accent badge.
- `{typography.text-xs}` (10 px 600), uppercase, `{rounded.lg}` (10 px), padding 2 px 8 px, background `{colors.accent}`, white text.

**`provider-badge`** — model provider label.
- 12 px, 500 weight, `{rounded.lg}` (12 px), padding 2 px 10 px, `{colors.bg}` background, `{colors.border}` border.

**`status-badge`** — semantic status indicator (auth-keys, provider status).
- Active: `{colors.green}` tinted background + green text, `{rounded.full}` (9999 px), 12 px 600.
- Inactive: `{colors.red}` tinted background + red text, `{rounded.full}`.

### Cards & Containers

**`card`** — the canonical summary card. Background `{colors.surface}`, text `{colors.text}`, 1 px `{colors.border}`, `{rounded.md}` (8 px), `var(--space-card)` padding. Contains a top label (`{typography.uppercase-label}`, `{colors.text-muted}`, 8 px bottom margin) and a large value (`{typography.text-xxl}` 28 px 700).

**`card` (chart container)** — chart wrapper. Same card chrome, `var(--space-block)` padding.

**`modal`** — dialog surface. Background `{colors.surface}`, `{rounded.md}` (8 px), width 560 px, max-height 80 vh.

**`table-wrapper`** — table container. Background `{colors.surface}`, `{rounded.md}` (8 px), 1 px `{colors.border}`, `overflow: hidden`.

### Inputs & Forms

**`input`** — the canonical text input. Background `{colors.surface}`, text `{colors.text}`, 1 px `{colors.border}`, `{typography.text-sm-13}` (13 px), padding 8 px 10 px, `{rounded.md}` (8 px), min-height 38 px, focus border `{colors.accent}`.

**`form-input`** — variant with same specs but used in form grids.

**`textarea`** — same as input, min-height 60 px, `resize: vertical`.

**`form-field-label`** — `{typography.uppercase-label}`, `{colors.text-muted}`, `display: inline-block`.

**`form-error`** — `{colors.red}` text, 1 px red-tinted border, red-tinted background, 13 px 600.

### Navigation

**`sidebar`** — the signature collapsible, resizable sidebar. Width `{layout.sidebar-w}` (240 px) ↔ `{layout.sidebar-collapsed}` (60 px), background `{colors.surface}`, three-zone hierarchy:

- **Header**: `{layout.sidebar-header-h}` tall, 20 px padding, logo (28 px, `{colors.accent}`) + app name (18 px 700).
- **Primary nav**: items with `{layout.nav-item-padding}` (8 px 12 px), `{typography.text-base-500}` (14 px 500), `{rounded.md}`. Active state = `{colors.accent}` fill + white text. Notification dot = 8 px `{colors.orange}` circle.
- **Footer**: 16 px padding, `{colors.border}` top border. Contains theme toggle, API key button, access scope indicator, external auth user info.
- **Resize**: draggable handle (7 px wide, keyboard accessible with ArrowLeft/ArrowRight/Home/End), localStorage persistence, 200 ms transition.

**Nav items**: 14 px 500 weight, 10 px gap between icon and label. Icons at 18 px. Collapsed state hides labels, shows only icons.

### Theme Toggle

**`theme-toggle`** — three-state theme picker in the sidebar footer.

- **Wide form** (sidebar expanded, viewport > 768 px): three-button pill with `{colors.bg}` background, `{colors.border}` border, `{rounded.sm}` (6 px), 2 px padding. Each button 28 px × 24 px, `{rounded.sm}` (4 px), `{colors.text-muted}` default, `{colors.accent}` + white text when active. Icons at 14 px (Sun/Monitor/Moon from lucide).
- **Narrow form** (sidebar collapsed or viewport ≤ 768 px): single 36 px × 36 px button, cycles `light → system → dark`, icon at 16 px.

### Table

**`data-table`** — the canonical data table.
- Wrapper: `{colors.surface}` background, `{rounded.md}` (8 px), 1 px `{colors.border}`, `overflow: hidden`.
- Header row: `{colors.bg}` background, `{typography.uppercase-label}`, 12 px padding top/bottom, `var(--space-cell)` horizontal.
- Body rows: `{typography.text-base}` (14 px), 10 px padding top/bottom, `var(--space-cell)` horizontal, `{colors.border}` bottom border. Hover = `{colors.surface-hover}`.
- Last row: no bottom border.

### Loading Spinner

**`loading-spinner`** — 16 px × 16 px circle, `{colors.border}` border, `{colors.accent}` top border, `border-radius: 50%`, `animation: loading-spin 0.8s linear infinite`.

### Workflow Node

**`workflow-node`** — pipeline step card. `{colors.surface}` background, `{rounded.md}` (8 px), 1 px `{colors.border}`, 8 px 12 px padding, min-width 72 px.

### Alias Toggle

**`alias-toggle`** — model enabled/restricted toggle. 6 px 10 px padding, `{rounded.sm}` (6 px), 1 px `{colors.border}`, `{colors.bg}` background. Track: 34 px × 18 px, `{rounded.sm}` (6 px). Thumb: 16 px × 16 px, `{rounded.sm}` (6 px). Enabled state: green-tinted border and track.

## Theme Switching Mechanism

The dashboard implements a **three-state theme system** with CSS custom properties:

1. **State management**: `ThemeStore` (`ui.svelte.js`) with `theme` (`"light"` | `"system"` | `"dark"`) and `tick` (version counter for chart rebuilds).
2. **DOM binding**: `data-theme` attribute on `<html>` — `system` removes it, `light`/`dark` sets it.
3. **CSS cascade**:
   - `:root` = dark theme (default, `color-scheme: dark`)
   - `[data-theme="light"]` = explicit light theme
   - `@media (prefers-color-scheme: light) { :root:not([data-theme="dark"]) }` = system-follow light
4. **Persistence**: `localStorage` key `gomodel_theme`.
5. **Chart synchronisation**: `ChartCanvas.svelte` reads `themeStore.tick` in `$effect` — every theme change destroys and rebuilds all Chart.js instances, re-reading CSS variable colours via `getComputedStyle`.
6. **`light-dark()` not used**: `chartTheme.js` reads CSS variables via `getComputedStyle`, and `light-dark()` would pass the literal token stream to Chart.js instead of a resolved colour.

## Do's and Don'ts

### Do
- Reserve `{colors.accent}` (`#0066cc`) for primary actions, active states, links, and progress fills. The accent is the only conversion colour.
- Use the four-stop semantic palette (green / blue / orange / red) for status indicators — never for primary actions.
- Use `{rounded.sm}` 6 px for buttons, `{rounded.md}` 8 px for cards and inputs. The single-radius approach is intentional.
- Keep the sidebar collapsible and resizable with localStorage persistence; 200 ms transition.
- Keep the theme toggle in the sidebar footer — the dashboard has no graph toolbar, so the sidebar is the single entry point.
- Use `--space-*` tokens for all spacing values — components carry no spacing media queries.
- Use `color-mix()` for dynamic colour generation (hover states, tinted backgrounds, semantic borders) rather than hardcoding derived colours.
- Use `var(--accent)` for focus-visible outlines on every interactive element.

### Don't
- Don't use chromatic accents as button backgrounds — blue is the only primary action colour.
- Don't promote body weight beyond 400; keep the ceiling at 700 (page headers and statistics only).
- Don't break the cool-toned palette with warm surfaces — the blue colour temperature is a brand identifier.
- Don't introduce a fifth semantic colour — the four-stop palette is the system.
- Don't put language/theme selectors anywhere other than the sidebar footer (single entry point).
- Don't use `light-dark()` CSS function — Chart.js colour resolution requires concrete `rgb()` values.
- Don't hardcode spacing values — always use `var(--space-*)` tokens for responsive consistency.
- Don't use gradients for any element — the design has no gradient decoration.
- Don't use pill-shaped CTAs — pill shapes are reserved for status badges only.