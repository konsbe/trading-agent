import { Link, useLocation } from 'react-router-dom';
import { DocLayoutProps, JumpNavLink } from './types';
import './DocLayout-styles.css';

const NavLink = ({ id, title, current }: JumpNavLink & { current: string }) => (
    <Link
        to={{ hash: `#${encodeURIComponent(id)}` }}
        className="education-doc__link"
        aria-current={current === id ? 'location' : undefined}
    >
        {title}
    </Link>
);

/**
 * A narrative page: a jump-nav of groups (Handbook sections / MasterClass
 * modules) → entries, beside the content. Links are real anchors on the current
 * route (`#<id>`), so they work with Tab/Enter, can be opened or copied, and
 * `useHashScroll` does the scrolling. Sticky beside the content on wide
 * screens; at the top of the page on narrow ones.
 */
const DocLayout = ({ navLabel, groups, children }: DocLayoutProps) => {
    const { hash } = useLocation();
    let current = '';
    try {
        current = decodeURIComponent(hash.replace(/^#/, ''));
    } catch {
        current = hash.replace(/^#/, '');
    }

    return (
        <div className="education-doc">
            <nav className="education-doc__nav" aria-label={navLabel} data-testid="jump-nav">
                <p className="education-doc__nav-title">{navLabel}</p>
                <ol className="education-doc__groups">
                    {groups.map(group => (
                        <li key={group.id} className="education-doc__group">
                            <NavLink id={group.id} title={group.title} current={current} />
                            {group.items.length > 0 && (
                                <ol className="education-doc__items">
                                    {group.items.map(item => (
                                        <li key={item.id}>
                                            <NavLink id={item.id} title={item.title} current={current} />
                                        </li>
                                    ))}
                                </ol>
                            )}
                        </li>
                    ))}
                </ol>
            </nav>
            <div className="education-doc__content">{children}</div>
        </div>
    );
};

export default DocLayout;
