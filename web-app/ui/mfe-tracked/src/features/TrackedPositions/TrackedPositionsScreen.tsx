import { getErrorMessage } from '@/common/errors/errorMessages';
import ApiErrorState from '@/components/ApiErrorState';
import PageLayout from '@/components/PageLayout';
import FreshnessBanner from './components/FreshnessBanner';
import TrackedPanel from './components/TrackedPanel';
import TrackedSkeleton from './components/TrackedSkeleton';
import TrackedTabs, { panelId, tabId } from './components/TrackedTabs';
import useOpenNotes from './hooks/useOpenNotes';
import useTrackedPositions from './hooks/useTrackedPositions';
import useTrackedTab from './hooks/useTrackedTab';
import './TrackedPositionsScreen-styles.css';

export const PAGE_INTRO =
    'Every scanner gate-pass alert, followed through the pre-registered exit rules. Research instrumentation — nothing here was traded.';

const ID_BASE = 'tracked';

/**
 * Tracked Positions: the chain's freshness banner, then Active / Closed tabs
 * over one `status=all` response. Descriptive only, like the rest of the app.
 */
const TrackedPositionsScreen = () => {
    const { positions, loadError, refreshError, isLoading, reload } = useTrackedPositions();
    const [tab, setTab] = useTrackedTab();
    const [openNotes, toggleNote] = useOpenNotes();

    return (
        <PageLayout title="Tracked Positions" subtitle={PAGE_INTRO}>
            {isLoading && !positions && <TrackedSkeleton />}

            {loadError && <ApiErrorState error={loadError} onRetry={reload} />}

            {positions && (
                <div className="tracked-screen ta-fit">
                    {refreshError && (
                        <p className="tracked-screen__refresh-error" role="status" data-testid="refresh-error">
                            Couldn&apos;t check for updates: {getErrorMessage(refreshError)} The rows below are from the last load.
                        </p>
                    )}
                    <FreshnessBanner chain={positions.chain} />
                    <TrackedTabs
                        idBase={ID_BASE}
                        selected={tab}
                        counts={{ active: positions.summary.active_count, closed: positions.summary.closed_count }}
                        onSelect={setTab}
                    />
                    <div className="ta-fit" role="tabpanel" id={panelId(ID_BASE, tab)} aria-labelledby={tabId(ID_BASE, tab)} data-testid={`panel-${tab}`}>
                        <TrackedPanel key={tab} variant={tab} rows={positions[tab]} openNotes={openNotes} onToggleNote={toggleNote} />
                    </div>
                </div>
            )}
        </PageLayout>
    );
};

export default TrackedPositionsScreen;
