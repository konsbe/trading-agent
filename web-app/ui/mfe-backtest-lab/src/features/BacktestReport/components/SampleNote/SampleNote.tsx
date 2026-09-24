import { SampleNoteProps } from './types';
import '@/styles/backtest-global.css';
import './SampleNote-styles.css';

/** The published sample behind a section, as muted secondary text. */
const SampleNote = ({ children, 'data-testid': testId }: SampleNoteProps) => (
    <p className="backtest-muted backtest-sample" data-testid={testId}>
        <span className="backtest-sample__label">Sample:</span> {children}
    </p>
);

export default SampleNote;
