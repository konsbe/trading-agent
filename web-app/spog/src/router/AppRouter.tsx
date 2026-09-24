import React from "react";
import { createBrowserRouter, Navigate, RouteObject } from "react-router-dom";

import Layout from "../layouts/AppLayout/Layout";
import PageNotFound from "../pages/PageNotFound/PageNotFound";
import SingleMfePage from "../pages/SingleMfePage";
import UserAccessControl from "../components/UserAccessControl/UserAccessControl";
import { DEFAULT_ROUTE } from "../constants/routes";

const PrivateRouter = ({ roles = [], component }: { roles?: string[], component: React.ReactNode }) => (
    <UserAccessControl roles={roles}>
        {component}
    </UserAccessControl>
);

export const routes: RouteObject[] = [
    {
        path: "/",
        element: <Layout />,
        children: [
            {
                index: true,
                element: <Navigate to={DEFAULT_ROUTE} replace />,
            },
            {
                path: "candidates/*",
                element: (
                    <PrivateRouter
                        component={
                            <SingleMfePage
                                mfe_key="mfe_scanner"
                                mfe_component="./Scanner"
                                mfe_header_title="Today's Candidates"
                                mfe_navigation_path="/candidates"
                                mfe_enable_navigation={false}
                            />
                        }
                    />
                ),
            },
            {
                path: "stock-detail/*",
                element: <PrivateRouter component={<>Stock Detail</>} />,
            },
            {
                path: "backtest-lab/*",
                element: (
                    <PrivateRouter
                        component={
                            <SingleMfePage
                                mfe_key="mfe_backtest_lab"
                                mfe_component="./BacktestLab"
                                mfe_header_title="Backtest Lab"
                                mfe_navigation_path="/backtest-lab"
                                mfe_enable_navigation={false}
                            />
                        }
                    />
                ),
            },
            {
                path: "alarm-history/*",
                element: <PrivateRouter component={<>Alarm History</>} />,
            },
            {
                path: "watchlist/*",
                element: (
                    <PrivateRouter
                        component={
                            <SingleMfePage
                                mfe_key="mfe_watchlist"
                                mfe_component="./Watchlist"
                                mfe_header_title="Watchlist"
                                mfe_navigation_path="/watchlist"
                                mfe_enable_navigation={false}
                            />
                        }
                    />
                ),
            },
            {
                path: "tracked-positions/*",
                element: <PrivateRouter component={<>Tracked Positions</>} />,
            },
            {
                path: "data-source/*",
                element: (
                    <PrivateRouter
                        component={
                            <SingleMfePage
                                mfe_key="mfe_data_source"
                                mfe_component="./DataSource"
                                mfe_header_title="Data Source"
                                mfe_navigation_path="/data-source"
                                mfe_enable_navigation={false}
                            />
                        }
                    />
                ),
            },
            {
                path: "settings/*",
                element: <PrivateRouter component={<>Settings</>} />,
            },
            {
                path: "/404",
                element: <PageNotFound statusCode={404} message={'Page Not Found'} />,
            },
            {
                path: "/unauthorized",
                element: <PageNotFound statusCode={401} message={'Unauthorized'} />,
            },
            {
                path: "*",
                element: <Navigate to="/404" replace />,
            },
        ],
    },
];

const router = createBrowserRouter(routes);

export default router;
