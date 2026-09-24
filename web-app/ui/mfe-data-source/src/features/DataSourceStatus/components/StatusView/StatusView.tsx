import DailyChainSection from '../DailyChainSection';
import OverallStatus from '../OverallStatus';
import ProvidersSection from '../ProvidersSection';
import { StatusViewProps } from './types';
import './StatusView-styles.css';

/** A loaded status: the overall line (always visible), then providers and the daily chain. */
const StatusView = ({ status }: StatusViewProps) => (
    <div className="data-source-status" data-testid="status-view" data-checked-at={status.checked_at}>
        <OverallStatus overall={status.overall} reasons={status.overall_reasons} />
        <ProvidersSection providers={status.providers} reasons={status.overall_reasons} />
        <DailyChainSection chain={status.daily_chain} />
    </div>
);

export default StatusView;
