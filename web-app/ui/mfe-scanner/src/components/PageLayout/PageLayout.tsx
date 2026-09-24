import { DisclaimerPill } from '@trading-agent/shared-components';
import { useIsHosted } from '@/providers/HostModeContext';
import { PageLayoutProps } from './types';
import '@/styles/scanner-global.css';
import './PageLayout-styles.css';

/**
 * Page frame for every scanner screen. The disclaimer pill is rendered here
 * only when standalone; hosted, spog's header carries it on every screen.
 */
const PageLayout = ({ title, subtitle, actions, backLink, children }: PageLayoutProps) => {
    const isHosted = useIsHosted();
    const showPill = !isHosted;
    const hasHeader = Boolean(title || subtitle || actions || showPill);

    return (
        <div className={`scanner-page${isHosted ? ' scanner-page--hosted' : ''}`} data-testid="scanner-page">
            {backLink && <nav className="scanner-page__back" aria-label="Breadcrumb">{backLink}</nav>}
            {hasHeader && (
                <header className="scanner-page__header">
                    <div className="scanner-page__heading">
                        {title && <h1 className="scanner-page__title">{title}</h1>}
                        {subtitle && <div className="scanner-page__subtitle">{subtitle}</div>}
                    </div>
                    <div className="scanner-page__actions">
                        {actions}
                        {showPill && <DisclaimerPill />}
                    </div>
                </header>
            )}
            <div className="scanner-page__body">{children}</div>
        </div>
    );
};

export default PageLayout;
