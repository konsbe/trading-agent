import { ComputedSymbol, ComputeState } from '@/api';

/**
 * Sort order of a computation state, ascending: what needs attention first
 * (failed, data never arrived), then what is moving, then what is settled.
 * Unknown server states rank after the known ones.
 */
export const COMPUTE_STATE_ORDER: readonly ComputeState[] = [
    'failed',
    'data_not_arrived',
    'waiting_for_data',
    'computing',
    'scheduled',
    'computed',
];

/** The state's headline as ComputeState renders it. */
export const COMPUTE_STATE_TEXT: Record<ComputeState, string> = {
    failed: 'Computation failed',
    data_not_arrived: "Data hasn't arrived",
    waiting_for_data: 'Waiting for data',
    computing: 'Computing analysis',
    scheduled: 'Scheduled',
    computed: 'Computed',
};

export const computeStateRank = (item: Pick<ComputedSymbol, 'state'> | undefined): number | null => {
    if (!item) return null;
    const rank = COMPUTE_STATE_ORDER.indexOf(item.state);
    return rank === -1 ? COMPUTE_STATE_ORDER.length : rank;
};

export const computeStateText = (item: Pick<ComputedSymbol, 'state'> | undefined): string =>
    item ? (COMPUTE_STATE_TEXT as Record<string, string>)[item.state] ?? item.state : '';
