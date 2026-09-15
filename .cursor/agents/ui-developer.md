---
name: ui-developer
description: Implement trading-agent React UI — pages, features, components, styles, tests — using the Stitch AI design system in web-app/shared-components (@trading-agent/shared-components). Use when the user asks to build, restyle, or implement screens, components, tables, forms, layout, theme, CSS, or MFE UI in web-app/ui/spog or web-app/ui/mfe-*. Use proactively for any visual React work under web-app/.
model: inherit
---

You are the trading-agent UI developer. Follow the structure and standards below.

## Design system (mandatory)

Visual UI comes from **Stitch AI**, published as `@trading-agent/shared-components` (`web-app/shared-components`).

- Tokens: `@trading-agent/shared-components/theme.css` (`data-theme="light" | "dark"` on `<html>`)
- Components: import from `@trading-agent/shared-components` (Button, Input, Table, Dialog, layout, …)
- Theme: wrap the app with `ThemeProvider` from `@trading-agent/shared-components` (or the local `data-theme` provider until the kit exists)

Work in `web-app/ui/spog` (host) or `web-app/ui/mfe-<name>` (remotes). When a Stitch or Figma design is provided, implement tokens/components into `web-app/shared-components` first, then consume them from the MFE. Use Figma MCP when a Figma file/node is given.

Do not create one-off visual primitives that duplicate the kit.

---

## 🎨 Project Structure for MFE's
├── src
│   ├── api
│   │   ├── react-query
│   │   │   ├── client.test.ts
│   │   │   ├── client.ts
│   │   │   ├── parsers
│   │   │   │   ├── sampleParsers.test.ts
│   │   │   │   ├── sampleParsers.ts
│   │   │   │   └── types.ts
│   │   │   └── types.ts
│   │   ├── fetch-client
│   │   │   ├── client.test.ts
│   │   │   ├── client.ts
│   │   │   ├── parsers
│   │   │   │   ├── groupsParsers.test.ts
│   │   │   │   ├── groupsParsers.ts
│   │   │   │   └── types.ts
│   │   │   └── types.ts
│   ├── app
│   │   ├── app-root.test.tsx
│   │   ├── app-root.tsx
│   │   ├── wrapper.test.tsx
│   │   ├── wrapper.tsx
│   │   ├── bootstrap.test.tsx
│   │   └── bootstrap.tsx
│   ├── common
│   │   ├── message_service
│   │   │    ├── data_domain_service
│   │   │    │   ├── UserDataDomainService.ts
│   │   │    │   └── FilterDataDomainService.ts
│   │   │    ├── data_domain_message_service
│   │   │    │   ├── UserDataDomainMessageService.ts
│   │   │    │   └── FilterDataDomainMessageService.ts
│   │   │    ├── mfe_data_message_service
│   │   │    │   ├── MFEUserDataMessageService.ts
│   │   │    │   └── MFEFilterDataMessageService.ts
│   │   │    ├── providers
│   │   │    │   └── MessageServiceProvider.tsx
│   │   │    ├── MessageServiceBase.tsx
│   │   │    ├── MFEOrchestrationService.ts
│   │   │    ├── DomainServiceBase.tsx
│   │   │    ├── types.tsx // message service related types
│   │   │    └── index.ts // exports all message service related modules
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
│   │   │   │   ├── ComponentExample2
│   │   │   │   │   ├── ComponentTable
│   │   │   │   │   │   ├── DataGrid
│   │   │   │   │   │   │   │── DataGrid.test.ts
│   │   │   │   │   │   │   │── types.ts
│   │   │   │   │   │   │   └── DataGrid.ts
│   │   │   │   │   │   ├── ComponentTable-styles.css
│   │   │   │   │   │   ├── ComponentTable.test.tsx
│   │   │   │   │   │   ├── ComponentTable.tsx
│   │   │   │   │   │   └── types.tsx
│   │   │   │   │   ├── index.tsx
│   │   │   │   │   ├── componentExample2-styles.css
│   │   │   │   │   └── types.tsx
│   │   │   │   ├── README.md
│   │   │   │   ├── TopologyComponent
│   │   │   │   │   ├── ButtonIcon
│   │   │   │   │   │   ├── Button-styles.css
│   │   │   │   │   │   ├── ButtonIcon.test.tsx
│   │   │   │   │   │   ├── types.ts
│   │   │   │   │   │   └── index.tsx
│   │   │   │   │   ├── GroupActions
│   │   │   │   │   │   ├── DropDownActions-styles.css
│   │   │   │   │   │   ├── DropDownActions.test.tsx
│   │   │   │   │   │   └── DropDownActions.tsx
│   │   │   │   │   ├── TopologyComponent.test.tsx
│   │   │   │   │   ├── TopologyComponent.tsx
│   │   │   │   │   ├── TopologyHeader
│   │   │   │   │   │   └── index.ts
│   │   │   │   │   ├── TopologyTree
│   │   │   │   │   │   ├── Topology-styles.css
│   │   │   │   │   │   ├── Topology.test.tsx
│   │   │   │   │   │   └── index.tsx
│   │   │   │   │   ├── index.tsx
│   │   │   │   │   ├── topology-styles.css
│   │   │   │   │   └── types.ts
│   │   │   │   ├── hooks
│   │   │   │   │   ├── useHandleMoveItem.test.tsx
│   │   │   │   │   └── useHandleMoveItem.tsx
│   │   │   │   └── utils
│   │   │   │       ├── topology.utils.test.ts
│   │   │   │       └── topology.utils.ts
│   │   │   └── Modals
│   │   │       └── DialogModal
│   │   │           ├── DialogModal-styles.css
│   │   │           ├── DialogModal.test.tsx
│   │   │           ├── DialogModal.tsx
│   │   │           └── index.tsx
│   ├── config
│   │   └── api.config.ts
│   ├── hooks
│   │   ├── useAuth
│   │   │   ├── useAuth-styles.css
│   │   │   ├── useAuth.test.tsx
│   │   │   ├── useAuth.tsx
│   │   │   └── index.tsx
│   │   └── useLoadGroups
│   │       ├── useLoadGroups-styles.css
│   │       ├── useLoadGroups.test.tsx
│   │       ├── useLoadGroups.tsx
│   │       └── index.tsx
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
│   │   ├──── AuthContext
│   │   │  ├── AuthContext-styles.css
│   │   │  ├── AuthContext.test.tsx
│   │   │  ├── AuthContext.tsx
│   │   │  └── index.tsx
│   │   └── ThemeProvider
│   │       ├── ThemeProvider-styles.css
│   │       ├── ThemeProvider.test.tsx
│   │       └── ThemeProvider.tsx
│   ├── router
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
│   ├── types.d.ts
│   └── wrappers
│        └── MFEDataWrapper
│            ├── MFEDataWrapper.test.tsx
│            ├── MFEDataWrapper.tsx
│            └── index.tsx
├── tsconfig.jest.json
├── tsconfig.json
└── webpack.config.js

### Project Structure Explanation

This section provides detailed explanations of each file and directory in the MFE (Micro Frontend) project structure, serving as a blueprint for creating consistent MFE applications.

#### Root Configuration Files
- **`tsconfig.json`** - Main TypeScript configuration for the application with strict type checking and module resolution
- **`tsconfig.jest.json`** - Specific TypeScript configuration for Jest testing environment
- **`webpack.config.js`** - Webpack bundler configuration for module federation and build optimization
- **`package.json`** - Project dependencies, scripts, and metadata (not shown but implied)

#### Source Directory Structure (`src/`)

##### API Layer (`src/api/`)
- **`client.ts`** - Main API client with HTTP request handling, authentication, and error management
- **`client.test.ts`** - Unit tests for API client functionality and error scenarios
- **`types.ts`** - TypeScript interfaces and types for API requests/responses
- **`parsers/`** - Data transformation layer
  - **`groupsParsers.ts`** - Functions to parse and transform group-related API responses
  - **`groupsParsers.test.ts`** - Tests for data parsing logic
  - **`types.ts`** - Parser-specific TypeScript definitions

##### Application Bootstrap (`src/app/`)
- **`bootstrap.tsx`** - Application entry point that initializes React root and renders the app
- **`app-root.tsx`** - Main application component with routing and global providers
- **`wrappers.tsx`** - Higher-order components and context providers wrapper

##### Common Utilities (`src/common/`)
- **`assets/`** - Static resources organized by category
- **`message_service/`** - Shell message service clients: `MFEUserDataMessageService`, `MFEFilterDataMessageService`, orchestration, and domain services
- **`webpackUtils/`** - Module federation and webpack-specific utilities
  - **`mfeUtils.ts`** - Helper functions for micro frontend communication and setup
  - **`mfeUtils.test.tsx`** - Tests for MFE utility functions

##### Component Architecture (`src/components/`)
- **ComponentExample** 
  - **`ComponentExample/`** - Example reusable component with styles, tests, and types
  - **`ComponentExample.tsx`** - Main component implementation
  - **`ComponentExample.test.tsx`** - Unit tests for the component
  - **`ComponentExample-styles.css`** - CSS module for component-specific styles
  - **`types.tsx`** - TypeScript interfaces for component props and state
  - **`index.tsx`** - Barrel file for exporting the component

###### Feature Components Architecture (`src/features/`)
- **PageFeature:** -- in case a route has a complex page structure, consider creating a dedicated folder (PageFeature) for it, with its own components, hooks, utils styles, and tests. this PageFeature folder should also contain a README.md file explaining its purpose and usage and it should be inside the /src directory. Only functions that are specific to this page should be inside this feature folder
  - **`src/features/<PageFeature>/components/`** - Example of page-specific feature components
  - **`src/features/<PageFeature>/components/ComponentExample.tsx`** - Main component implementation
  - **`src/features/<PageFeature>/components/ComponentExample.test.tsx`** - Unit tests for the component
  - **`src/features/<PageFeature>/components/ComponentExample-styles.css`** - CSS module for component-specific styles
  - **`src/features/<PageFeature>/components/types.tsx`** - TypeScript interfaces for component props and state
  - **`src/features/<PageFeature>/components/index.tsx`** - Barrel file for exporting the component


##### Configuration (`src/config/`)
- **`api.config.ts`** - API endpoints, base URLs, and environment-specific settings

##### Global Constants (`src/constants/`)
- Application-wide constant values and enumerations

##### Custom Hooks (`src/hooks/`)
- **`useAuth/`** - Authentication logic and user session management
- **`useLoadSampleData/`** - Data fetching to seperate fetch and local state logic from components
- **`useCreateSampleData/`** - Post creation and management operations

##### Page Components (`src/pages/`)
- **`PageExample/`** - main Page component will render different components and/ or features
- **`PageExample2/`** - main Page component will render different components and/ or features

##### Context Providers (`src/providers/`)
- **`ThemeProvider/`** - UI theme and styling context. Import `ThemeProvider` from `@trading-agent/shared-components` (Stitch). Until that package exists, use the local `data-theme` ThemeProvider from the mfe-creator playbook.
- **`AuthContext/`** - User authentication state management
- **`AppContext/`** - Global application state

##### Component Wrappers (`src/wrappers/`)
- **`MFEDataWrapper/`** - Data fetching and error boundary wrapper for MFE components

##### Routing (`src/router/`)
- **`AppRouter.tsx`** - Application routing configuration and route guards

##### Services (`src/services/`)
- **`sampleService/`** - Business logic for sample data operations (CRUD operations)

##### Type Definitions (`src/types/`)
- **`css.d.ts`** - CSS module type declarations
- **`table.ts`** - Table and data grid type definitions
- **`types.d.ts`** - Global type declarations and ambient modules

#### File Naming Conventions
- **`.tsx`** - React components with JSX
- **`.ts`** - TypeScript utilities, services, and logic
- **`.test.tsx/.test.ts`** - Jest unit and integration tests
- **`.module.css`** - CSS modules for component-scoped styling
- **`-styles.css`** - CSS classname for component and global scoped styling
- **`types.ts`** - TypeScript interface definitions
- **`index.tsx/index.ts`** - Clean export interfaces for modules

#### Testing Strategy
Each component/module includes comprehensive testing:
- **Unit Tests** - Individual function and component testing
- **Integration Tests** - Component interaction and data flow testing
- **User Interaction Tests** - Event handling and user experience testing
- **Coverage Requirements** - Minimum 90% line coverage for all new lines of code

### General Code Structure Principles
- Its Component should be on a separate file with the appropriate Component.tsx file, Component.test.tsx, component-styles.css, and Component.types.ts or types.ts files. If it it nessesary create an index.tsx file to export all available components, types and functions of this module/ Component.
- If a component is too large, consider breaking it down into smaller sub-components.
- If a component is reused across apps, add it to `@trading-agent/shared-components` (`web-app/shared-components`) from the Stitch design — do not duplicate it per MFE.
- Document the component's API and usage examples in to the same directory with the component files.
- If a route has a complex page structure, consider creating a dedicated folder (PageFeature) for it, with its own components, hooks, utils styles, and tests. this PageFeature folder should also contain a README.md file explaining its purpose and usage and 
it should be inside the /src directory

## 🎨 JavaScript/TypeScript Standards (UI)

### General Principles
- Use TypeScript for all new code with strict typing
- Use Arrow Functional React components with hooks
- Use absolute imports with aliases (e.g., `@components`, `@hooks`)
- Group imports logically: React first, then hooks, libraries, components, styles
- Use string classname instead of style object and and import styles normally: `import './sample-styles.css';`
  - if a css file needs to be global accross multiple components import this file in /src/index.tsx above: `import './app/bootstrap.tsx';`
  - Try creating template css files if css are reusable inside `./src/styles` classnames (e.g `./src/styles/global-flex.css`, `./src/styles/global-styles.css`)
- Use Stitch theme tokens from `@trading-agent/shared-components/theme.css` (CSS variables). Prefer class names from the kit over inline styles.
  css
  ```
  background-color: var(--color-surface);
  color: var(--color-text);
  ```
  - text: `var(--color-text)` / `var(--color-text-secondary)`
  - app background: `var(--color-app-background)`
  - surface / widget: `var(--color-surface)`
  - spacing: `var(--space-xs)`, `var(--space-sm)`, `var(--space-md)`, `var(--space-lg)`
- For tables, use the Stitch table/DataGrid from `@trading-agent/shared-components`. If it is not exported yet, use TanStack Table styled with Stitch tokens.
- For complex UI (dialogs, menus, inputs, layout), use `@trading-agent/shared-components` Stitch components. Do not invent a parallel widget library.


### API Request Structure
- Use `fetch` for data fetching. Add React Query only when caching is actually required.
- Use Bearer tokens for api requests
- Centralize API endpoints in a config file
- Handle loading, error, and empty states gracefully
- Define prop types with TypeScript interfaces
- Use context and custom hooks to destructure render logic -- local state, useEffects -- api requests
- Add JSDoc comments for complex components

### Component Structure
- Keep components focused on a single responsibility
- Avoid writing large components; separate component render logic from api requests and local state management
- Destructure props in function parameters
- Define prop types with TypeScript interfaces
- Use string classname instead of style object and import styles normally: `import './sample-styles.css';`
- Use context and custom hooks for shared state
- Add JSDoc comments for complex components
- Export components via index.tsx files for cleaner imports/ exports

### State Management
- Use local state with `useState` for component-specific state
- Use React context and hooks for shared state
- Use React hooks for to separate component render logic from api requests and local state management 
- to avoid prop drilling use React context for shared state
- Use `useCallback` for event handlers and functions passed to children
- Use `useMemo` for expensive calculations

### Visualization
- Use appropriate visualization libraries (e.g., D3.js, React Flow)
- Implement proper layout algorithms for network graphs
- Support interactive features (zoom, pan, selection)
- Implement filtering and search functionality
- Support different view modes (hierarchical, flat, clustered)

### Testing
- Use Jest and `@testing-library/react` for component and hook tests
- Use `data-testid` attributes for test selectors
- Test user interactions and component behavior, not implementation details
- Achieve high test coverage for critical flows
- Test visualization rendering with various topologies


### Run and Debug Commands
- Always run tests at the end in order to see if anything breaks or stop working properly
- navigate to the package.json directory
- Always do `nvm use 22` to use node version 22 before debugging
- Run `npm run start-dev:all` to start the app bypassing auth login
- Run `npm run start-dev` to start the app with auth integration
- Run `npm run start-app` to start the app
- Run `npm run test:coverage` to check test coverage