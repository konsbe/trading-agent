const path = require("path");
const HtmlWebpackPlugin = require("html-webpack-plugin");
const ModuleFederationPlugin = require("webpack/lib/container/ModuleFederationPlugin");
const { DefinePlugin } = require("webpack");
const { createShellRemotePromise } = require("./src/common/webpackUtils/mfeUtils.ts");

const isCI = process.env.CI === "true" || process.env.JENKINS_HOME;

const SHARED_COMPONENTS_DIR = path.resolve(__dirname, "../../shared-components");
const DEFAULT_MOMENTUM_API_URL = "http://localhost:8090";

module.exports = (env, argv) => {
    const isProduction = argv.mode === "production";

    const MFE_PORT = 3002;
    const MFE_PUBLIC_PATH = isProduction ? "auto" : `http://localhost:${MFE_PORT}/`;
    const SPOG_DEV_REMOTE = "shell_spog@http://localhost:3000/remoteEntry.js";

    return {
        mode: argv.mode,
        cache: isCI ? false : {
            type: "filesystem",
            buildDependencies: { config: [__filename] },
        },
        parallelism: isCI ? 1 : 100,
        entry: "./src/index.tsx",
        output: {
            path: path.resolve(__dirname, "dist"),
            filename: "bundle.js",
            publicPath: MFE_PUBLIC_PATH,
            clean: true,
        },
        module: {
            rules: [
                {
                    test: /\.(js|jsx|ts|tsx)$/,
                    exclude: /node_modules/,
                    use: { loader: "babel-loader" },
                },
                {
                    test: /\.module\.css$/,
                    use: [
                        "style-loader",
                        {
                            loader: "css-loader",
                            options: {
                                modules: {
                                    mode: "local",
                                    localIdentName: "[name]__[local]--[hash:base64:5]",
                                    namedExport: false,
                                },
                                sourceMap: true,
                            },
                        },
                    ],
                },
                {
                    test: /\.css$/,
                    exclude: /\.module\.css$/,
                    use: [
                        "style-loader",
                        {
                            loader: "css-loader",
                            options: { modules: false, importLoaders: 1, sourceMap: true },
                        },
                    ],
                },
                {
                    test: /\.(gif|webp)$/i,
                    type: "asset/resource",
                },
                {
                    test: /\.(woff|woff2|eot|ttf|otf)$/i,
                    type: "asset/resource",
                    generator: { filename: "fonts/[hash]-[name].[ext]" },
                },
                {
                    test: /\.(png|jpe?g|svg)$/i,
                    use: [{
                        loader: "url-loader",
                        options: { limit: 8000, name: "images/[hash]-[name].[ext]" },
                    }],
                },
            ],
        },
        resolve: {
            extensions: [".webpack.js", ".web.js", ".ts", ".tsx", ".js", ".jsx"],
            // shared-components is compiled from source; resolve its bare imports
            // (react, react-dom, ...) from this package so only one React copy is bundled.
            modules: [path.resolve(__dirname, "node_modules"), "node_modules"],
            alias: {
                "@": path.resolve(__dirname, "./src"),
                "@trading-agent/shared-components/theme.css": path.resolve(SHARED_COMPONENTS_DIR, "src/theme/theme.css"),
                "@trading-agent/shared-components$": path.resolve(SHARED_COMPONENTS_DIR, "src/mfe.ts"),
            },
        },
        plugins: [
            new HtmlWebpackPlugin({ template: "./public/index.html" }),
            new ModuleFederationPlugin({
                // Must equal `mfes.mfe_watchlist.scope` in web-app/spog/public/config.json.
                name: "mfe_watchlist",
                filename: "remoteEntry.js",
                exposes: {
                    "./Watchlist": "./src/app/app-root",
                },
                remotes: {
                    // `shellSpog` is the request prefix used by useAuthMFE in shared-components.
                    shellSpog: isProduction ? createShellRemotePromise() : SPOG_DEV_REMOTE,
                },
                shared: {
                    react: { singleton: true, requiredVersion: ">=19.1.1", eager: true },
                    "react-dom": { singleton: true, requiredVersion: ">=19.1.1", eager: true },
                    "react-router-dom": { singleton: true, requiredVersion: ">=7.0.0", eager: true },
                },
            }),
            new DefinePlugin({
                "process.env.NODE_ENV": JSON.stringify(argv.mode),
                "process.env.isProduction": JSON.stringify(isProduction),
                "process.env.PUBLIC_PATH": JSON.stringify(MFE_PUBLIC_PATH),
                "process.env.MOMENTUM_API_URL": JSON.stringify(
                    process.env.MOMENTUM_API_URL || DEFAULT_MOMENTUM_API_URL
                ),
            }),
        ],
        devServer: {
            static: { directory: path.join(__dirname, "public") },
            compress: true,
            port: MFE_PORT,
            hot: true,
            historyApiFallback: true,
            // spog (:3000) fetches remoteEntry.js and chunks from this origin.
            headers: { "Access-Control-Allow-Origin": "*" },
        },
    };
};
