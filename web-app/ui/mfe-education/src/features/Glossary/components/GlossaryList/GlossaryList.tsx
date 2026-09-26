import { Link } from 'react-router-dom';
import { entryLink, SOURCE_LABELS } from '../../utils/glossary';
import { GlossaryListProps } from './types';
import './GlossaryList-styles.css';

/** Flat list: term (with synonyms), one-line definition, and a link to the fuller entry. */
const GlossaryList = ({ terms }: GlossaryListProps) => (
    <ul className="education-glossary" aria-label="Glossary terms" data-testid="glossary-list">
        {terms.map(term => (
            <li key={`${term.source}:${term.term}`} className="education-glossary__row" data-testid="glossary-row">
                <div className="education-glossary__head">
                    <dfn className="education-glossary__term">{term.term}</dfn>
                    {term.synonyms.length > 0 && (
                        <span className="education-glossary__synonyms">Also: {term.synonyms.join(', ')}</span>
                    )}
                </div>
                <p className="education-glossary__definition">{term.definition}</p>
                <Link
                    to={entryLink(term)}
                    className="education-glossary__link"
                    aria-label={`Read “${term.term}” in the ${SOURCE_LABELS[term.source]}`}
                >
                    {SOURCE_LABELS[term.source]} →
                </Link>
            </li>
        ))}
    </ul>
);

export default GlossaryList;
