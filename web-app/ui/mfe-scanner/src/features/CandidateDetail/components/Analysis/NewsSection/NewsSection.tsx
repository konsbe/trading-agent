import { CollapsibleCard } from '@trading-agent/shared-components';
import { NewsSectionProps } from './types';
import '@/styles/scanner-global.css';
import '../analysis-styles.css';
import './NewsSection-styles.css';

/** Section 5 — headlines as title + source + link only (no sentiment score), like every news list in the app. */
const NewsSection = ({ headlines }: NewsSectionProps) => (
    <CollapsibleCard id="scanner-analysis-news" persistKey="scanner.detail.news" data-testid="analysis-news" title="Sentiment & news">
        {headlines.length === 0 ? (
            <p className="scanner-muted scanner-analysis__note" data-testid="news-empty">
                No headlines stored for this symbol.
            </p>
        ) : (
            <ul className="scanner-news" data-testid="news">
                {headlines.map((item, i) => (
                    <li key={`${item.url ?? item.title}-${i}`} className="scanner-news__item" data-testid="news-item">
                        {item.url ? (
                            <a className="scanner-link" href={item.url} target="_blank" rel="noopener noreferrer" data-testid="news-link">
                                {item.title}
                            </a>
                        ) : (
                            <span>{item.title}</span>
                        )}
                        <span className="scanner-muted"> · {item.source}</span>
                    </li>
                ))}
            </ul>
        )}
    </CollapsibleCard>
);

export default NewsSection;
