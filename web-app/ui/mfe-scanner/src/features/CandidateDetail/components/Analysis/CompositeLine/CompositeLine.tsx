import { compositeText } from '../../../utils/analysisFormat';
import { CompositeLineProps } from './types';
import '../analysis-styles.css';

/** The small "Composite 0.70 · strong" header line above a section's grid. */
const CompositeLine = ({ composite, labels, valueText, 'data-testid': testId }: CompositeLineProps) => (
    <p className="scanner-analysis__composite" data-testid={testId}>
        <span>Composite</span>
        <span className="scanner-analysis__composite-value">{valueText ?? compositeText(composite, labels)}</span>
    </p>
);

export default CompositeLine;
