import { CollapsibleCard } from '@trading-agent/shared-components';
import { EMPTY_VALUE } from '@/common/format/format';
import { clusterLine, COMBINED_PATTERN_LABELS, CORRELATION_TIER_LABELS, displayBand, NET_SIGNAL_LABELS } from '../../../utils/analysisFormat';
import CompositeLine from '../CompositeLine';
import { CorrelationsSectionProps } from './types';
import '@/styles/scanner-global.css';
import '../analysis-styles.css';

const SentenceList = ({ items, empty, testId }: { items: string[]; empty: string; testId: string }) =>
    items.length === 0 ? (
        <p className="scanner-muted scanner-analysis__note" data-testid={`${testId}-empty`}>
            {empty}
        </p>
    ) : (
        <ul className="scanner-analysis__list" data-testid={testId}>
            {items.map((item, i) => (
                <li key={`${i}-${item}`}>{item}</li>
            ))}
        </ul>
    );

/**
 * Section 4 — composite tier, each cluster's health as plain text, the master
 * signal (as "Combined patterns": net count and patterns met), and the aligned (cluster positives) / divergent (cluster warnings)
 * lines as the stored sentences.
 */
const CorrelationsSection = ({ correlations }: CorrelationsSectionProps) => {
    const { clusters, aligned_signals: aligned, master_signals: master } = correlations;
    const divergent = clusters.flatMap(cluster => cluster.warnings);

    return (
        <CollapsibleCard
            id="scanner-analysis-correlations"
            persistKey="scanner.detail.correlations"
            data-testid="analysis-correlations"
            title="Correlations"
        >
            <CompositeLine composite={correlations.composite} labels={CORRELATION_TIER_LABELS} data-testid="correlations-composite" />

            <div className="scanner-analysis__group">
                <h3 className="scanner-analysis__group-title">Cluster health</h3>
                <SentenceList
                    items={clusters.map(c => clusterLine(c.name, c.tier, CORRELATION_TIER_LABELS))}
                    empty="No correlation clusters stored."
                    testId="correlation-clusters"
                />
            </div>

            <div className="scanner-analysis__group">
                <h3 className="scanner-analysis__group-title">Combined patterns</h3>
                <p className="scanner-analysis__note" data-testid="master-signal">
                    Net count: {displayBand(master.net_signal, NET_SIGNAL_LABELS) ?? EMPTY_VALUE}
                    {master.fired.length > 0 && ` · met: ${master.fired.map(name => displayBand(name, COMBINED_PATTERN_LABELS)).join(', ')}`}
                </p>
            </div>

            <div className="scanner-analysis__group">
                <h3 className="scanner-analysis__group-title">Aligned signals</h3>
                <SentenceList items={aligned} empty="No aligned signals stored." testId="aligned-signals" />
            </div>

            <div className="scanner-analysis__group">
                <h3 className="scanner-analysis__group-title">Divergent signals</h3>
                <SentenceList items={divergent} empty="No divergent signals stored." testId="divergent-signals" />
            </div>
        </CollapsibleCard>
    );
};

export default CorrelationsSection;
