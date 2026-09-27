import { CollapsibleCard } from '@trading-agent/shared-components';
import { scoreWithText, servedClusterLine, servedMasterSignalText } from '../../../utils/analysisFormat';
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
 * Section 4 — composite tier, each cluster's health, the master signal (net
 * count and patterns met) and the aligned (cluster positives) / divergent
 * (cluster warnings) lines. Tier, cluster, pattern and heading text is the
 * API's served labels, shown verbatim.
 */
const CorrelationsSection = ({ correlations }: CorrelationsSectionProps) => {
    const { clusters, aligned_signals: aligned, master_signals: master, labels } = correlations;
    const divergent = clusters.flatMap(cluster => cluster.warnings);

    return (
        <CollapsibleCard
            id="scanner-analysis-correlations"
            persistKey="scanner.detail.correlations"
            data-testid="analysis-correlations"
            title="Correlations"
        >
            <CompositeLine
                composite={correlations.composite}
                valueText={scoreWithText(correlations.composite.score, correlations.composite_label)}
                data-testid="correlations-composite"
            />

            <div className="scanner-analysis__group">
                <h3 className="scanner-analysis__group-title">Cluster health</h3>
                <SentenceList items={clusters.map(servedClusterLine)} empty="No correlation clusters stored." testId="correlation-clusters" />
            </div>

            <div className="scanner-analysis__group">
                {labels.patterns_heading && <h3 className="scanner-analysis__group-title">{labels.patterns_heading}</h3>}
                <p className="scanner-analysis__note" data-testid="master-signal">
                    {servedMasterSignalText(master, labels)}
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
