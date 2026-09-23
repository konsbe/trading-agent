const path = require("path");
const HtmlWebpackPlugin = require("html-webpack-plugin");
const ModuleFederationPlugin = require('webpack/lib/container/ModuleFederationPlugin');
const { DefinePlugin } = require('webpack');
require('dotenv').config();

module.exports = (env, argv) => {
    const isProduction = argv.mode === "production"; // We need this later
    const isTestEnviroment = env.TEST_ENV === 'playwright';
    const SHELL_PUBLIC_PATH = isProduction ? "auto" : ("/");

    return {
        mode: argv.mode, // Use 'production' for builds
        entry: "./src/index.tsx",
        output: {
            path: path.resolve(__dirname, "dist"),
            filename: "bundle.js",
            //TODO: remove prod variable in integration
            publicPath: SHELL_PUBLIC_PATH, // Important for routing and dev server
        },
        module: {
            rules: [
                {
                    test: /\.(js|jsx|ts|tsx)$/,
                    exclude: /node_modules/,
                    use: {
                        loader: "babel-loader",
                    },
                },
                // CSS Modules
                {
                    test: /\.module\.css$/,
                    use: [
                        "style-loader",
                        {
                            loader: "css-loader",
                            options: {
                                // Enable CSS Modules processing
                                modules: {
                                    mode: "local",
                                    localIdentName:
                                        "[name]__[local]--[hash:base64:5]",
                                    namedExport: false, // Explicitly disable named-export-only mode
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
                            options: {
                                modules: false,
                                importLoaders: 1,
                                sourceMap: true,
                            },
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
                    generator: {
                        filename: 'fonts/[hash]-[name].[ext]' // Optional: organize fonts into a 'fonts' folder
                    }
                },
                {
                    test: /\.(png|jpe?g|svg)$/i,
                    use: [{
                        loader: 'url-loader',
                        options: {
                            limit: 8000, // Convert images < 8kb to base64 strings
                            name: 'images/[hash]-[name].[ext]'
                        }
                    }],
                },
            ],
        },
        resolve: {
            extensions: [".tsx", ".ts", ".js", ".jsx"],
            // Shared-components sources are compiled in place; resolve their bare imports
            // (react, react-dom, ...) from this package so only one React copy is bundled.
            modules: [path.resolve(__dirname, "node_modules"), "node_modules"],
            alias: {
                "@components": path.resolve(__dirname, "./src/components"),
                "@common": path.resolve(__dirname, "./src/common"),
                "@styles": path.resolve(__dirname, "./src/styles"),
                "@constants": path.resolve(__dirname, "./src/constants"),
                "@layouts": path.resolve(__dirname, "./src/layouts"),
                "@trading-agent/shared-components/theme.css": path.resolve(__dirname, "../shared-components/src/theme/theme.css"),
                // Shell must not load dist/index.js (barrel pulls useAuthMFE → shellSpog/*).
                "@trading-agent/shared-components$": path.resolve(__dirname, "../shared-components/src/shell.ts"),
            },
        },
        plugins: [
            new HtmlWebpackPlugin({
                template: "./public/index.html",
                inject: false, // bundle.js URL comes from config.json at runtime
            }),
            new ModuleFederationPlugin({
                name: 'shell_spog',
                filename: 'remoteEntry.js',
                shared: {
                    react: {
                        singleton: true,
                        requiredVersion: ">=19.1.1",
                        eager: true,
                    },
                    "react-dom": {
                        singleton: true,
                        requiredVersion: ">=19.1.1",
                        eager: true,
                    },
                    "react-router-dom": {
                        singleton: true,
                        requiredVersion: ">=7.0.0",
                        eager: true,
                    },
                    '@reduxjs/toolkit': {
                        singleton: true,
                        requiredVersion: '^2.0.0',
                        eager: true,
                    },
                    'react-redux': {
                        singleton: true,
                        requiredVersion: '>=9.2.0',
                        eager: true,
                    },
                    'react-dnd': {
                        singleton: true,
                        requiredVersion: '>=11.1.3',
                        eager: true,
                    },
                    'react-dnd-html5-backend': {
                        singleton: true,
                        requiredVersion: '>=11.1.3',
                        eager: true,
                    },
                    'ag-grid-community': {
                        singleton: true,
                        requiredVersion: '>=35.0.1',
                        eager: true,
                    },
                    'ag-grid-react': {
                        singleton: true,
                        requiredVersion: '>=35.0.1',
                        eager: true,
                    }

                },
                exposes: {
                    './userDataMessageService': './src/common/message_service/services/MfeServices/UserData',
                    './filterDataMessageService': './src/common/message_service/services/MfeServices/FilterData',
                },
                // No static remotes - all MFEs loaded dynamically at runtime from config.json
                remotes: {},
            }),
            new DefinePlugin({
                'process.env.NODE_ENV': JSON.stringify(argv.mode),
                'process.env.IS_PROD_ENV': isProduction,
                'process.env.IS_TEST_ENV': isTestEnviroment,
                'process.env.IS_JEST_ENV': false,
                'process.env.PUBLIC_PATH': SHELL_PUBLIC_PATH,
            }),
            // Note: In dev mode, config.json is served from public/ via devServer.static
            // For production builds, you'll need to handle config.json deployment separately
        ],
        
        devServer: {
            static: path.resolve(__dirname, "public"),
            compress: true,
            port: 3000,
            open: false, // Automatically open the browser
            hot: true, // Enable Hot Module Replacement
            historyApiFallback: true,
            headers: {
                "Access-Control-Allow-Origin": "*",
                "Access-Control-Allow-Methods": "GET, POST, PUT, DELETE, PATCH, OPTIONS",
                "Access-Control-Allow-Headers": "X-Requested-With, content-type, Authorization"
            },
        },
    };
};
