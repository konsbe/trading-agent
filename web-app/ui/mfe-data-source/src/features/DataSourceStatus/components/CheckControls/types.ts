export interface LastCheckedProps {
    /** RFC 3339. */
    checkedAt: string;
}

export interface RefreshButtonProps {
    onRefresh: () => void;
    /** A request is in flight: the button is disabled and reads "Checking…". */
    isChecking: boolean;
}
