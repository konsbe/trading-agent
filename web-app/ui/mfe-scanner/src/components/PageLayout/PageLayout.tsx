import { DisclaimerPill, PageFrame } from '@trading-agent/shared-components';
import { useIsHosted } from '@/providers/HostModeContext';
import { PageLayoutProps } from './types';
import '@/styles/scanner-global.css';
import './PageLayout-styles.css';

/**
 * Page frame for every scanner screen: the shared header bar (back arrow,
 * h1, subtitle, badges, actions) above the body, the page's only scroll
 * container. spog renders no header row of its own for /candidates (config
 * `shell_header: false`). The disclaimer pill is rendered here only when
 * standalone; hosted, spog's top bar carries it.
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
        <PageFrame
            title={title}
            subtitle={subtitle}
            back={back}
            badges={badges}
            actions={actionsNode}
            className={`scanner-page${isHosted ? ' scanner-page--hosted' : ''}`}
            bodyClassName="scanner-page__body"
            data-testid="scanner-page"
        >
            {children}
        </PageFrame>
    );
};

export default PageLayout;
