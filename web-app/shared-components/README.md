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
- `SeverityBadge` – API alert `severity` (info / notice / warning) as a dot + word tag; `SEVERITY_LEVELS`
- `SplitScreen` + `Pane` – two-pane resizable layout (ratio or pixel sizing)
- Icons – `MenuIcon`, `ArrowLeftIcon`, `CloseIcon`, `ChevronDownIcon`, `LogoutIcon`,
  `CheckCircleIcon`, `MinusCircleIcon`, `AlertTriangleIcon`, `CircleDashedIcon`, …
- `ThemeProvider` – applies the Stitch light/dark tokens to an MFE subtree
- `getThemeVariables`, `applyTheme`, `getSystemTheme` – token helpers (for ThemeProviders only)
- `ErrorBoundary`, `ContentWrapper`, `ContentRenderer`, `MFEDataWrapper`
- Market cells – `ChangeCell` (`value`), `MarketCapCell` (`row: MarketCapFields`), `ScoreCell`
  (`row: ScoreFields`, "53/75 unvalidated" / "— unvalidated") and `MARKET_COLUMNS` (Close … Score
  with the candidates table's labels, order and tooltips). Any row satisfying `MarketRow` renders
  through them — the scanner's candidates table and the watchlist table both do. Each is also a
  `TableColumn` (sorted by value — market cap by the value shown, breakout/catalyst by rank —
  and searched by the text shown), so they plug straight into `useTableView`.
- `PageHeader` – `title` (the h1), `subtitle?`, `back?: { label, to?, onClick? }` (a router `Link`
  with `to`, else a link-styled button), `badges?: ReactNode[]`, `actions?`. Presentational; not
  spog's `HeaderComponent`.
- Tables – `useTableView`, `SortableHeader`, `TableSearch`, `StockDetailLink` (see below)
- Formatters – `EMPTY_VALUE`, `formatPrice`, `formatSignedPercent`, `formatMultiple`, `formatCompactUsd`,
  `formatInteger`, `formatBreakoutState`, `formatRatioAsPercent`, `formatCatalystTier`, `formatScore`,
  `marketCapText`, `marketCapIsEstimate`, … and `signOf` / `formatSignedNumber` (negatives use "−", U+2212)

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

## Sortable, searchable tables

`react-router-dom` (≥ 7, a Module Federation singleton in spog and every MFE) is a
peer dependency: `useTableView` keeps its state in the URL and `StockDetailLink` /
`PageHeader` render router links. MFE jest configs must map `^react-router-dom$`
(and `^react-router$`) to their own `node_modules`, as they do for React, so the
shared code sees the app's Router context.

```tsx
import { SortableHeader, TableSearch, TableColumn, useTableView } from '@trading-agent/shared-components';

const COLUMNS: TableColumn<Row>[] = [
    { key: 'symbol', label: 'Symbol', searchText: r => `${r.symbol} ${r.company ?? '—'}` },
    { key: 'close', label: 'Close', searchText: r => formatPrice(r.close) },
    { key: 'remove', label: 'Remove', sortable: false },
];

const view = useTableView({ rows, columns: COLUMNS, defaultSort: { key: 'close', direction: 'desc' }, urlKey: 'watchlist' });

<TableSearch value={view.query} onChange={view.setQuery} total={view.total} shown={view.shown} controls="tbl" />
<table id="tbl">
    <thead><tr>{COLUMNS.map(c => <SortableHeader key={c.key} {...view.headerProps(c.key)} />)}</tr></thead>
    <tbody>{view.rows.map(/* … */)}</tbody>
</table>
```

- **`useTableView<Row>({ rows, columns, defaultSort, urlKey, tieBreak? })`** →
  `{ rows, sort, toggleSort(key), query, setQuery(text), headerProps(key), total, shown }`.
  - Column: `{ key, label, sortable? = true, sortValue?(row), searchText?(row), initialDirection? }`.
    `sortValue` defaults to `row[key]` (number/string); `searchText` — the text **as displayed** —
    defaults to `String(sortValue)`.
  - Sort: clicking the active column toggles; another column starts at its `initialDirection`
    (else asc for strings, desc for numbers). Missing values (null, undefined, NaN, ±Infinity —
    what renders "—") sort **last in both directions**; ties go to `tieBreak` (default: `symbol`
    ascending when rows have one), then input order (stable).
  - Search: case-insensitive substring of the trimmed query over every column's `searchText`;
    `sortable: false` columns only when they supply `searchText`. Empty = all rows.
  - URL: `${urlKey}_sort=<key>:<asc|desc>` and `${urlKey}_q=<text>`, read on mount (reload / back
    restore the view), written with `replace` (no history entry per click), other params untouched,
    the default sort omitted. An unknown or unsortable key falls back to `defaultSort`. Give each
    table on a page its own `urlKey`.
  - Pure helpers, exported and unit-tested: `sortRows`, `filterRows`, `isMissingSortValue`, `bySymbol`,
    `initialSortDirection`, `columnSortValue`, `columnSearchText`, `parseSort`, `serializeSort`,
    `sortParamName`, `queryParamName`.
- **`SortableHeader`** – a `<th>` (all th attributes pass through) with a full-cell button,
  `aria-sort` on the th, and a fixed-size drawn ▲/▼ on the active column (dim ⇅ otherwise);
  `align="end"` for numeric columns; `sortable={false}` renders a plain th. Sortable cells have no
  padding of their own (the button has it).
- **`TableSearch`** – `value`, `onChange` (debounced `TABLE_SEARCH_DEBOUNCE_MS` = 150 ms; clear
  button and Escape clear at once), `label` ("Search this table", visually hidden unless
  `showLabel`), `placeholder` ("Filter rows…"), `total` / `shown` → "N of M rows" while filtering,
  `controls` (table id).

## Stock Detail links

Stock Detail is mfe-scanner's `/candidates/:symbol`. Link to it so the page can offer
"← Back to {label}" to the exact list URL (sort and search included):

- `stockDetailLink(symbol, { label, from? }, { hash? })` → `{ pathname: '/candidates/<symbol>', hash?,
  state: { from, fromLabel } }`; `from` defaults to `window.location` pathname + search. Use as
  `navigate({ pathname, hash }, { state })`.
- `<StockDetailLink symbol originLabel="Watchlist" hash? className …>` – router `Link` that takes
  `from` from the router location; renders the symbol unless given children.
- `readStockDetailState(location.state)` → `{ from, fromLabel } | null` (only internal `/…` paths).
- `isStockDetailEligible({ symbol, exchange_type?, asset_type?, alert_type? })` – **only stocks and
  funds link.** False for crypto (`exchange_type`/`asset_type` `crypto`, or a `*USDT` / `*USDC` /
  `*BUSD` pair), Treasury yields and indices (`asset_type` `treasury_yield` / `yield` / `index`, a
  `^`-prefixed symbol), market-wide rows (`alert_type` `vix_elevated`, the `VIX` itself) and an
  empty symbol. Render those symbols as plain text.

## Development

```bash
nvm use 22
npm install
npm run build   # tsc + copy css/svg into dist/
npm test
npm run watch
```
