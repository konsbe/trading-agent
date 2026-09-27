import { Button } from '@trading-agent/shared-components';
import { FollowButtonProps } from './types';

/** Follow from a search result; already-followed symbols read "Following" and can't be pressed. */
const FollowButton = ({ symbol, isFollowed, isSaving, onFollow }: FollowButtonProps) => (
    <Button
        variant={isFollowed ? 'ghost' : 'primary'}
        size="sm"
        disabled={isFollowed || isSaving}
        aria-busy={isSaving}
        aria-label={isFollowed ? `Following ${symbol}` : `Follow ${symbol}`}
        onClick={() => onFollow(symbol)}
        data-testid={`follow-${symbol}`}
    >
        {isSaving ? 'Following…' : isFollowed ? 'Following' : 'Follow'}
    </Button>
);

export default FollowButton;
