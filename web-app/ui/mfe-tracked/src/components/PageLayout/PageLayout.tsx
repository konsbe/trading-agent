import { DisclaimerPill } from '@trading-agent/shared-components';
import { useIsHosted } from '@/providers/HostModeContext';
import { PageLayoutProps } from './types';
import './PageLayout-styles.css';

/**
 * Page frame for the tracked positions screen. The disclaimer pill is rendered here
 * only when standalone; hosted, spog's header carries it on every screen.
 */
const PageLayout = ({ title, subtitle, actions, backLink, children }: PageLayoutProps) => {
    const isHosted = useIsHosted();
    const showPill = !isHosted;
    const hasHeader = Boolean(title || subtitle || actions || showPill);

    return (
        <div className={`tracked-page${isHosted ? ' tracked-page--hosted' : ''}`} data-testid="tracked-page">
            {backLink && <nav className="tracked-page__back" aria-label="Breadcrumb">{backLink}</nav>}
            {hasHeader && (
                <header className="tracked-page__header">
                    <div className="tracked-page__heading">
                        {title && <h1 className="tracked-page__title">{title}</h1>}
                        {subtitle && <div className="tracked-page__subtitle">{subtitle}</div>}
                    </div>
                    <div className="tracked-page__actions">
                        {actions}
                        {showPill && <DisclaimerPill />}
                    </div>
                </header>
            )}
            <div className="tracked-page__body">{children}</div>
        </div>
    );
};

export default PageLayout;
