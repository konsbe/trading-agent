import { formatTradingDay } from '@/common/format/format';
import { isNotYetEvaluated } from '../../utils/rows';
import { SessionsCellProps } from './types';
import './SessionsCell-styles.css';

export const NOT_YET_EVALUATED = 'Not yet evaluated';

/**
 * Trading sessions the exit rules have evaluated. A row never evaluated reads
 * "Not yet evaluated" (the API's stored 0 is not a count); a row the tracker has
 * not evaluated through the latest scan says what date its count is as of.
 */
const SessionsCell = ({ row }: SessionsCellProps) => {
    if (isNotYetEvaluated(row)) {
        return (
            <span className="tracked-sessions__pending" data-testid="sessions-value">
                {NOT_YET_EVALUATED}
            </span>
        );
    }

    const asOf = formatTradingDay(row.last_evaluated_date);
    return (
        <span className="tracked-sessions">
            <span className="tracked-sessions__value" data-testid="sessions-value">
                {row.sessions_elapsed}
            </span>
            {row.evaluation_behind && (
                <span
                    className="tracked-sessions__hint"
                    title={`Sessions elapsed as of ${asOf}, the last session the tracker evaluated this row`}
                    data-testid="evaluation-behind"
                >
                    as of {asOf}
                </span>
            )}
        </span>
    );
};

export default SessionsCell;
