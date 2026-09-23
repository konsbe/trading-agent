# Momentum Scanner design system (Stitch)

Source of truth for the trading-agent UI theme, exported from Stitch.

| File | What it is |
|------|------------|
| `DESIGN.light.md` | Stitch spec for **Momentum Scanner Light** (colors, typography, rules) |
| `DESIGN.dark.md` | Stitch spec for **Momentum Scanner** (dark) |
| `tokens.json` | Unified W3C-format tokens for both themes, with per-token intent |

## How the tokens reach the UI

```
docs/design-system/tokens.json   (canonical)
        │ copied verbatim — a test fails if they drift
        ▼
web-app/shared-components/src/theme/tokens.json
        │ tokens.ts → getThemeVariables(mode) / applyTheme(el, mode)
        ▼
ThemeProvider (the ONLY places light/dark is applied)
  • shell: web-app/spog/src/providers/ThemeProvider/ThemeProvider.tsx
           → applyTheme(document.documentElement, theme)
  • MFEs:  web-app/shared-components/src/ui/providers/ThemeProvider/ThemeProvider.tsx
           → CSS variables on the .ta-theme-root wrapper
```

`web-app/shared-components/src/theme/theme.css` holds only mode-independent
tokens (spacing, radius, font sizes/weights). Never add light/dark colour values
to CSS files, and never read `prefers-color-scheme` outside the ThemeProviders
(`getSystemTheme` lives in `shared-components/src/theme/tokens.ts`).

To change the theme: re-export from Stitch, replace `docs/design-system/tokens.json`,
copy it to `web-app/shared-components/src/theme/tokens.json`, and run the
shared-components tests.

## CSS variables

Every Stitch colour is exposed as `--color-<token>`, e.g. `--color-surface-container-high`.

| Variable | Use for |
|----------|---------|
| `--color-surface` | Viewport / page backdrop (alias `--color-app-background`) |
| `--color-surface-container-lowest` | Deepest canvas / lowest cards |
| `--color-surface-container-low` | Header, sidebar, metric cards, table rows |
| `--color-surface-container` | Standard card body |
| `--color-surface-container-high` | Elevated cards, hovered rows, search input, dialogs, secondary buttons |
| `--color-surface-container-highest` | Popovers, menus, active segment pills |
| `--color-surface-variant` | Secondary groupings, chip fills |
| `--color-on-surface` | Primary text, tickers, values (alias `--color-text`) |
| `--color-on-surface-variant` | Secondary labels (alias `--color-text-secondary`) |
| `--color-outline` | Interactive borders, search stroke |
| `--color-outline-variant` | Dividers, row separators, grid lines (alias `--color-border`) |
| `--color-primary` / `--color-on-primary` | Primary actions, active indicators |
| `--color-primary-container` / `--color-on-primary-container` | Tinted pills, active filters, active nav item |
| `--color-secondary` (+ `on-`/`-container`) | Live telemetry badges, status pulses |
| `--color-tertiary` (+ `on-`/`-container`) | Disclaimer pill "Screener — not a forecast", warnings |
| `--color-error` (+ `on-`/`-container`) | System alerts, feed disconnects |

Other theme variables: `--color-focus-ring`, `--color-overlay`, `--shadow-sm`, `--shadow-lg`.

Hover/pressed states are state layers over the base colour, not new tokens:

```css
background-color: color-mix(in srgb, var(--color-primary), var(--color-on-primary) 8%);
```

## Typography

| Variable | Value | Use for |
|----------|-------|---------|
| `--font-family` | Inter | Body text |
| `--font-family-headline` | Inter | Headings, brand, dialog titles |
| `--font-family-label` | JetBrains Mono | Tickers, prices, numeric labels |

The shell loads Inter and JetBrains Mono from Google Fonts in `web-app/spog/public/index.html`.

Roundness is `ROUND_FOUR`: `--radius-md` = 4px (default), `--radius-sm` = 2px,
`--radius-lg` = 8px (cards), `--radius-full` for pills.

## Rules

1. **Green and red are reserved for price movement deltas.** Never use them in
   UI chrome (buttons, nav, borders, badges). Live telemetry uses
   `secondary`; system alerts use `error`.
2. **Disclaimer pill "Screener — not a forecast"** uses the tertiary (amber)
   palette (`tertiary-container` fill, `on-tertiary-container` text) and stays
   visible on every screen.
3. Light theme: clean white/off-white surfaces, crisp low-contrast containers
   for data density; 600-level tints for WCAG AA (≥ 4.5:1).
4. Dark theme: 500-level tints on obsidian backdrops to avoid halation.
