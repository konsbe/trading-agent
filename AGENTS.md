# trading-agent agent routing

When a prompt matches a row below, **launch that subagent**. Playbooks live in `.cursor/agents/`.

| User intent | Subagent | `subagent_type` |
|-------------|----------|-----------------|
| Create / scaffold an MFE, remote, or `web-app/ui/spog` | MFE creator | `mfe-creator` |
| Build or restyle screens, components, tables, forms, layout, theme, CSS | UI developer | `ui-developer` |
| Write or review React/TypeScript (hooks, types, tests, imports) | React standards | `react` |
| Create / scaffold a microservice under `services/svc-*` | Service creator | `svc-creator` |

## Repo layout

```
trading-agent/
├── auth/                         # Keycloak deployment
├── services/
│   └── svc-<service-name>/       # one folder per microservice
└── web-app/
    ├── shared-components/        # Stitch theme + components (@trading-agent/shared-components)
    └── ui/
        ├── spog/                 # host / shell MFE
        └── mfe-<mfe-name>/       # remote MFEs
```

## Package contract

Every **MFE** (`web-app/ui/spog`, `web-app/ui/mfe-*`) and every **service** (`services/svc-*`) must have, at its own root:

- `README.md` — purpose plus how to build, test, run, and docker-build
- `Makefile` — at least `build`, `test`, `run`, `docker`
- `Dockerfile`

`auth/` is the Keycloak deployment and follows the same README / Makefile / Dockerfile contract.

## Defaults

- Theme and components: Stitch via `web-app/shared-components` (`@trading-agent/shared-components`)
- Design spec: `docs/design-system/` (DESIGN.light.md, DESIGN.dark.md, tokens.json); light/dark applied only by the ThemeProviders
- Host remote: `spog@http://localhost:3000/remoteEntry.js`
- Public npm (`registry.npmjs.org`)
- UI: TypeScript + React 19 + webpack Module Federation
- `nvm use 22` before npm install/start/test

Slash commands: `/create-mfe`, `/create-svc`, `/implement-ui`

## Git: never commit or push without permission

**Do not run `git commit`, `git push`, `git tag`, `git rebase`, `git reset`, amend, or any other history-changing git command (including `scripts/push_via_api.py`) unless the user has explicitly asked for it in the current request.** Finishing a task is not permission. Leave changes uncommitted, report what changed (`git status` / diff summary), and ask. Permission covers only the commit/push the user asked for, not later ones.
