import { makeActiveRow, makeClosedRow, makeUnevaluatedRow } from '@/test-utils/tracked';
import { isNotYetEvaluated, rowKey, splitByStatus } from './rows';

describe('rows', () => {
    it('keys a row by symbol and alert date, so a symbol tracked twice stays two rows', () => {
        expect(rowKey(makeActiveRow({ symbol: 'SDEV', alerted_date: '2026-09-24' }))).toBe('SDEV-2026-09-24');
        expect(rowKey(makeActiveRow({ symbol: 'SDEV', alerted_date: '2026-09-25' }))).toBe('SDEV-2026-09-25');
    });

    it('treats only a row without last_evaluated_date as not yet evaluated', () => {
        expect(isNotYetEvaluated(makeUnevaluatedRow())).toBe(true);
        expect(isNotYetEvaluated(makeActiveRow({ sessions_elapsed: 0 }))).toBe(false);
    });

    it('splits rows by status, keeping their order', () => {
        const rows = [makeUnevaluatedRow(), makeClosedRow(), makeActiveRow()];
        expect(splitByStatus(rows)).toEqual({ active: [rows[0], rows[2]], closed: [rows[1]] });
    });
});
