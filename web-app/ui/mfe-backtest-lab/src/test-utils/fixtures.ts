import reportJson from '../../../../../shared/content/backtest_lab_report.json';
import { BacktestLabReport } from '@/api';

/**
 * The file momentum-api serves verbatim, imported from the repo (not copied),
 * so tests always run against the current report.
 */
export const SHARED_REPORT_JSON: unknown = reportJson;

/** A fresh, mutable deep copy of the shared report body. */
export const makeReportBody = (): any => JSON.parse(JSON.stringify(reportJson));

export const makeReport = (): BacktestLabReport => makeReportBody() as BacktestLabReport;

/** Minimal `fetch` Response stand-in (jsdom has no Response). */
export const mockResponse = (status: number, body: unknown, { raw = false } = {}): Response =>
    ({
        ok: status >= 200 && status < 300,
        status,
        text: () => Promise.resolve(raw ? String(body) : body === undefined ? '' : JSON.stringify(body)),
    }) as unknown as Response;
