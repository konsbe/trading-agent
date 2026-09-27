const isCI = process.env.CI === 'true' || process.env.JENKINS_HOME || process.env.GITLAB_CI;

module.exports = {
    preset: 'ts-jest',
    testEnvironment: 'jsdom',
    bail: isCI ? 1 : 0,
    setupFilesAfterEnv: ['<rootDir>/jest.setup.ts'],

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
        // shared-components sources must use this package's React copy.
        '^react$': '<rootDir>/node_modules/react',
        '^react/(.*)$': '<rootDir>/node_modules/react/$1',
        '^react-dom$': '<rootDir>/node_modules/react-dom',
        '^react-dom/(.*)$': '<rootDir>/node_modules/react-dom/$1',
        '^@trading-agent/shared-components$': '<rootDir>/../../shared-components/src/mfe.ts',
        '^shellSpog/(.*)$': '<rootDir>/__mocks__/shellSpog/$1.ts',
        '^@/(.*)$': '<rootDir>/src/$1',
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
                module: 'commonjs',
                moduleResolution: 'node',
            },
            diagnostics: false,
        }],
        '^.+\\.(js|jsx)$': 'babel-jest',
    },
    transformIgnorePatterns: ['/node_modules/'],
    testPathIgnorePatterns: ['/node_modules/', '/dist/'],

    collectCoverageFrom: [
        'src/**/*.{ts,tsx}',
        '!src/**/*.d.ts',
        '!src/**/*.test.{ts,tsx}',
        '!src/index.tsx',
        '!src/common/webpackUtils/**',
        '!src/test-utils/**',
    ],
    coverageDirectory: '<rootDir>/coverage',
    coverageReporters: isCI ? ['text-summary', 'lcov'] : ['text', 'lcov', 'html'],
};
