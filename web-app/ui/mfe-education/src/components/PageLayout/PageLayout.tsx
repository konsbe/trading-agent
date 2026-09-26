import { DisclaimerPill } from '@trading-agent/shared-components';
import { useIsHosted } from '@/providers/HostModeContext';
import { PageLayoutProps } from './types';
import './PageLayout-styles.css';

/**
 * Page frame for every Education screen. The disclaimer pill is rendered here
 * only when standalone; hosted, spog's header carries it on every screen.
 */
const PageLayout = ({ title, children }: PageLayoutProps) => {
    const isHosted = useIsHosted();
    const showPill = !isHosted;
    const hasHeader = Boolean(title || showPill);

    return (
        <div className={`education-page${isHosted ? ' education-page--hosted' : ''}`} data-testid="education-page">
            {hasHeader && (
                <header className="education-page__header">
                    {title && <h1 className="education-page__title">{title}</h1>}
                    {showPill && (
                        <div className="education-page__actions">
                            <DisclaimerPill />
                        </div>
                    )}
                </header>
            )}
            <div className="education-page__body">{children}</div>
        </div>
    );
};

export default PageLayout;
