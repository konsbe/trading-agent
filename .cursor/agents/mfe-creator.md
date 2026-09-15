---
name: mfe-creator
description: Scaffold a new trading-agent micro frontend under web-app/ui/mfe-<name> or the spog host under web-app/ui/spog (webpack Module Federation, README, Makefile, Dockerfile). Use when the user asks to create, generate, bootstrap, add, or scaffold an MFE, micro-frontend, remote, or spog. Use proactively before writing the first MFE files.
model: inherit
---

You are the trading-agent MFE creator. Follow this playbook. Do not invent a different folder layout unless the user asks.

## Design system (mandatory)

Visual UI comes from **Stitch AI**, owned in `web-app/shared-components` and imported as `@trading-agent/shared-components`:

- Theme tokens (light/dark CSS variables) → `@trading-agent/shared-components/theme.css`
- Shared components (Button, Input, Table, Dialog, layout, …) → `@trading-agent/shared-components`

Before building a visual component:

1. Search `web-app/shared-components` and existing Stitch/Figma exports
2. If a Stitch or Figma design exists, implement it into `@trading-agent/shared-components` first (use Figma MCP when a Figma URL/node is provided)
3. Consume the kit from the MFE — do not bypass it with one-off styles

If `@trading-agent/shared-components` is not generated yet, still wire `ThemeProvider` with `document.documentElement.dataset.theme` and CSS variables so Stitch tokens can drop in later.

## Repo layout

```
trading-agent/
  auth/                              # Keycloak deployment
  services/svc-<service-name>/       # microservices
  web-app/
    shared-components/               # Stitch theme + components (@trading-agent/shared-components)
    ui/
      spog/                          # host / shell MFE
      mfe-<name>/                    # each remote MFE (this guide)
```

Every MFE (`web-app/ui/spog` and `web-app/ui/mfe-*`) **must** include at its package root:

- `README.md` — what it is, how to install, test, run, and docker-build
- `Makefile` — `build`, `test`, `run`, `docker` targets
- `Dockerfile` — image for this package

## When invoked

1. Confirm identity (`name`, `displayName`, `port`, `exposedModule`). Host goes in `web-app/ui/spog`. Remotes go in `web-app/ui/mfe-<name>`.
2. Create the package directory first, then all files from this guide inside it
3. Point webpack `remotes.spog` at the host (`spog@http://localhost:3000/remoteEntry.js` in development)
4. Use `@trading-agent/shared-components` (Stitch) for theme and components; public npm only
5. Always write `README.md`, `Makefile`, and `Dockerfile` at the package root

---

## 🎨 Project Structure for MFE's
├── src
│   ├── api
│   ├── app
│   │   ├── app-root.test.tsx
│   │   ├── app-root.tsx
│   │   ├── wrapper.test.tsx
│   │   ├── wrapper.tsx
│   │   ├── bootstrap.test.tsx
│   │   └── bootstrap.tsx
│   ├── common
│   │   ├── message_service
│   │   ├── KeyCodes.js
│   │   ├── assets
│   │   │   ├── actions
│   │   │   │   ├── edit_pencil.svg
│   │   │   │   ├── folder_action.svg
│   │   │   │   ├── folder_plus.svg
│   │   │   │   ├── save.svg
│   │   │   │   └── trash_icon.svg
│   │   │   ├── arrows
│   │   └── webpackUtils
│   │       ├── mfeUtils.test.tsx
│   │       └── mfeUtils.ts
│   ├── components
│   │   ├── ComponentExample
│   │   │   ├── ComponentExample-styles.css
│   │   │   ├── ComponentExample.test.tsx
│   │   │   ├── ComponentExample.tsx
│   │   │   ├── types.tsx
│   │   │   └── index.tsx
│   ├── features
│   │   ├── PageFeature
│   │   │   ├── components
│   │   │   │   ├── ComponentExample
│   │   │   │   │   ├── ComponentExample-styles.css
│   │   │   │   │   ├── ComponentExample.tsx
│   │   │   │   │   ├── ComponentExample.test.tsx
│   │   │   │   │   ├── types.tsx
│   │   │   │   │   └── index.tsx
│   │   │   │   ├── hooks
│   │   │   │   │   ├── useHandleMoveItem.test.tsx
│   │   │   │   │   └── useHandleMoveItem.tsx
│   │   │   │   └── utils
│   │   │   │       ├── topology.utils.test.ts
│   │   │   │       └── topology.utils.ts
│   ├── config
│   │   └── example.config.ts
│   ├── hooks
│   │   ├── useAuth
│   │   │   ├── useAuth-styles.css
│   │   │   ├── useAuth.test.tsx
│   │   │   ├── useAuth.tsx
│   │   │   └── index.tsx
│   ├── pages
│   │   ├── PageExample
│   │   │   ├── PageExample-styles.css
│   │   │   ├── PageExample.test.tsx
│   │   │   └── PageExample.tsx
│   │   └── PageExample2
│   │       ├── PageExample2-styles.css
│   │       ├── PageExample2.test.tsx
│   │       └── PageExample2.tsx
│   ├── providers
│   │   ├──── AuthenticatedProvider
│   │   │  ├── AuthenticatedProvider-styles.css
│   │   │  ├── AuthenticatedProvider.test.tsx
│   │   │  ├── AuthenticatedProvider.tsx
│   │   │  └── index.tsx
│   │   ├──── MFEDataWrapper
│   │   │  ├── MFEDataWrapper-styles.css
│   │   │  ├── MFEDataWrapper.test.tsx
│   │   │  ├── MFEDataWrapper.tsx
│   │   │  └── index.tsx
│   │   └── ThemeProvider
│   │       ├── ThemeProvider-styles.css
│   │       ├── ThemeProvider.test.tsx
│   │       └── ThemeProvider.tsx
│   ├── routes
│   │   └── AppRouter.tsx
│   ├── services
│   │   └── sampleService
│   │       ├── sampleService.test.tsx
│   │       └── sampleService.tsx
│   ├── types
│   │   ├── index.ts
│   │   ├── constants.ts
│   │   ├── css.d.ts
│   │   └── table.ts
│   └── types.d.ts
├── tsconfig.jest.json
├── tsconfig.json
└── webpack.config.js



# MFE Creation Template Guide

This guide provides **step-by-step, from-scratch instructions** for creating a new Micro Frontend (MFE). All file contents are provided inline — no reference repository or existing codebase is required.

## Key Concept: MFE Identity

MFE-specific configuration (name, port, display name, exposed module) is set directly in:
- `webpack.config.js` — Module Federation plugin name, exposed module, and dev server port
- `package.json` — Package name
- `src/hooks/auth/useAuth/useAuth.tsx` — Message service subscription name

## Monorepo vs Standalone Repo

**Monorepo** (MFE lives alongside `web-app/shared-components` in the same repository): Add `@trading-agent/shared-components` as a `file:` dependency in `package.json`. The `ThemeProvider` and shared UI primitives come from there.

**Standalone repo** (MFE lives in its own repository): Omit `@trading-agent/shared-components` from `package.json` and build a local `ThemeProvider` using Stitch UI directly. See the **Variant B ThemeProvider** section at the end of this guide.

## Prerequisites

- Node.js 22.x (`nvm use 22`)
- npm 10.x
- Access to the public npm registry (registry.npmjs.org)

## Step-by-Step MFE Creation

### Step 1: Determine MFE Identity



Before creating files, decide:
- **`name`** — snake_case, used in ModuleFederationPlugin (e.g. `mfe_kpi`)
- **`displayName`** — human-readable, used in package.json (e.g. `mfe-kpi`)
- **`port`** — dev server port (e.g. `3020`)
- **`exposedModule`** — the module path exposed to spog (the host) (e.g. `./KPI`)
- ```shellRemoteUrl: {
        development: 'spog@http://localhost:3000/remoteEntry.js',
        production: 'SPOG.SPOG_REMOTE_URL'  // Injected at runtime
    },
```
These values are used directly in `webpack.config.js` and `package.json`.

---

### Step 2: Create Directory Structure

Create the package under `web-app/ui/` (use `spog` for the host, `mfe-<name>` for a remote), then these directories inside it (note: `router` not `routes`):

```bash
mkdir -p web-app/ui/mfe-your-name
cd web-app/ui/mfe-your-name
mkdir -p src/app
mkdir -p src/hooks/auth/useAuth
mkdir -p src/hooks/message_service/filterData
mkdir -p src/providers/AuthenticatedProvider
mkdir -p src/providers/MFEDataWrapper
mkdir -p src/components
mkdir -p src/services
mkdir -p src/types
mkdir -p src/router
mkdir -p src/common/webpackUtils
mkdir -p public
mkdir -p build
mkdir -p __mocks__
```

---

### Step 3: Create package.json

Create **`package.json`**. Node.js cannot execute JS in JSON, so `name` must be set manually.

**Change for new MFE:** Update `name` to match your MFE name (kebab-case or snake_case).

```json
{
  "name": "mfe-your-name",
  "version": "1.0.0",
  "description": "",
  "main": "index.js",
  "scripts": {
    "start": "NODE_OPTIONS='--require ts-node/register' webpack serve --mode development",
    "start-app": "NODE_OPTIONS='--require ts-node/register' webpack serve --mode production",
    "start-dev": "NODE_OPTIONS='--require ts-node/register' webpack serve --mode development",
    "start-dev:all": "NODE_OPTIONS='--require ts-node/register' webpack serve --mode development --env TEST_ENV=playwright",
    "build": "NODE_OPTIONS='--require ts-node/register' webpack --mode production",
    "format": "prettier --write .",
    "lint": "eslint \"src/**/*.{ts,tsx}\"",
    "test": "jest",
    "test:watch": "jest --watch",
    "test:coverage": "jest --coverage --maxWorkers=50% --passWithNoTests",
    "test:coverage:ci": "NODE_OPTIONS='--max-old-space-size=1024' jest --coverage --maxWorkers=1 --forceExit --passWithNoTests"
  },
  "dependencies": {
    "@trading-agent/shared-components": "file:../../shared-components",
    "react": "19.1.1",
    "react-dnd": "11.1.3",
    "react-dnd-html5-backend": "11.1.3",
    "react-dom": "19.1.1",
    "react-router-dom": "^7.6.0",
    "styled-components": "6.1.19"
  },
  "devDependencies": {
    "@babel/core": "^7.28.4",
    "@babel/preset-env": "^7.28.3",
    "@babel/preset-react": "^7.27.1",
    "@babel/preset-typescript": "^7.27.1",
    "@playwright/test": "^1.52.0",
    "@testing-library/dom": "^10.4.1",
    "@testing-library/jest-dom": "^6.8.0",
    "@testing-library/react": "^16.3.2",
    "@testing-library/react-hooks": "^8.0.1",
    "@testing-library/user-event": "^14.6.1",
    "@types/jest": "^29.5.12",
    "@types/node": "^22.18.1",
    "@types/react": "^19.1.1",
    "@types/react-dom": "^19.1.1",
    "babel-jest": "^29.7.0",
    "babel-loader": "^10.0.0",
    "css-loader": "^7.1.2",
    "file-loader": "^6.2.0",
    "html-webpack-plugin": "^5.5.3",
    "identity-obj-proxy": "^3.0.0",
    "jest": "^29.7.0",
    "jest-environment-jsdom": "^29.7.0",
    "playwright": "^1.52.0",
    "style-loader": "^4.0.0",
    "ts-jest": "^29.4.3",
    "ts-loader": "^9.5.2",
    "ts-node": "^10.9.2",
    "typescript": "^5.9.3",
    "url-loader": "^4.1.1",
    "webpack": "^5.88.2",
    "webpack-cli": "^5.1.4",
    "webpack-dev-server": "^4.15.1"
  }
}
```

**Key notes:**
- `@trading-agent/shared-components` is the Stitch-generated design system (`web-app/shared-components`). In this monorepo use `"@trading-agent/shared-components": "file:../../shared-components"`. If the package does not exist yet, still add the dependency and a local `ThemeProvider` that applies Stitch CSS variables (`data-theme` + `theme.css`).
- `react-dnd` / `react-dnd-html5-backend` are required if your MFE uses drag-and-drop; remove if not needed
- No `@tanstack/react-query` or `axios` — use `fetch` directly

---

### Step 4: npm registry

Use the **public npm registry** (`registry.npmjs.org`).

If you need a project `.npmrc` at all, keep it minimal:

```properties
legacy-peer-deps=true
```

---

### Step 5: Create webpack.config.js

Create **`webpack.config.js`**. MFE identity values (name, port, exposed module) are set directly in this file — update them per MFE:

```javascript
const path = require("path");
const HtmlWebpackPlugin = require("html-webpack-plugin");
const ModuleFederationPlugin = require('webpack/lib/container/ModuleFederationPlugin');
const { DefinePlugin } = require('webpack');
const { createRemoteMFEPromise } = require("./src/common/webpackUtils/mfeUtils.ts");

// Detect CI environment
const isCI = process.env.CI === 'true' || process.env.JENKINS_HOME;

if (isCI) {
    console.log('[WEBPACK] CI mode detected - applying memory optimizations');
}

module.exports = (env, argv) => {
    const isProduction = argv.mode === "production";
    const isTestEnviroment = env.TEST_ENV === 'playwright';

    // *** CHANGE for new MFE ***
    const MFE_PORT = 3010;
    const MFE_PUBLIC_PATH = isProduction ? "auto" : `http://localhost:${MFE_PORT}/`;

    return {
        mode: argv.mode,
        // Disable cache in CI to prevent corruption and reduce memory usage
        cache: isCI ? false : {
            type: 'filesystem',
            buildDependencies: {
                config: [__filename],
            },
        },
        parallelism: isCI ? 1 : 100,
        entry: "./src/index.tsx",
        output: {
            path: path.resolve(__dirname, "dist"),
            filename: 'bundle.js',
            publicPath: MFE_PUBLIC_PATH,
            clean: true
        },
        module: {
            rules: [
                {
                    test: /\.(js|jsx|ts|tsx)$/,
                    exclude: /node_modules/,
                    use: { loader: "babel-loader" },
                },
                // CSS Modules
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
                    test: /\.(webp)$/i,
                    type: "asset/resource",
                },
                {
                    test: /\.(woff|woff2|eot|ttf|otf)$/i,
                    type: "asset/resource",
                    generator: { filename: 'fonts/[hash]-[name].[ext]' }
                },
                {
                    test: /\.(png|jpe?g|svg)$/i,
                    use: [{
                        loader: 'url-loader',
                        options: { limit: 8000, name: 'images/[hash]-[name].[ext]' }
                    }],
                },
            ],
        },
        resolve: {
            extensions: [".webpack.js", ".web.js", ".ts", ".tsx", ".js", ".jsx"],
            alias: {
                "@": path.resolve(__dirname, "./src"),
                "@trading-agent/shared-components": path.resolve(__dirname, "node_modules/@trading-agent/shared-components"),
            },
        },
        plugins: [
            new HtmlWebpackPlugin({ template: "./public/index.html" }),
            new ModuleFederationPlugin({
                // *** CHANGE for new MFE: name, exposes key ***
                name: 'mfe_your_name',
                filename: 'remoteEntry.js',
                exposes: {
                    './YourModule': './src/app/app-root',
                },
                remotes: {
                    spog: isProduction
                        ? createRemoteMFEPromise('SPOG.SPOG_REMOTE_URL', 'spog', 'spog')
                        : 'spog@http://localhost:3000/remoteEntry.js'
                },
                shared: {
                    react: { singleton: true, requiredVersion: "19.1.1", eager: true },
                    "react-dom": { singleton: true, requiredVersion: "19.1.1", eager: true },
                    "react-router-dom": { singleton: true, requiredVersion: ">=6.0.0", eager: true },
                    "@trading-agent/shared-components": { singleton: true, eager: true },
                    "styled-components": { singleton: true, requiredVersion: "6.1.19", eager: true },
                    "react-dnd": { singleton: true, requiredVersion: "11.1.3", eager: true },
                    "react-dnd-html5-backend": { singleton: true, requiredVersion: "11.1.3", eager: true },
                },
            }),
            new DefinePlugin({
                'process.env.NODE_ENV': JSON.stringify(argv.mode),
                'process.env.isProduction': isProduction,
                'process.env.isTestEnviroment': isTestEnviroment,
                'process.env.PUBLIC_PATH': MFE_PUBLIC_PATH,
            }),
        ],
        devServer: {
            static: { directory: path.join(__dirname, "dist") },
            compress: true,
            port: MFE_PORT,  // *** CHANGE for new MFE ***
            hot: true,
            historyApiFallback: true,
        },
    };
};
```

**Change for new MFE:** Update `MFE_PORT`, `name` in `ModuleFederationPlugin`, and the `exposes` key/value.

---

### Step 6: Create TypeScript Configuration Files

#### tsconfig.json

```json
{
    "compilerOptions": {
        "target": "ES6",
        "module": "CommonJS",
        "jsx": "react-jsx",
        "moduleResolution": "node",
        "esModuleInterop": true,
        "forceConsistentCasingInFileNames": true,
        "strict": true,
        "skipLibCheck": true,
        "outDir": "dist",
        "types": ["jest", "node", "@testing-library/jest-dom"],
        "baseUrl": "./src",
        "paths": {
            "@/*": ["./*"]
        },
        "resolveJsonModule": true,
        "allowJs": true,
        "isolatedModules": true,
        "noEmit": false,
        "lib": ["dom", "dom.iterable", "esnext"]
    },
    "ts-node": {
        "compilerOptions": {
            "module": "CommonJS"
        }
    },
    "include": ["src", "common", "webpack.config.js"],
    "exclude": ["node_modules", "dist", "**/*.spec.*", "**/*.test.*"]
}
```

#### tsconfig.jest.json

```json
{
  "extends": "./tsconfig.json",
  "compilerOptions": {
    "module": "commonjs",
    "noEmit": false,
    "emitDeclarationOnly": false,
    "isolatedModules": false,
    "esModuleInterop": true,
    "allowSyntheticDefaultImports": true,
    "types": ["jest", "@testing-library/jest-dom"]
  },
  "include": ["src/**/*", "jest.setup.ts"],
  "exclude": ["node_modules", "dist", "build"]
}
```

---

### Step 7: Create Babel and Jest Configuration

#### babel.config.js

```javascript
module.exports = {
  presets: [
    ['@babel/preset-env', { targets: { node: 'current' } }],
    ['@babel/preset-react', { runtime: 'automatic' }],
    '@babel/preset-typescript',
  ],
};
```

#### jest.config.js

The jest config includes CI-aware optimizations (memory limits, worker count):

```javascript
// Detect CI environment
const isCI = process.env.CI === 'true' || process.env.JENKINS_HOME || process.env.GITLAB_CI;

module.exports = {
    preset: 'ts-jest',
    testEnvironment: 'jsdom',
    bail: isCI ? 1 : false,
    setupFilesAfterEnv: ['<rootDir>/jest.setup.ts'],

    // CI: serial execution to stay within memory constraints
    maxWorkers: isCI ? 1 : '50%',
    workerIdleMemoryLimit: isCI ? '512MB' : '2048MB',
    forceExit: isCI,
    verbose: !isCI,
    silent: false,

    testTimeout: 10000,
    slowTestThreshold: 5,
    cache: true,
    cacheDirectory: '<rootDir>/.jest-cache',

    clearMocks: true,
    resetMocks: false,
    restoreMocks: false,

    moduleNameMapper: {
        '\\.(jpg|jpeg|png|gif|eot|otf|webp|ttf|woff|woff2|mp4|webm|wav|mp3|m4a|aac|oga|svg)$':
            '<rootDir>/__mocks__/fileMock.js',
        '\\.(css|scss|sass|less)$': 'identity-obj-proxy',
        'react-dom/server': 'react-dom/server.edge',
        '^@/(.*)$': '<rootDir>/src/$1',
        '^react$': '<rootDir>/node_modules/react',
        '^react-dom$': '<rootDir>/node_modules/react-dom',
        '^react-dom/(.*)$': '<rootDir>/node_modules/react-dom/$1',
        '^@trading-agent/(.*)$': '<rootDir>/node_modules/@trading-agent/$1',
    },
    transform: {
        '^.+\\.(ts|tsx)$': ['ts-jest', {
            tsconfig: {
                jsx: 'react-jsx',
                skipLibCheck: true,
                noEmit: true,
                isolatedModules: true,
                allowJs: true,
                moduleResolution: 'node',
                noResolve: false
            },
            diagnostics: false,
            isolateModules: true
        }],
        '^.+\\.(js|jsx)$': 'babel-jest',
    },
    transformIgnorePatterns: ['/node_modules/?!(@trading-agent/)/'],
    testPathIgnorePatterns: ['/component-tests/'],

    collectCoverageFrom: [
        'src/**/*.{ts,tsx,js,jsx}',
        '!src/**/*.d.ts',
        '!src/**/*.test.{ts,tsx,js,jsx}',
        '!src/**/__tests__/**',
        '!src/**/__mocks__/**',
    ],
    coverageDirectory: '<rootDir>/coverage',
    coverageReporters: isCI ? ['text-summary', 'lcov'] : ['text', 'lcov', 'html'],
};
```

#### jest.setup.ts

```typescript
import '@testing-library/jest-dom';
import { TextEncoder, TextDecoder } from 'util';

declare global {
    var TextEncoder: typeof TextEncoder;
    var TextDecoder: typeof TextDecoder;
}

(global as any).TextEncoder = TextEncoder;
(global as any).TextDecoder = TextDecoder;

// Mock window.matchMedia
Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: jest.fn().mockImplementation(query => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: jest.fn(),
    removeListener: jest.fn(),
    addEventListener: jest.fn(),
    removeEventListener: jest.fn(),
    dispatchEvent: jest.fn(),
  })),
});
```

#### __mocks__/fileMock.js

```javascript
module.exports = 'test-file-stub';
```

---

### Step 8: Create Source Code Structure

#### src/index.tsx

```typescript
// Async import to ensure Module Federation is properly initialized
import('./app/bootstrap');
```

#### src/app/bootstrap.tsx


```typescript
import { createRoot } from "react-dom/client";
import "@trading-agent/shared-components/theme.css";
import { JSX } from "react";
import { BrowserRouter } from "react-router-dom";

import App from "./app-root";

const container = document.getElementById("root");

if (!container) {
  throw new Error("Root container missing in index.html");
}

const root = createRoot(container);

export const AppRender = (): JSX.Element => {

  return (
      <BrowserRouter>
        <App />
      </BrowserRouter>
  );
};

root.render(<AppRender />);
```

#### src/app/wrapper.tsx

```typescript
import AuthMFEProvider from "@/providers/AuthenticatedProvider";
import { ThemeProvider } from "@trading-agent/shared-components";
import useAuthMFE from "@/hooks/auth/useAuth";
import { FC, ReactNode } from "react";


const AppWrapper: FC<{ children: ReactNode }> = ({ children }) => {
    const { userData, isAuthenticated, isLoading, error } = useAuthMFE();

    return (
        <ThemeProvider userData={userData}>
            <AuthMFEProvider
                userData={userData}
                isAuthenticated={isAuthenticated}
                isLoading={isLoading}
                error={error}
            >
                {children}
            </AuthMFEProvider>
        </ThemeProvider>
    )
}

export default AppWrapper;
```


#### src/app/app-root.tsx

`ThemeProvider` is imported from `@trading-agent/shared-components`. It accepts an optional `userData` prop to apply per-user theme preferences. The app-ro8ot wires authentication → theme → router.

```typescript
import AppRouter from "../router/AppRouter";
import AppWrapper from "./wrapper";

function App() {

   return (
      <AppWrapper>
         <AppRouter/>
      </AppWrapper>
   );
}

export default App;
```

**Note (Variant A — monorepo):** There is no separate `wrapper.tsx`, `ThemeProvider.tsx`, or `AdaptiveThemeProvider.tsx` — the `ThemeProvider` from `@trading-agent/shared-components` handles theme detection (including system dark/light mode) and accepts optional `userData?.theme` to override.

**Note (Variant B — standalone repo):** If `@trading-agent/shared-components` is not available, create `src/providers/ThemeProvider/ThemeProvider.tsx` with an inline Stitch UI-based implementation. See the **Variant B ThemeProvider** section at the end of this guide and replace the import in `app-root.tsx` with `import { ThemeProvider } from '@/providers/ThemeProvider/ThemeProvider';`.

#### src/router/AppRouter.tsx

Note the directory is `router`, not `routes`. The router reads auth context to extract the token.

```typescript
import { Routes, Route, Outlet } from 'react-router-dom';
import { useContext } from 'react';
import { AuthMFEContext } from '@/providers/AuthenticatedProvider';
import Dashboard from '../components/Dashboard';

function AppRouter() {
    const authDataProps = useContext(AuthMFEContext);
    const token = authDataProps?.token || (process.env.isTestEnviroment ? 'test-token' : '');

    return (
        <Routes>
            <Route path="/" element={<Outlet />}>
                <Route index element={<Dashboard token={token} />} />
            </Route>
        </Routes>
    );
}

export default AppRouter;
```

#### src/components/Dashboard/index.tsx

```typescript
interface DashboardProps {
    token?: string;
}

const Dashboard = ({ token }: DashboardProps) => {
    return (
        <div style={{ height: '100%' }}>
            <h1>Dashboard</h1>
        </div>
    );
};

export default Dashboard;
```

#### src/types.d.ts

```typescript
// Global type declarations for trading-agent MFE application
import * as React from "react";

declare module '*.module.css' {
  const classes: { [key: string]: string };
  export default classes;
}

declare module '*.svg' {
  import React from 'react';
  const ReactComponent: React.FunctionComponent<React.SVGProps<SVGSVGElement>>;
  export { ReactComponent };
  const src: string;
  export default src;
}

declare module '*.png' { const src: string; export default src; }
declare module '*.jpg' { const src: string; export default src; }
declare module '*.jpeg' { const src: string; export default src; }
declare module '*.gif' { const src: string; export default src; }
declare module '*.webp' { const content: string; export default content; }

declare global {
  interface Window {
    RUNTIME_CONFIG?: Record<string, Record<string, string> | string>;
    [key: string]: any;
  }
  var __webpack_public_path__: string;
  var __webpack_init_sharing__: (scope: string) => Promise<void>;
  var __webpack_share_scopes__: { default: any };
}

export { };
```

---

### Step 9: Create Authentication Setup

The auth flow uses the shell's `mfeUserDataMessageService` (event-driven, non-hook). The shell dispatches events in Redux slice format `{ currentUser: {...}, isAuthenticated: true }`, so the hook extracts `data.currentUser`.

#### src/hooks/auth/useAuth/useAuth.tsx

Each hook lives in its own subdirectory with an `index.tsx` barrel export.

```typescript
import { useEffect, useState } from 'react';

/**
 * Subscribes to shell user data via mfeUserDataMessageService.
 * Shell dispatches events in Redux slice format:
 *   { currentUser: { token, userName, ... }, isAuthenticated: true }
 */
const useAuthMFE = () => {
    const [userData, setUserData] = useState<any>(null);
    const [isAuthenticated, setIsAuthenticated] = useState(false);
    const [isLoading, setIsLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        let unsubscribe: (() => void) | null = null;

        const loadUserData = async () => {
            try {
                const { mfeUserDataMessageService } = await import('spog/userDataMessageService');

                const handleUserData = (event: Event) => {
                    const customEvent = event as CustomEvent;
                    const data = customEvent.detail;

                    // Extract from Redux slice format: { currentUser: {...}, isAuthenticated: true }
                    let userData = data;
                    if (data && data.currentUser) {
                        userData = data.currentUser;
                    }

                    if (userData) {
                        setUserData(userData);
                        setIsAuthenticated(!!userData?.authenticated);
                        setIsLoading(false);
                    }
                };

                // *** CHANGE for new MFE: update component name ***
                unsubscribe = mfeUserDataMessageService.subscribe('mfe-your-name', handleUserData);

            } catch (err) {
                setError(`Failed to connect to shell: ${(err as Error).message}`);
                setIsLoading(false);
            }
        };

        loadUserData();

        return () => {
            if (unsubscribe) {
                unsubscribe();
            }
        };
    }, []);

    return { userData, isAuthenticated, isLoading, error };
};

export default useAuthMFE;
```

#### src/hooks/auth/useAuth/index.tsx

```typescript
export { default } from './useAuth';
export { default as useAuthMFE } from './useAuth';
```

**Change for new MFE:** Update the subscription name `'mfe-your-name'` to your MFE's kebab-case name.

#### src/providers/AuthenticatedProvider/index.tsx

Uses the `@/` alias for imports:

```typescript
import useAuthMFE from '@/hooks/auth/useAuth';
import React, { createContext } from 'react';

export const AuthMFEContext = createContext<any | null>(null);

interface AuthMFEProviderProps {
    children: React.ReactNode;
}

const AuthMFEProvider: React.FC<AuthMFEProviderProps> = ({ children }) => {
    const { userData, isAuthenticated, isLoading, error } = useAuthMFE();

    if (isLoading && !process.env.isTestEnviroment) {
        return (
            <div style={{ padding: '20px', textAlign: 'center' }}>
                Authenticating user...
            </div>
        );
    }

    if (error && !process.env.isTestEnviroment) {
        return (
            <div style={{ padding: '20px', textAlign: 'center', color: 'red' }}>
                Authentication error{process.env.NODE_ENV === 'development' ? `: ${String(error)}` : ''}
            </div>
        );
    }

    if (!isAuthenticated && !process.env.isTestEnviroment) {
        return (
            <div style={{ padding: '20px', textAlign: 'center', color: 'orange' }}>
                You are not authorized to view this content.
            </div>
        );
    }

    return (
        <AuthMFEContext.Provider value={{ ...userData }}>
            {children}
        </AuthMFEContext.Provider>
    );
}

export default AuthMFEProvider;
```

#### src/providers/MFEDataWrapper/index.tsx

Uses a CSS `display` toggle (instead of early returns) to avoid unmounting/remounting children — critical when using React DnD or other stateful child components:

```typescript
import React from 'react';

export interface MFEDataWrapperProps {
    children: React.ReactNode;
    isLoading?: boolean;
    isError?: boolean;
    data?: any;
    dataExist?: boolean;
    loadingMessage?: string;
    errorMessage?: string | Error | null;
    noDataMessage?: string;
}

const getErrorMessage = (error: string | Error | null): string => {
    if (typeof error === 'string') return error;
    if (error instanceof Error) return error.message;
    return 'Unknown error';
};

const MFEDataWrapper: React.FC<MFEDataWrapperProps> = ({
    children,
    isLoading = false,
    isError = false,
    data = null,
    dataExist = false,
    loadingMessage = 'Loading data...',
    errorMessage = 'There was a problem trying to fetch your data',
    noDataMessage = 'No data available',
}) => {
    const hasData = (Array.isArray(data) && data.length > 0 && data[0]?.name !== undefined) || dataExist;
    const shouldShowChildren = !isLoading && !isError && hasData;

    return (
        <>
            {isLoading && (
                <div style={{ padding: '20px', textAlign: 'center', color: 'var(--color-text-secondary)' }}>
                    {loadingMessage}
                </div>
            )}
            {!isLoading && isError && (
                <div style={{ padding: '20px', textAlign: 'center', color: 'var(--color-text-secondary)' }}>
                    There was a problem trying to fetch your data
                    {process.env.NODE_ENV === 'development' ? `: ${getErrorMessage(errorMessage)}` : ''}
                </div>
            )}
            {!isLoading && !isError && !hasData && (
                <div style={{ padding: '20px', textAlign: 'center', color: 'var(--color-text-secondary)' }}>
                    {noDataMessage}
                </div>
            )}
            {/* Always rendered — use CSS to show/hide, NOT conditional rendering */}
            <div style={{ display: shouldShowChildren ? 'block' : 'none', height: '100%' }}>
                {children}
            </div>
        </>
    );
};

export default MFEDataWrapper;
```

#### src/providers/MFEDefaultStateProvider.tsx

Fallback state provider used when the shell's `StateProvider` is unavailable (e.g. in tests or standalone mode):

```typescript
import React, { ReactNode } from 'react';

interface MFEStateProviderProps {
    children: ReactNode;
}

export const MFEStateProvider: React.FC<MFEStateProviderProps> = ({ children }) => {
    return <>{children}</>;
};

export default MFEStateProvider;
```

---

### Step 10: Create Filter Data Message Service Hook

MFEs can subscribe to filter selections (products, timeframe) pushed from the shell via `mfeFilterDataMessageService`. Place message-service hooks under `src/hooks/message_service/`.

#### src/hooks/message_service/filterData/useFilterData.tsx

```typescript
import { useEffect, useState } from 'react';
import { FilterData } from 'spog/filterDataMessageService';

/**
 * Custom hook to subscribe to filter data events from the shell application.
 * Receives filter data updates including products, elements, and timeframe.
 */
const useFilterData = () => {
    const [filterData, setFilterData] = useState<FilterData | null>(null);
    const [isLoading, setIsLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        let unsubscribe: (() => void) | null = null;

        const loadFilterData = async () => {
            try {
                const { mfeFilterDataMessageService } = await import('spog/filterDataMessageService');

                const handleFilterData = (event: Event) => {
                    const customEvent = event as CustomEvent;
                    const data = customEvent.detail as FilterData;

                    if (data) {
                        setFilterData(data);
                        setIsLoading(false);
                    }
                };

                // *** CHANGE for new MFE: update component name ***
                unsubscribe = mfeFilterDataMessageService.subscribe('mfe-your-name', handleFilterData);

            } catch (err) {
                setError(`Failed to connect to shell: ${(err as Error).message}`);
                setIsLoading(false);
            }
        };

        loadFilterData();

        return () => {
            if (unsubscribe) {
                unsubscribe();
            }
        };
    }, []);

    return { filterData, isLoading, error };
};

export default useFilterData;
```

#### src/hooks/message_service/filterData/index.tsx

```typescript
export { default } from './useFilterData';
export { default as useFilterData } from './useFilterData';
```

---

### Step 11: CSS Styling

- The MFE renders inside the shell's grid layout system — do **not** use `100vh`
- The outermost container div of the MFE should use `height: 100%`
- This ensures it fills the shell's `shell content cell` cell without overflow

```css
/* Correct */
.mfe-root { height: 100%; }

/* Wrong - causes overflow in shell grid */
.mfe-root { height: 100vh; }
```

---

### Step 12: Create Type Definitions

#### src/types/module-federation.d.ts

Declares both shell message services. The `MfeMessageService` interface covers both `userDataMessageService` and `filterDataMessageService`:

```typescript
// TypeScript declarations for Module Federation remote modules

interface MfeMessageService {
  /**
   * Subscribe to events (non-hook version)
   * @param componentName - Name of the subscribing component
   * @param handler - Event handler function
   * @returns Cleanup function to unsubscribe
   */
  subscribe(componentName: string, handler: (event: Event) => void): () => void;
  publish(data: any): void;
}

declare module 'spog/userDataMessageService' {
  export interface UserData {
    userName?: string;
    token?: string;
    userRoles?: string[];
    id?: string;
    email?: string;
    authenticated?: boolean;
    theme?: string;
    [key: string]: any;
  }

  export const mfeUserDataMessageService: MfeMessageService;
}

declare module 'spog/filterDataMessageService' {
  export interface FilterData {
    products?: Array<Product>;
    timeframe?: TimeFrame;
  }
  export interface Product {
    cluster: string;
    name: string;
    elements: { name: string; version: string }[];
  }
  export interface TimeFrame {
    type: 'RELATIVE' | 'ABSOLUTE';
    from: number | string;
    to: number | string;
  }

  export const mfeFilterDataMessageService: MfeMessageService;
}
```

---

### Step 13: Create Public Files

#### public/index.html

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>trading-agent MFE</title>
</head>
<body>
    <div id="root"></div>
</body>
</html>
```

---

### Step 14: Create Common Utilities

#### src/common/webpackUtils/mfeUtils.ts

```typescript
/**
 * Utility function to create a promise-based remote MFE loader for Module Federation.
 * This allows for dynamic runtime resolution of remote entry URLs in production.
 */
export function createRemoteMFEPromise(
  envVarPath: string,
  remoteName: string,
  displayName: string
): string {
  return \`promise new Promise((resolve, reject) => {
    const urlPath = "\${envVarPath}";
    const remoteName = "\${remoteName}";
    
    // Navigate through nested environment variables (e.g., SPOG.SPOG_REMOTE_URL)
    const parts = urlPath.split('.');
    let remoteUrl = window;
    
    for (const part of parts) {
      remoteUrl = remoteUrl[part];
      if (!remoteUrl) {
        reject(new Error(\\\`\${displayName} remote URL not found at path: \\\${urlPath}\\\`));
        return;
      }
    }
    
    if (typeof remoteUrl !== 'string') {
      reject(new Error(\\\`\${displayName} remote URL is not a string: \\\${typeof remoteUrl}\\\`));
      return;
    }
    
    const script = document.createElement('script');
    script.src = remoteUrl;
    script.type = 'text/javascript';
    script.async = true;
    
    script.onload = () => {
      const container = window[remoteName];
      if (!container) {
        reject(new Error(\\\`\${displayName} container not found: \\\${remoteName}\\\`));
        return;
      }
      resolve(container);
    };
    
    script.onerror = (error) => {
      reject(new Error(\\\`Failed to load \${displayName} from \\\${remoteUrl}: \\\${error}\\\`));
    };
    
    document.head.appendChild(script);
  })\`;
}
```

---

### Step 15: README, Makefile, and Dockerfile

Every MFE package root (`web-app/ui/spog` or `web-app/ui/mfe-<name>`) must have these three files.

#### README.md

```markdown
# mfe-your-name

Micro frontend for trading-agent. Loaded by `web-app/ui/spog`.

## Prerequisites

- Node.js 22 (`nvm use 22`)
- npm 10
- Docker (for `make docker`)

## Commands

| Make target | What it does |
|-------------|----------------|
| `make build` | Install deps and production webpack build |
| `make test` | Unit tests with coverage |
| `make run` | Dev server (webpack serve) |
| `make docker` | Build the Docker image |

```bash
nvm use 22
make build
make test
make run          # http://localhost:<port>
make docker
```
```

#### Makefile

Must live at the package root. Required targets: `build`, `test`, `run`, `docker`.

```makefile
.PHONY: build test run docker clean

MFE_NAME := mfe-your-name
IMAGE_NAME := trading-agent-$(MFE_NAME)
BUILD_VERSION ?= latest

build:
	nvm use 22
	npm install --legacy-peer-deps
	npm run build

test:
	nvm use 22
	npm run test:coverage

run:
	nvm use 22
	npm run start-dev

docker: build
	docker build --file ./Dockerfile --tag $(IMAGE_NAME):$(BUILD_VERSION) .

clean:
	rm -rf node_modules dist coverage
```

#### Dockerfile

Must live at the package root (not only under `build/`).

```dockerfile
FROM nginx:alpine
COPY dist/ /usr/share/nginx/html/
COPY nginx.conf /etc/nginx/conf.d/default.conf
```

A minimal `nginx.conf` in the same directory:

```nginx
server {
    listen 80;
    root /usr/share/nginx/html;
    location / {
        try_files $uri $uri/ /index.html;
    }
}
```

---

### Step 16: Create Environment Files

#### .env

```env
# Environment Variables
REACT_APP_API_URL=http://localhost:8080/api
```

#### .gitignore

```gitignore
# Dependencies
node_modules/
package-lock.json

# Build output
dist/
build/
*.log

# Testing
coverage/
*.test.js
*.spec.js
.nyc_output/
playwright-report/
test-results/

# IDE
.vscode/
.idea/
*.swp
*.swo
*~

# OS
.DS_Store
Thumbs.db

# Environment
.env
.env.local
.env.development.local
.env.test.local
.env.production.local

# Cache
.cache/
.parcel-cache/
.webpack/

# Misc
*.tgz
mfe_packages/
```

---

## Quick Start Checklist

Use this checklist when creating a new MFE:

- [ ] Create the package at `web-app/ui/mfe-<name>` (or `web-app/ui/spog` for the host)
- [ ] Create all files from scratch following the steps in this guide
- [ ] Add package-root `README.md`, `Makefile` (`build`, `test`, `run`, `docker`), and `Dockerfile`
- [ ] Update `package.json` `name` field (kebab-case or snake_case)
- [ ] Update `webpack.config.js`: `MFE_PORT`, `ModuleFederationPlugin.name`, `exposes` key
- [ ] Use public npm (`registry.npmjs.org`)
- [ ] Update subscription name in `src/hooks/auth/useAuth/useAuth.tsx` (`'mfe-your-name'`)
- [ ] Update subscription name in `src/hooks/message_service/filterData/useFilterData.tsx` (if used)
- [ ] Run `nvm use 22 && npm install --legacy-peer-deps`
- [ ] Run `make run` to verify the MFE loads on the configured port
- [ ] Run `make test` to verify tests pass

## Configuration Summary

### Files Requiring Manual Updates (per MFE)

| File | What to change |
|------|---------------|
| Path | `web-app/ui/mfe-<name>` or `web-app/ui/spog` |
| `README.md` | Name, port, make commands |
| `Makefile` | `MFE_NAME`, `build` `test` `run` `docker` |
| `Dockerfile` | Image for this package |
| `package.json` | `name` field |
| `webpack.config.js` | `MFE_PORT`, `ModuleFederationPlugin.name`, `exposes` key |
| `src/hooks/auth/useAuth/useAuth.tsx` | subscription component name string |
| `src/hooks/message_service/filterData/useFilterData.tsx` | subscription component name string |

### Shell Message Services

| Remote module | Exported constant | Purpose |
|--------------|------------------|---------|
| `spog/userDataMessageService` | `mfeUserDataMessageService` | Auth / user data (token, roles, theme) |
| `spog/filterDataMessageService` | `mfeFilterDataMessageService` | Product/timeframe filter selections |

Both services use the same `MfeMessageService` interface: `subscribe(componentName, handler)` → cleanup function.

### ThemeProvider

**Variant A — Monorepo**: `ThemeProvider` is provided by `@trading-agent/shared-components` (local `file:` package). It detects system dark/light mode and accepts an optional `userData?.theme` override.

```typescript
import { ThemeProvider } from "@trading-agent/shared-components";
// Usage: <ThemeProvider userData={userData}>...</ThemeProvider>
```

**Variant B — Standalone repo**: Build a local `ThemeProvider` at `src/providers/ThemeProvider/ThemeProvider.tsx` using Stitch UI primitives directly (see inline code at the end of this guide). Update `app-root.tsx` to import from `@/providers/ThemeProvider/ThemeProvider` instead.

### Key Architecture Notes

- **No `useShellStateProvider`** — MFEs do not need to wrap with shell's Redux StateProvider
- **No `mfe.config.ts`** — MFE identity values are set directly in `webpack.config.js` and `package.json`
- **`src/router/`** — router directory name is `router`, not `routes`
- **Hook barrel pattern** — each hook lives in its own subdirectory with `index.tsx` re-exporting the default
- **`@/` alias** — maps to `src/`, configured in both `tsconfig.json` and `webpack.config.js` resolve aliases
- **`MFEDataWrapper` CSS toggle** — uses `display: none/block` to avoid unmounting stateful children (React DnD)
- **Bootstrap `StyleSheetManager`** — required to suppress styled-components prop-forwarding warnings in Module Federation
- **No `mfe.config.ts`** — MFE identity values are set directly in `webpack.config.js` and `package.json`; the Makefile/build.sh hardcode the name as a variable

---

## Variant B ThemeProvider (until `@trading-agent/shared-components` exists)

Use this when `web-app/shared-components` has not been generated from Stitch yet. Create a local provider that only sets `data-theme` and loads CSS variables. Replace these placeholder tokens with Stitch exports, then switch the import to `import { ThemeProvider } from "@trading-agent/shared-components"`.

#### src/providers/ThemeProvider/theme.css

```css
:root,
:root[data-theme="light"] {
  --color-text: #111111;
  --color-text-secondary: #5c5c5c;
  --color-app-background: #f5f5f5;
  --color-surface: #ffffff;
  --space-xs: 4px;
  --space-sm: 8px;
  --space-md: 16px;
  --space-lg: 24px;
}

:root[data-theme="dark"] {
  --color-text: #f5f5f5;
  --color-text-secondary: #b3b3b3;
  --color-app-background: #121212;
  --color-surface: #1e1e1e;
}
```

#### src/providers/ThemeProvider/ThemeProvider.tsx

```typescript
import React, { useEffect, useState } from 'react';
import './theme.css'; // move to `@trading-agent/shared-components/theme.css` when web-app/shared-components exists

type ThemeMode = 'light' | 'dark';

interface ThemeProviderProps {
  children: React.ReactNode;
  userData?: { theme?: string };
}

const getSystemTheme = (): ThemeMode =>
  window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';

export const ThemeProvider: React.FC<ThemeProviderProps> = ({ children, userData }) => {
  const [theme, setTheme] = useState<ThemeMode>(() => {
    if (userData?.theme === 'dark' || userData?.theme === 'light') return userData.theme;
    return getSystemTheme();
  });

  useEffect(() => {
    if (userData?.theme === 'dark' || userData?.theme === 'light') {
      setTheme(userData.theme as ThemeMode);
    }
  }, [userData?.theme]);

  useEffect(() => {
    const mq = window.matchMedia('(prefers-color-scheme: dark)');
    const handler = () => {
      if (!userData?.theme) setTheme(getSystemTheme());
    };
    mq.addEventListener('change', handler);
    return () => mq.removeEventListener('change', handler);
  }, [userData?.theme]);

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    document.documentElement.style.colorScheme = theme;
  }, [theme]);

  return <>{children}</>;
};

export default ThemeProvider;
```

#### src/app/app-root.tsx (Variant B)

```typescript
import AuthMFEProvider from "@/providers/AuthenticatedProvider";
import AppRouter from "../router/AppRouter";
import { ThemeProvider } from "@/providers/ThemeProvider/ThemeProvider";
import useAuthMFE from "@/hooks/auth/useAuth";

function App() {
   const { userData } = useAuthMFE();

   return (
      <ThemeProvider userData={userData}>
         <AuthMFEProvider>
            <AppRouter />
         </AuthMFEProvider>
      </ThemeProvider>
   );
}

export default App;
```
