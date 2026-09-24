import { parseBacktestReport } from './parsers';
import { makeReportBody, SHARED_REPORT_JSON } from '@/test-utils/fixtures';

const OPTIONAL_PATHS = [
    /^rvol_stratification_funnel\.steps\[\d+\]\.(excess_odds|composition_share_pct|verdict)$/,
    /^research_round_1\.hypotheses\[\d+\]\.verdict_note$/,
];

const isOptional = (path: string) => OPTIONAL_PATHS.some(re => re.test(path));

/** Every object key path in the report (recursing into arrays of objects), e.g. `research_round_1.hypotheses[1].verdict_note`. */
const keyPaths = (value: unknown, prefix = ''): string[] => {
    if (Array.isArray(value)) {
        return value.flatMap((item, i) => (item !== null && typeof item === 'object' ? keyPaths(item, `${prefix}[${i}]`) : []));
    }
    if (value === null || typeof value !== 'object') return [];
    return Object.entries(value).flatMap(([key, child]) => {
        const path = prefix ? `${prefix}.${key}` : key;
        return [path, ...keyPaths(child, path)];
    });
};

const deleteAt = (root: any, path: string) => {
    const parts = path.match(/[^.[\]]+/g)!;
    const parent = parts.slice(0, -1).reduce((node, part) => node[part], root);
    delete parent[parts[parts.length - 1]];
};

const setAt = (root: any, path: string, value: unknown) => {
    const parts = path.match(/[^.[\]]+/g)!;
    const parent = parts.slice(0, -1).reduce((node, part) => node[part], root);
    parent[parts[parts.length - 1]] = value;
};

describe('parseBacktestReport', () => {
    it('parses the shared report file exactly, with no keys added or dropped', () => {
        expect(parseBacktestReport(makeReportBody())).toStrictEqual(SHARED_REPORT_JSON);
    });

    it('keeps the load-bearing numbers the spec pins (API addendum §3 drift check)', () => {
        const report = parseBacktestReport(makeReportBody());

        expect(report.entry_gate.result.p_value).toBe(0.947);
        expect(report.entry_gate.result.mh_odds_ratio).toBe(0.991);
        expect(report.v2_score_finding.composition_share_pct).toBe(97);
        expect(report.rvol_stratification_funnel.steps[3].odds_ratio).toBe(0.991);
        expect(report.entry_gate.rule_committed_before_result).toBe(true);
        expect(report.sample_size).toMatchObject({ applies_to: 'research_round_1', episodes: 7579, lockbox_opened: false });
    });

    it('keeps optional funnel fields only on the steps that carry them', () => {
        const steps = parseBacktestReport(makeReportBody()).rvol_stratification_funnel.steps;

        expect(steps.map(s => 'excess_odds' in s)).toEqual([true, true, true, false]);
        expect(steps.map(s => 'composition_share_pct' in s)).toEqual([false, true, true, false]);
        expect(steps.map(s => 'verdict' in s)).toEqual([false, false, false, true]);
        expect(steps[3].verdict).toBe('indistinguishable from no effect');
    });

    it('keeps verdict_note only on hypothesis b, and parses the abandoned hypothesis', () => {
        const round = parseBacktestReport(makeReportBody()).research_round_1;

        expect(round.hypotheses.filter(h => h.verdict_note !== undefined).map(h => h.id)).toEqual(['b']);
        expect(round.hypotheses.map(h => h.id)).toEqual(['a', 'b', 'd', 'e']);
        expect(round.hypotheses[1].best_effect.ci).toEqual([1.389, 1.88]);
        expect(round.abandoned).toEqual([expect.objectContaining({ id: 'c', label: 'Catalyst via Tiingo News' })]);
    });

    it('accepts a report without any optional field', () => {
        const body = makeReportBody();
        keyPaths(body).filter(isOptional).reverse().forEach(path => deleteAt(body, path));

        const report = parseBacktestReport(body);

        expect(report.rvol_stratification_funnel.steps.every(s => Object.keys(s).sort().join() === 'label,odds_ratio')).toBe(true);
        expect(report.research_round_1.hypotheses.some(h => 'verdict_note' in h)).toBe(false);
    });

    it('accepts an empty abandoned list', () => {
        const body = makeReportBody();
        body.research_round_1.abandoned = [];
        expect(parseBacktestReport(body).research_round_1.abandoned).toEqual([]);
    });

    const requiredPaths = keyPaths(SHARED_REPORT_JSON).filter(path => !isOptional(path));

    it('covers every section of the report', () => {
        expect(requiredPaths.length).toBeGreaterThan(60);
    });

    it.each(requiredPaths)('rejects a body missing %s', path => {
        const body = makeReportBody();
        deleteAt(body, path);
        expect(() => parseBacktestReport(body)).toThrow(`at ${path}:`);
    });

    it.each([
        ['a string body', () => 'nope', '$'],
        ['an array body', () => [], '$'],
        ['a null body', () => null, '$'],
    ])('rejects %s', (_label, make, path) => {
        expect(() => parseBacktestReport(make())).toThrow(`at ${path}:`);
    });

    it.each([
        ['a null optional excess_odds', 'rvol_stratification_funnel.steps[0].excess_odds', null],
        ['a string composition_share_pct', 'rvol_stratification_funnel.steps[1].composition_share_pct', '59'],
        ['a numeric step verdict', 'rvol_stratification_funnel.steps[3].verdict', 0],
        ['a null verdict_note', 'research_round_1.hypotheses[1].verdict_note', null],
        ['a string p_value', 'entry_gate.result.p_value', '0.947'],
        ['a non-finite odds ratio', 'rvol_stratification_funnel.steps[0].odds_ratio', Infinity],
        ['a string lockbox flag', 'sample_size.lockbox_opened', 'false'],
        ['a closed_date that is not YYYY-MM-DD', 'report.closed_date', '22 Sep 2026'],
        ['a ci with one bound', 'research_round_1.hypotheses[0].best_effect.ci', [0.986]],
        ['a ci with reversed bounds', 'research_round_1.hypotheses[0].best_effect.ci', [1.449, 0.986]],
        ['a ci with a string bound', 'research_round_1.hypotheses[0].best_effect.ci', [0.986, '1.449']],
        ['steps that are not an array', 'rvol_stratification_funnel.steps', {}],
        ['an abandoned entry that is not an object', 'research_round_1.abandoned[0]', 'c'],
    ])('rejects %s', (_label, path, value) => {
        const body = makeReportBody();
        setAt(body, path, value);
        expect(() => parseBacktestReport(body)).toThrow(`at ${path}`);
    });
});
