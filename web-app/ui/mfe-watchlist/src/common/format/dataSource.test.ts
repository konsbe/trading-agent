import { makeDailyBarsItem, makeWatchlistItem, TSM_MARKET_CAP_NOTE } from '@/test-utils/fixtures';
import { dailyBarsProviders, dailyBarsTooltip, isDailyBarsRow, marketCapNoteLabel, providerName } from './dataSource';

describe('providerName', () => {
    it('names known providers and shows an unknown id as served', () => {
        expect(providerName('yahoo_finance')).toBe('Yahoo Finance');
        expect(providerName('tiingo')).toBe('Tiingo');
        expect(providerName('stooq')).toBe('stooq');
    });
});

describe('dailyBarsProviders', () => {
    it('lists each daily-bar provider once, ignoring other sources', () => {
        expect(
            dailyBarsProviders({
                close: 'daily_bars:yahoo_finance',
                rsi_14: 'daily_bars:yahoo_finance',
                volume: 'daily_bars:tiingo',
                market_cap: 'finnhub_metric',
            })
        ).toEqual(['Yahoo Finance', 'Tiingo']);
        expect(dailyBarsProviders(null)).toEqual([]);
    });
});

describe('dailyBarsTooltip', () => {
    it('explains the row and names the source', () => {
        expect(dailyBarsTooltip(makeDailyBarsItem())).toBe(
            "Outside the scanner's universe: computed from this symbol's own daily bars (source: Yahoo Finance) with the scanner's formulas. Not a scanner reading — no score."
        );
    });

    it('drops the source when none is named', () => {
        expect(dailyBarsTooltip({ sources: null })).toBe(
            "Outside the scanner's universe: computed from this symbol's own daily bars with the scanner's formulas. Not a scanner reading — no score."
        );
    });
});

describe('isDailyBarsRow', () => {
    it('is true only for data_source "daily_bars"', () => {
        expect(isDailyBarsRow(makeDailyBarsItem())).toBe(true);
        expect(isDailyBarsRow(makeWatchlistItem())).toBe(false);
        expect(isDailyBarsRow({ data_source: null })).toBe(false);
    });
});

describe('marketCapNoteLabel', () => {
    it('names the reporting currency when the note does', () => {
        expect(marketCapNoteLabel(TSM_MARKET_CAP_NOTE)).toBe('Not in USD (TWD) — not converted');
    });

    it('says "Not in USD" when the note names no currency, else "See note"', () => {
        expect(marketCapNoteLabel('market_cap is null: Finnhub marketCapitalization is not in USD (priced on a non-US listing)')).toBe(
            'Not in USD — not converted'
        );
        expect(marketCapNoteLabel('market_cap is null: no figure')).toBe('See note');
    });
});
