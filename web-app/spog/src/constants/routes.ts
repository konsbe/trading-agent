export interface AppRoute {
    path: string;
    label: string;
    roles?: string[];
}

export const APP_ROUTES: AppRoute[] = [
    { path: '/candidates', label: 'Candidates' },
    { path: '/stock-detail', label: 'Stock Detail' },
    { path: '/backtest-lab', label: 'Backtest Lab' },
    { path: '/alarm-history', label: 'Alarm History' },
    { path: '/watchlist', label: 'Watchlist' },
    { path: '/tracked-positions', label: 'Tracked Positions' },
    { path: '/data-source', label: 'Data Source' },
    { path: '/settings', label: 'Settings' },
];

export const DEFAULT_ROUTE = APP_ROUTES[0].path;
