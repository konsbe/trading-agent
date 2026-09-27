import {
    addLocalDays,
    defaultRange,
    isLocalDay,
    localMidnight,
    parseLocalDay,
    rangeBounds,
    toLocalDay,
    toOffsetTimestamp,
} from './localDays';

const HOUR = 3_600_000;

// jest.config.js pins TZ to Europe/Athens (+03:00 summer, +02:00 winter).
describe('local days', () => {
    it('runs in the pinned zone', () => {
        expect(Intl.DateTimeFormat().resolvedOptions().timeZone).toBe('Europe/Athens');
    });

    it('parses only real calendar days', () => {
        expect(parseLocalDay('2026-09-27')).toEqual({ y: 2026, m: 9, d: 27 });
        expect(isLocalDay('2026-02-30')).toBe(false);
        expect(isLocalDay('2026-9-27')).toBe(false);
        expect(isLocalDay('')).toBe(false);
    });

    it('steps whole calendar days across month and year ends', () => {
        expect(addLocalDays('2026-09-30', 1)).toBe('2026-10-01');
        expect(addLocalDays('2026-12-31', 1)).toBe('2027-01-01');
        expect(addLocalDays('2026-03-01', -1)).toBe('2026-02-28');
        expect(() => addLocalDays('nope', 1)).toThrow('not a calendar day');
        expect(() => localMidnight('nope')).toThrow('not a calendar day');
    });

    it('sends local midnight of "from" and of the day after "to", with that day\'s offset', () => {
        expect(rangeBounds({ from: '2026-09-27', to: '2026-09-27' })).toEqual({
            since: '2026-09-27T00:00:00+03:00',
            until: '2026-09-28T00:00:00+03:00',
        });
        expect(rangeBounds({ from: '2026-09-21', to: '2026-09-27' })).toEqual({
            since: '2026-09-21T00:00:00+03:00',
            until: '2026-09-28T00:00:00+03:00',
        });
    });

    it('is DST-safe when the range spans the autumn change (Oct 25: +03:00 → +02:00)', () => {
        const bounds = rangeBounds({ from: '2026-10-24', to: '2026-10-25' });

        expect(bounds).toEqual({ since: '2026-10-24T00:00:00+03:00', until: '2026-10-26T00:00:00+02:00' });
        // Oct 25 has 25 hours: bounds are local midnights, not 24-hour steps.
        expect(Date.parse(bounds.until!) - Date.parse(bounds.since!)).toBe(49 * HOUR);
    });

    it('is DST-safe across the spring change (Mar 29: +02:00 → +03:00)', () => {
        const bounds = rangeBounds({ from: '2026-03-29', to: '2026-03-29' });

        expect(bounds).toEqual({ since: '2026-03-29T00:00:00+02:00', until: '2026-03-30T00:00:00+03:00' });
        expect(Date.parse(bounds.until!) - Date.parse(bounds.since!)).toBe(23 * HOUR);
    });

    it('writes any instant as local wall-clock time with its offset', () => {
        expect(toOffsetTimestamp(new Date('2026-09-27T02:06:07Z'))).toBe('2026-09-27T05:06:07+03:00');
        expect(toOffsetTimestamp(new Date('2026-01-05T22:00:00Z'))).toBe('2026-01-06T00:00:00+02:00');
    });

    it('leaves a side open when its date is empty or partial', () => {
        expect(rangeBounds({ from: '', to: '' })).toEqual({});
        expect(Object.keys(rangeBounds({ from: '2026-09-01', to: '2026-09' }))).toEqual(['since']);
        expect(Object.keys(rangeBounds({ from: '', to: '2026-09-10' }))).toEqual(['until']);
    });

    it('defaults to the last 7 local days, today included', () => {
        // 22:30 UTC on the 27th is already the 28th locally.
        expect(defaultRange(new Date('2026-09-27T22:30:00Z'))).toEqual({ from: '2026-09-22', to: '2026-09-28' });
        expect(defaultRange(new Date('2026-09-27T20:59:00Z'))).toEqual({ from: '2026-09-21', to: '2026-09-27' });
        expect(toLocalDay(new Date('2026-09-27T20:59:00Z'))).toBe('2026-09-27');
    });
});
