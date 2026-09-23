module.exports = function (api) {
    const isDevelopment = api.env("development");

    return {
        presets: [
            ["@babel/preset-env", { targets: "defaults" }],
            ['@babel/preset-react', { runtime: 'automatic' }],
            "@babel/preset-typescript",
        ],
    };
};
