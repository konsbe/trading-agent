import { CollapsibleCard } from '@trading-agent/shared-components';
import InstrumentCard from '../InstrumentCard';
import { InstrumentGroupsProps } from './types';
import '@/styles/market-report-global.css';
import './InstrumentGroups-styles.css';

/** Section 2 — the fixed list, then the live watchlist, each its own collapsible group. */
const InstrumentGroups = ({ instruments }: InstrumentGroupsProps) => {
    const tracked = instruments.filter(i => i.source === 'fixed_list');
    const watchlist = instruments.filter(i => i.source === 'watchlist');

    return (
        <>
            <CollapsibleCard id="report-tracked" persistKey="report.tracked" title="Tracked" data-testid="group-tracked">
                <div className="market-report-instruments">
                    {tracked.map(inst => (
                        <InstrumentCard key={inst.key} instrument={inst} />
                    ))}
                </div>
            </CollapsibleCard>
            <CollapsibleCard id="report-watchlist" persistKey="report.watchlist" title="Watchlist" data-testid="group-watchlist">
                {watchlist.length === 0 ? (
                    <p className="market-report-muted" data-testid="watchlist-empty">
                        No symbols on your watchlist.
                    </p>
                ) : (
                    <div className="market-report-instruments">
                        {watchlist.map(inst => (
                            <InstrumentCard key={inst.key} instrument={inst} />
                        ))}
                    </div>
                )}
            </CollapsibleCard>
        </>
    );
};

export default InstrumentGroups;
