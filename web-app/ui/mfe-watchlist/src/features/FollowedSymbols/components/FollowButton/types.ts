export interface FollowButtonProps {
    symbol: string;
    isFollowed: boolean;
    isSaving: boolean;
    onFollow: (symbol: string) => void;
}
