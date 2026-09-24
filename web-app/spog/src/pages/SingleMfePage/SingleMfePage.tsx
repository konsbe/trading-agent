import { lazy, useMemo } from 'react';
import { ContentWrapper } from '@trading-agent/shared-components';
import HeaderComponent from '@components/HeaderComponent/HeaderComponent';
import GridLayout from '../../layouts/GridLayout';
import { Navigate } from 'react-router-dom';
import { isMfeEnabled, loadMfeComponent } from '../../common/dynamic_load';
import { useMfeReloadToken } from '../../providers/SseMetadataProvider/SseMetadataProvider';

const SingleMfePage = ({
    mfe_key,
    mfe_component,
    mfe_header_title,
    mfe_header_icon,
    mfe_navigation_path,
    mfe_enable_navigation,
    header_scrollable = false,
    content_wrapper_scrollable = false,
    noPadding = true,
}: {
    mfe_key: string,
    mfe_component: string,
    mfe_header_title: string,
    mfe_header_icon?: React.ComponentType<any>,
    mfe_navigation_path: string,
    mfe_enable_navigation: boolean,
    header_scrollable?: boolean,
    content_wrapper_scrollable?: boolean,
    noPadding?: boolean,
}) => {

    const reloadToken = useMfeReloadToken(mfe_key);

    const MFE_ENABLED = isMfeEnabled(mfe_key);

    const RemoteMfeComponent = useMemo(
        () =>
            lazy(() =>
                loadMfeComponent(mfe_key, mfe_component).then((Component) => ({
                    default: Component,
                }))
            ),
        // mfe_key/mfe_component: recreate when navigating to a different MFE route
        // reloadToken: recreate when SSE signals an MFE update
        // eslint-disable-next-line react-hooks/exhaustive-deps
        [mfe_key, mfe_component, reloadToken]
    );


    if (!MFE_ENABLED) {
        return <Navigate to="/404" replace />;
    }

    const layout = [
        {
            columns: [{
                grid: { xs: 24, sm: 24, md: 24, lg: 24, xl: 24 },
                layout: [
                    // row
                    {
                        columns: [{
                            grid: { xs: 24, sm: 24, md: 24, lg: 24, xl: 24 },
                            components: [{
                                component: (
                                    <HeaderComponent
                                        title={mfe_header_title}
                                        icon={mfe_header_icon}
                                        enableNavigation={mfe_enable_navigation}
                                        navigationPath={mfe_navigation_path}
                                    />
                                ),
                                id: `header-component-${mfe_key}`,
                                style: { height: 'auto' },
                                scrollable: header_scrollable,
                            }],
                        }],
                        style: { maxHeight: '44px' },
                    },
                    // row
                    {
                        columns: [{
                            grid: { xs: 24, sm: 24, md: 24, lg: 24, xl: 24 },
                            components: [{
                                component: (
                                    <ContentWrapper
                                        id={mfe_key}
                                        type="remote"
                                        component={RemoteMfeComponent}
                                        componentProps={{}}
                                        mfeKey={mfe_key}
                                    />
                                ),
                                id: mfe_key,
                                style: { height: '100%' },
                                scrollable: content_wrapper_scrollable,
                                noPadding: noPadding,
                            }],
                        }],
                    },
                ],
            }],
            style: { height: '100%' },
        },
    ];

    return (
        <GridLayout
            layout={layout}
            spacing={1}
            showTitles={false}
        />
    );
}

export default SingleMfePage;