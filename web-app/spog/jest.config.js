// Detect CI environment
const isCI = process.env.CI === 'true' || process.env.JENKINS_HOME || process.env.GITLAB_CI;

module.exports = {
    preset: 'ts-jest',
    testEnvironment: "jsdom",
    setupFilesAfterEnv: ["<rootDir>/jest.setup.ts"], // for react-router-dom and keycloak because they use it internal

    // ============================================================================
    // PERFORMANCE OPTIMIZATIONS FOR CI/CD (6GB Memory Limit)
    // ============================================================================

    // CRITICAL: Run tests serially in CI to prevent OOM errors
    // CI: Use 1 worker (runInBand equivalent) to stay within 6GB limit
    // Local: Use 50% of CPUs for better performance
    maxWorkers: isCI ? 1 : '50%',

    // Limit memory per worker (in MB) - prevents OOM errors
    workerIdleMemoryLimit: isCI ? '1536MB' : '2048MB',

    // Force exit after tests complete to prevent worker hangs
    forceExit: isCI,

    // Cache configuration - prevent cache bloat
    cache: true,
    cacheDirectory: '<rootDir>/.jest-cache',

    // Clear mocks between tests to free memory
    clearMocks: true,
    resetMocks: false,
    restoreMocks: false,

    // Bail early on CI to save resources
    bail: isCI ? 1 : 0,

    // Disable verbose output in CI to reduce log size
    verbose: !isCI,

    // Disable silent mode to allow optimization messages
    silent: false,

    // Optimize test execution
    testTimeout: 10000, // 10 seconds max per test
    slowTestThreshold: 5, // Warn if test takes > 5 seconds

    moduleNameMapper: {
        "\\.(jpg|jpeg|png|gif|eot|otf|webp|ttf|woff|woff2|mp4|webm|wav|mp3|m4a|aac|oga|svg)$":
            "<rootDir>/__mocks__/fileMock.js",
        "\\.(css)$": "identity-obj-proxy",
        "react-dom/server": "react-dom/server.edge",
        // Shared-components sources must use this package's React copy.
        "^react$": "<rootDir>/node_modules/react",
        "^react/(.*)$": "<rootDir>/node_modules/react/$1",
        "^react-dom$": "<rootDir>/node_modules/react-dom",
        "^react-dom/(.*)$": "<rootDir>/node_modules/react-dom/$1",
        "^@trading-agent/shared-components$": "<rootDir>/../shared-components/src/shell.ts",
        "^@components/(.*)$": "<rootDir>/src/components/$1",
        "^@common/(.*)$": "<rootDir>/src/common/$1",
        "^@constants/(.*)$": "<rootDir>/src/constants/$1",
        "^@styles/(.*)$": "<rootDir>/src/styles/$1",
        "^@layouts/(.*)$": "<rootDir>/src/layouts/$1",
    },
    transform: {
        '^.+\\.(ts|tsx)$': ['ts-jest', {
            tsconfig: {
                jsx: 'react-jsx',
                esModuleInterop: true,
                resolveJsonModule: true,
                skipLibCheck: true,
                noEmit: true,
                isolatedModules: true,
                allowJs: true,
                moduleResolution: 'node',
                noResolve: false
            },
            diagnostics: false,
            // Isolate modules to prevent memory leaks
            isolatedModules: true
        }],
        '^.+\\.(js|jsx)$': 'babel-jest',
    },
    transformIgnorePatterns: ["/node_modules/(?!(react-dnd|react-dnd-html5-backend|dnd-core|@react-dnd)/)"],
    testPathIgnorePatterns: ["/component-tests/", "/node_modules/"],
    
    // Coverage configuration
    collectCoverageFrom: [
        'src/**/*.{ts,tsx}',
        '!src/**/*.d.ts',
        '!src/**/*.test.{ts,tsx}',
        '!src/__tests__/**',
        '!src/index.tsx',
        '!src/**/index.ts',
        '!src/**/index.tsx'
    ],
    coverageDirectory: 'coverage',
    
    // Limit coverage reporters in CI to reduce overhead
    coverageReporters: isCI
        ? ['text-summary', 'lcov'] // Minimal reporters for CI
        : ['text', 'lcov', 'html'], // Full reporters for local
    
    testMatch: [
        "**/__tests__/**/*.(test|spec).[jt]s?(x)",
        "**/?(*.)+(spec|test).[jt]s?(x)"
    ]
};
