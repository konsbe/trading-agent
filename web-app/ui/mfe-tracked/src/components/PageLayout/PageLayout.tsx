import { DisclaimerPill, PageFrame } from '@trading-agent/shared-components';
import { useIsHosted } from '@/providers/HostModeContext';
import { PageLayoutProps } from './types';
import './PageLayout-styles.css';

/**
 * Page frame for the tracked positions screen: the shared header bar (title, live count,
 * description) above the body, the page's only scroll container. spog renders
 * no header row of its own for this route (config `shell_header: false`). The
 * disclaimer pill is rendered here only when standalone; hosted, spog's top
 * bar carries it on every screen.
 */
const PageLayout = ({ title, subtitle, actions, children }: PageLayoutProps) => {
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
            actions={actionsNode}
            className={`tracked-page${isHosted ? ' tracked-page--hosted' : ''}`}
            bodyClassName="tracked-page__body"
            data-testid="tracked-page"
        >
            {children}
        </PageFrame>
    );
};

export default PageLayout;
