import { TagProps } from './types';
import './Tag-styles.css';

/** A small label chip: `neutral` for facts (chip fill), `accent` for the one worth noticing (tinted pill). */
const Tag = ({ children, tone = 'neutral', title, 'data-testid': testId }: TagProps) => (
    <span className={`watchlist-tag is-${tone}`} title={title} data-testid={testId}>
        {children}
    </span>
);

export default Tag;
