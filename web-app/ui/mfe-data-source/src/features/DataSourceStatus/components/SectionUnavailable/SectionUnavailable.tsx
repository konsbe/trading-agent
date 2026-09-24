import { SectionUnavailableProps } from './types';
import '@/styles/data-source-global.css';

/** Inline notice for a section the server could not read on this check; the rest of the page still renders. */
const SectionUnavailable = ({ explanation, 'data-testid': testId }: SectionUnavailableProps) => (
    <div className="data-source-unavailable" data-testid={testId}>
        <p className="data-source-unavailable__title">Unavailable</p>
        <p className="data-source-muted">{explanation}</p>
    </div>
);

export default SectionUnavailable;
