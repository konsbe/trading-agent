import { DisclaimerPill } from '@trading-agent/shared-components';
import { useIsHosted } from '@/providers/HostModeContext';
import { PageLayoutProps } from './types';
import './PageLayout-styles.css';

/**
 * Page frame for every Backtest Lab screen. The disclaimer pill is rendered here
 * only when standalone; hosted, spog's header carries it on every screen.
 */
const PageLayout = ({ title, subtitle, actions, backLink, children }: PageLayoutProps) => {
    const isHosted = useIsHosted();
    const showPill = !isHosted;
    const hasHeader = Boolean(title || subtitle || actions || showPill);

    return (
        <div className={`backtest-lab-page${isHosted ? ' backtest-lab-page--hosted' : ''}`} data-testid="backtest-lab-page">
            {backLink && <nav className="backtest-lab-page__back" aria-label="Breadcrumb">{backLink}</nav>}
            {hasHeader && (
                <header className="backtest-lab-page__header">
                    <div className="backtest-lab-page__heading">
                        {title && <h1 className="backtest-lab-page__title">{title}</h1>}
                        {subtitle && <div className="backtest-lab-page__subtitle">{subtitle}</div>}
                    </div>
                    <div className="backtest-lab-page__actions">
                        {actions}
                        {showPill && <DisclaimerPill />}
                    </div>
                </header>
            )}
            <div className="backtest-lab-page__body">{children}</div>
        </div>
    );
};

export default PageLayout;
