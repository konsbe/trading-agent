import React from "react";
import { createBrowserRouter, Navigate, RouteObject } from "react-router-dom";

import Layout from "../layouts/AppLayout/Layout";
import PageNotFound from "../pages/PageNotFound/PageNotFound";
import SingleMfePage from "../pages/SingleMfePage";
import UserAccessControl from "../components/UserAccessControl/UserAccessControl";
import { getDefaultRoute, getMfeRoutes, getPlaceholderRoutes } from "@common/navigation";

type AppRouter = ReturnType<typeof createBrowserRouter>;

const PrivateRouter = ({ roles = [], component }: { roles?: string[], component: React.ReactNode }) => (
    <UserAccessControl roles={roles}>
        {component}
    </UserAccessControl>
);

const toChildPath = (path: string): string => `${path.replace(/^\/+/, "")}/*`;

/** Route tree for the shell, generated from `config.mfes` plus the code-defined placeholders. */
export const buildRoutes = (config?: AppConfig): RouteObject[] => {
    const mfeRoutes: RouteObject[] = getMfeRoutes(config).map(mfe => ({
        path: toChildPath(mfe.path),
        element: (
            <PrivateRouter
                roles={mfe.roles}
                component={
                    <SingleMfePage
                        mfe_key={mfe.mfeKey}
                        mfe_component={mfe.module}
                        mfe_header_title={mfe.label}
                        mfe_navigation_path={mfe.path}
                        mfe_enable_navigation={false}
                    />
                }
            />
        ),
    }));

    const placeholderRoutes: RouteObject[] = getPlaceholderRoutes(config).map(({ path, label }) => ({
        path: toChildPath(path),
        element: <PrivateRouter component={<>{label}</>} />,
    }));

    return [
        {
            path: "/",
            element: <Layout />,
            children: [
                {
                    index: true,
                    element: <Navigate to={getDefaultRoute(config)} replace />,
                },
                ...mfeRoutes,
                ...placeholderRoutes,
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
};

/** Creates a browser router from the config loaded into `window.__APP_CONFIG__` at call time. */
export const createAppRouter = (): AppRouter => createBrowserRouter(buildRoutes(window.__APP_CONFIG__));

let appRouter: AppRouter | undefined;

/** The shell's single router instance, created on first use (after config.json has loaded). */
export const getAppRouter = (): AppRouter => {
    appRouter ??= createAppRouter();
    return appRouter;
};
