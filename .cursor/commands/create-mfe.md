Create a new trading-agent micro frontend using the `mfe-creator` subagent and `.cursor/agents/mfe-creator.md`.

Ask for any missing identity: `name`, `displayName`, `port`, `exposedModule`.

- Host: `web-app/ui/spog`
- Remote MFE: `web-app/ui/mfe-<name>`

Use `@trading-agent/shared-components` (Stitch, `web-app/shared-components`). Webpack remote: `spog@http://localhost:3000/remoteEntry.js`.

Always add package-root `README.md`, `Makefile` (`build`, `test`, `run`, `docker`), and `Dockerfile`.
