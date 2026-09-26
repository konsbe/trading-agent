import { CollapsibleCard } from '@trading-agent/shared-components';
import { QualitativeAnalysis } from '@/api';
import { formatNumber, formatPercent } from '@/common/format/format';
import FactGrid, { FactCell } from '../../FactGrid';
import { MOAT_PROXY_TIER_LABELS, RD_INTENSITY_TIER_LABELS, withBand } from '../../../utils/analysisFormat';
import { QualitativeSectionProps } from './types';
import '@/styles/scanner-global.css';
import '../analysis-styles.css';

export const buildQualitativeCells = (q: QualitativeAnalysis): FactCell[] => [
    {
        key: 'moat_proxy',
        label: 'Moat proxy',
        value: withBand(formatNumber(q.moat_proxy.value, 2), q.moat_proxy.tier, MOAT_PROXY_TIER_LABELS),
    },
    { key: 'insider_signal', label: 'Insider activity', value: withBand(formatNumber(q.insider_signal.value, 2), q.insider_signal.tier) },
    {
        key: 'news_sentiment_7d',
        label: 'News sentiment (7d)',
        value: withBand(formatNumber(q.news_sentiment_7d.value, 2), q.news_sentiment_7d.tier),
    },
    {
        key: 'news_sentiment_30d',
        label: 'News sentiment (30d)',
        value: withBand(formatNumber(q.news_sentiment_30d.value, 2), q.news_sentiment_30d.tier),
    },
    {
        key: 'rd_intensity',
        label: 'R&D intensity',
        value: withBand(formatPercent(q.rd_intensity.value, 1), q.rd_intensity.tier, RD_INTENSITY_TIER_LABELS),
        sub: 'R&D as % of revenue',
    },
];

/** Qualitative signals — the stored readings with their tiers as plain-text bands. */
const QualitativeSection = ({ qualitative }: QualitativeSectionProps) => (
    <CollapsibleCard
        id="scanner-analysis-qualitative"
        persistKey="scanner.detail.qualitative"
        data-testid="analysis-qualitative"
        title="Qualitative signals"
    >
        <FactGrid cells={buildQualitativeCells(qualitative)} testIdPrefix="qualitative" />
    </CollapsibleCard>
);

export default QualitativeSection;
