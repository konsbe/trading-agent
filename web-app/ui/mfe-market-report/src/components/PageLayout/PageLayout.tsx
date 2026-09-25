import { DisclaimerPill } from '@trading-agent/shared-components';
import { useIsHosted } from '@/providers/HostModeContext';
import { PageLayoutProps } from './types';
import './PageLayout-styles.css';

/**
 * Page frame for every Daily Market Report screen. The disclaimer pill is rendered here
 * only when standalone; hosted, spog's header carries it on every screen.
 */
const PageLayout = ({ title, subtitle, actions, backLink, children }: PageLayoutProps) => {
    const isHosted = useIsHosted();
    const showPill = !isHosted;
    const hasHeader = Boolean(title || subtitle || actions || showPill);

    return (
        <div className={`market-report-page${isHosted ? ' market-report-page--hosted' : ''}`} data-testid="market-report-page">
            {backLink && <nav className="market-report-page__back" aria-label="Breadcrumb">{backLink}</nav>}
            {hasHeader && (
                <header className="market-report-page__header">
                    <div className="market-report-page__heading">
                        {title && <h1 className="market-report-page__title">{title}</h1>}
                        {subtitle && <div className="market-report-page__subtitle">{subtitle}</div>}
                    </div>
                    <div className="market-report-page__actions">
                        {actions}
                        {showPill && <DisclaimerPill />}
                    </div>
                </header>
            )}
            <div className="market-report-page__body">{children}</div>
        </div>
    );
};

export default PageLayout;
