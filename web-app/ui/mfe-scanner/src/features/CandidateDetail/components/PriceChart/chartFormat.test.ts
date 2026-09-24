import { formatCrosshairTime, formatTickMark, TICK, withAlpha } from './chartFormat';

// 2026-09-21 13:30 UTC = 09:30 New York (EDT), the regular-session open.
const OPEN_2026_09_21 = Date.UTC(2026, 8, 21, 13, 30) / 1000;
// Daily bars are stamped at UTC midnight.
const DAY_2026_09_21 = Date.UTC(2026, 8, 21) / 1000;

describe('chartFormat', () => {
    it('labels intraday ticks in New York time', () => {
        expect(formatTickMark(OPEN_2026_09_21, TICK.Time, true)).toBe('09:30');
        expect(formatTickMark(OPEN_2026_09_21, TICK.DayOfMonth, true)).toBe('Sep 21');
        expect(formatCrosshairTime(OPEN_2026_09_21, true)).toBe('Sep 21, 09:30 ET');
    });

    it('labels daily ticks as UTC calendar dates (no westward shift)', () => {
        expect(formatTickMark(DAY_2026_09_21, TICK.DayOfMonth, false)).toBe('Sep 21');
        expect(formatTickMark(DAY_2026_09_21, TICK.Time, false)).toBe('Sep 21');
        expect(formatTickMark(DAY_2026_09_21, TICK.Month, false)).toBe('Sep');
        expect(formatTickMark(DAY_2026_09_21, TICK.Year, false)).toBe('2026');
        expect(formatCrosshairTime(DAY_2026_09_21, false)).toBe('Mon, Sep 21, 2026');
    });

    it('passes through non-numeric times', () => {
        expect(formatTickMark('2026-09-21', TICK.DayOfMonth, false)).toBe('2026-09-21');
        expect(formatCrosshairTime({ year: 2026 }, false)).toBe('[object Object]');
    });

    it('adds alpha to hex and rgb colours and leaves anything else untouched', () => {
        expect(withAlpha('#10b981', 0.45)).toBe('rgba(16, 185, 129, 0.45)');
        expect(withAlpha(' #fff ', 0.5)).toBe('rgba(255, 255, 255, 0.5)');
        expect(withAlpha('rgb(1, 2, 3)', 0.2)).toBe('rgba(1, 2, 3, 0.2)');
        expect(withAlpha('oklch(0.7 0.1 150)', 0.2)).toBe('oklch(0.7 0.1 150)');
    });
});
