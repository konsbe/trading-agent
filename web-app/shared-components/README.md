# @trading-agent/shared-components

Shared UI primitives, theme tokens and MFE utilities for the trading-agent web apps
(`web-app/spog` shell and the `mfe-*` remotes). No third-party UI library: every
component is plain React + CSS driven by the Stitch design-token CSS variables.

Theme tokens come from Stitch (**Momentum Scanner**, light + dark) — spec and
usage table in [`docs/design-system/`](../../docs/design-system/README.md).
`src/theme/tokens.json` is a verbatim copy of `docs/design-system/tokens.json`
(a test enforces it); `src/theme/tokens.ts` turns it into CSS variables
(`--color-<token>`, fonts, aliases).

Aliases with restricted use (see the design-system README):

- `--color-price-up` / `--color-price-down` — price deltas only
- `--color-status-ok` / `--color-status-warning` — infrastructure state (Data Source)
- `--color-status-constructive` / `-neutral` / `-stressed` / `-nodata` (+ `-container`,
  `on-…-container`) — the Market Report's macro classification indicators, mapped
  from the backend's stored `tone`. Never price, never stock signals.

Load the mode-independent tokens (spacing, radius, sizes) once at the app root:

```ts
import '@trading-agent/shared-components/theme.css';
```

Light/dark is applied only by a ThemeProvider: the shell's provider calls
`applyTheme(document.documentElement, mode)`; in an MFE, wrap the app in
`<ThemeProvider userData={userData}>` from this package.

## Entry points

| Entry | Use from |
|-------|----------|
| `src/index.ts` (package main) | MFEs – everything |
| `src/mfe.ts` | MFEs – hooks, providers, primitives |
| `src/shell.ts` | SPOG shell – UI only (no `shellSpog/*` remotes) |

## Components

- `Button` – `variant` (`primary`, `secondary`, `danger`, `ghost`), `size`, `iconOnly`, `fullWidth`
- `Dialog` – portal modal with `title`, `footer`, `onClose` (Escape / overlay click)
- `Spinner`, `Skeleton`, `FullSizeSkeleton`
- `SplitScreen` + `Pane` – two-pane resizable layout (ratio or pixel sizing)
- Icons – `MenuIcon`, `ArrowLeftIcon`, `CloseIcon`, `ChevronDownIcon`, `LogoutIcon`,
  `CheckCircleIcon`, `MinusCircleIcon`, `AlertTriangleIcon`, `CircleDashedIcon`, …
- `ThemeProvider` – applies the Stitch light/dark tokens to an MFE subtree
- `getThemeVariables`, `applyTheme`, `getSystemTheme` – token helpers (for ThemeProviders only)
- `ErrorBoundary`, `ContentWrapper`, `ContentRenderer`, `MFEDataWrapper`

```tsx
import { Button, Dialog } from '@trading-agent/shared-components';

<Dialog
  isOpen={open}
  title="Sign Out"
  onClose={close}
  footer={<Button variant="danger" onClick={logOut}>Log out</Button>}
>
  Are you sure you want to log out?
</Dialog>
```

## Development

```bash
nvm use 22
npm install
npm run build   # tsc + copy css/svg into dist/
npm test
npm run watch
```
