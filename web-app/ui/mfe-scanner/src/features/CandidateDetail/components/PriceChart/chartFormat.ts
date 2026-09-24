/**
 * Pure helpers for the price chart: axis/crosshair time labels and colour
 * alpha. Intraday bars are labelled in New York time (the session's clock);
 * daily bars are UTC-midnight stamps, so they are formatted in UTC to avoid
 * shifting the date west of Greenwich.
 */

export const NEW_YORK = 'America/New_York';

/** Mirrors Lightweight Charts' TickMarkType enum (Year, Month, DayOfMonth, Time, TimeWithSeconds). */
export const TICK = { Year: 0, Month: 1, DayOfMonth: 2, Time: 3, TimeWithSeconds: 4 } as const;

const fmt = (timeZone: string, options: Intl.DateTimeFormatOptions) =>
    new Intl.DateTimeFormat('en-US', { timeZone, ...options });

const toDate = (time: unknown): Date | null => (typeof time === 'number' ? new Date(time * 1000) : null);

export const formatTickMark = (time: unknown, tickMarkType: number, intraday: boolean): string => {
    const date = toDate(time);
    if (!date) return String(time);
    const zone = intraday ? NEW_YORK : 'UTC';
    switch (tickMarkType) {
        case TICK.Year:
            return fmt(zone, { year: 'numeric' }).format(date);
        case TICK.Month:
            return fmt(zone, { month: 'short' }).format(date);
        case TICK.DayOfMonth:
            return fmt(zone, { month: 'short', day: 'numeric' }).format(date);
        default:
            return intraday
                ? fmt(zone, { hour: '2-digit', minute: '2-digit', hourCycle: 'h23' }).format(date)
                : fmt(zone, { month: 'short', day: 'numeric' }).format(date);
    }
};

export const formatCrosshairTime = (time: unknown, intraday: boolean): string => {
    const date = toDate(time);
    if (!date) return String(time);
    return intraday
        ? `${fmt(NEW_YORK, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit', hourCycle: 'h23' }).format(date)} ET`
        : fmt('UTC', { weekday: 'short', month: 'short', day: 'numeric', year: 'numeric' }).format(date);
};

/**
 * `#rgb`/`#rrggbb`/`rgb()` → `rgba()` with `alpha`. Theme colours are hex in
 * tokens.json; anything unparseable is returned unchanged rather than guessed.
 */
export const withAlpha = (color: string, alpha: number): string => {
    const value = color.trim();
    const hex = /^#([0-9a-f]{3}|[0-9a-f]{6})$/i.exec(value);
    if (hex) {
        const digits = hex[1].length === 3 ? hex[1].replace(/./g, c => c + c) : hex[1];
        const [r, g, b] = [0, 2, 4].map(i => parseInt(digits.slice(i, i + 2), 16));
        return `rgba(${r}, ${g}, ${b}, ${alpha})`;
    }
    const rgb = /^rgba?\(\s*(\d+)[\s,]+(\d+)[\s,]+(\d+)/i.exec(value);
    if (rgb) return `rgba(${rgb[1]}, ${rgb[2]}, ${rgb[3]}, ${alpha})`;
    return value;
};
