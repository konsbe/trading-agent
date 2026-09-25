import { CollapsibleCard } from '@trading-agent/shared-components';
import { bandLabel, clusterLine, labelText } from '../../../utils/analysisFormat';
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
 * signal, and the aligned (cluster positives) / divergent (cluster warnings)
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
            <CompositeLine composite={correlations.composite} data-testid="correlations-composite" />

            <div className="scanner-analysis__group">
                <h3 className="scanner-analysis__group-title">Cluster health</h3>
                <SentenceList items={clusters.map(c => clusterLine(c.name, c.tier))} empty="No correlation clusters stored." testId="correlation-clusters" />
            </div>

            <div className="scanner-analysis__group">
                <h3 className="scanner-analysis__group-title">Master signal</h3>
                <p className="scanner-analysis__note" data-testid="master-signal">
                    Net signal: {labelText(master.net_signal)}
                    {master.fired.length > 0 && ` · fired: ${master.fired.map(name => bandLabel(name)).join(', ')}`}
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
