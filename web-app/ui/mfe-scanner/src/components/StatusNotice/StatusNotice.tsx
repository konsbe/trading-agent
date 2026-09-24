import { StatusNoticeProps } from './types';
import './StatusNotice-styles.css';

/** Neutral, calm notice for expected non-data states (scan pending, symbol not found, empty). */
const StatusNotice = ({ title, children, action, role = 'status', 'data-testid': testId }: StatusNoticeProps) => (
    <div className="scanner-notice" role={role} data-testid={testId}>
        <p className="scanner-notice__title">{title}</p>
        {children && <div className="scanner-notice__body">{children}</div>}
        {action && <div className="scanner-notice__action">{action}</div>}
    </div>
);

export default StatusNotice;
