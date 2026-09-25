import { Severity, TechnicalAnalysis } from '@/api';
import { formatNumber } from '@/common/format/format';
import { bandLabel } from '../../../utils/analysisFormat';

export interface FlaggedReading {
    key: string;
    /** "RSI 75.1". */
    reading: string;
    severity: Severity;
    /** The stored band in plain text, e.g. "overbought". */
    band: string | null;
}

/**
 * The technical readings the API itself flagged (non-null `severity`), in grid
 * order. Only the API's severity decides membership; no threshold is applied here.
 */
export const flaggedReadings = (t: TechnicalAnalysis): FlaggedReading[] => {
    const readings: FlaggedReading[] = [];
    if (t.rsi_14.severity) {
        readings.push({ key: 'rsi', reading: `RSI ${formatNumber(t.rsi_14.value, 1)}`, severity: t.rsi_14.severity, band: bandLabel(t.rsi_14.band) });
    }
    if (t.bb_squeeze.severity) {
        const band = t.bb_squeeze.active === null ? null : t.bb_squeeze.active ? 'active' : 'not active';
        readings.push({ key: 'bb_squeeze', reading: 'BB squeeze', severity: t.bb_squeeze.severity, band });
    }
    if (t.vix_regime.severity) {
        readings.push({
            key: 'vix',
            reading: `VIX ${formatNumber(t.vix_regime.value, 1)}`,
            severity: t.vix_regime.severity,
            band: bandLabel(t.vix_regime.band),
        });
    }
    return readings;
};
