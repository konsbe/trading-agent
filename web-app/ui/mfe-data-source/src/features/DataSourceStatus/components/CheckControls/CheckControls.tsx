import { Button } from '@trading-agent/shared-components';
import { formatCheckedAt } from '../../utils/format';
import { LastCheckedProps, RefreshButtonProps } from './types';
import '@/styles/data-source-global.css';

/** "Last checked: …" in the user's local time — the age of the answer, not a live clock. */
export const LastChecked = ({ checkedAt }: LastCheckedProps) => (
    <span data-testid="last-checked">
        Last checked: <time dateTime={checkedAt}>{formatCheckedAt(checkedAt)}</time>
    </span>
);

/** The page's only action: re-read the status once. No auto-refresh. */
export const RefreshButton = ({ onRefresh, isChecking }: RefreshButtonProps) => (
    <Button variant="secondary" size="sm" onClick={onRefresh} disabled={isChecking} aria-busy={isChecking} data-testid="refresh-button">
        {isChecking ? 'Checking…' : 'Refresh'}
    </Button>
);
