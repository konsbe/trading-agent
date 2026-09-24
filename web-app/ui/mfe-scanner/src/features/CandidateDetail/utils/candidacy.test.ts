import { candidacyNotice } from './candidacy';

const base = { is_candidate_today: false, gates_passed: false, as_of: '2026-09-23', latest_scan_date: '2026-09-23' };

describe('candidacyNotice', () => {
    it('is null for a candidate today, so candidates keep their layout', () => {
        expect(candidacyNotice({ ...base, is_candidate_today: true, gates_passed: true })).toBeNull();
    });

    it("says the symbol failed today's gates when its row is from the latest scan", () => {
        expect(candidacyNotice(base)).toEqual({
            title: "Not in today's candidates",
            reason: "Didn't pass today's gates — the gates panel below shows which.",
        });
    });

    it('names both dates when the newest row predates the latest scan', () => {
        expect(candidacyNotice({ ...base, as_of: '2026-09-21' })).toEqual({
            title: "Not in today's candidates",
            reason: 'Latest data is from Sep 21, 2026: no newer daily bar has arrived for this symbol, so the Sep 23, 2026 scan has nothing newer to show.',
        });
    });

    it('treats an older pass as not a candidate today (stale, not a gate failure)', () => {
        expect(candidacyNotice({ ...base, as_of: '2026-09-21', gates_passed: true })?.reason).toMatch(/^Latest data is from Sep 21, 2026/);
    });

    it('gives no reason for a same-day pass that the API still says is not a candidate', () => {
        expect(candidacyNotice({ ...base, gates_passed: true })).toEqual({ title: "Not in today's candidates", reason: null });
    });
});
