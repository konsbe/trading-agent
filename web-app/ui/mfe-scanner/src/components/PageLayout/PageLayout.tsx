import { DisclaimerPill, PageHeader } from '@trading-agent/shared-components';
import { useIsHosted } from '@/providers/HostModeContext';
import { PageLayoutProps } from './types';
import '@/styles/scanner-global.css';
import './PageLayout-styles.css';

/**
 * Page frame for every scanner screen. With a title the header is the shared
 * PageHeader (back action, h1, subtitle, badges); without one (the shell
 * already titles the page) only the subtitle and actions show. The disclaimer
 * pill is rendered here only when standalone; hosted, spog's header carries it.
 */
const PageLayout = ({ title, subtitle, actions, back, badges, children }: PageLayoutProps) => {
    const isHosted = useIsHosted();
    const showPill = !isHosted;
    const actionsNode =
        actions || showPill ? (
            <>
                {actions}
                {showPill && <DisclaimerPill />}
            </>
        ) : undefined;

    return (
        <div className={`scanner-page${isHosted ? ' scanner-page--hosted' : ''}`} data-testid="scanner-page">
            {title ? (
                <PageHeader title={title} subtitle={subtitle} back={back} badges={badges} actions={actionsNode} />
            ) : (
                (subtitle || actionsNode) && (
                    <header className="scanner-page__header">
                        {subtitle && <div className="scanner-page__subtitle">{subtitle}</div>}
                        {actionsNode && <div className="scanner-page__actions">{actionsNode}</div>}
                    </header>
                )
            )}
            <div className="scanner-page__body">{children}</div>
        </div>
    );
};

export default PageLayout;
