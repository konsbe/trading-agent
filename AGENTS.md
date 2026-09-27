# trading-agent agent routing

When a prompt matches a row below, **launch that subagent**. Playbooks live in `.cursor/agents/`.

| User intent | Subagent | `subagent_type` |
|-------------|----------|-----------------|
| Create / scaffold an MFE, remote, or `web-app/spog` | MFE creator | `mfe-creator` |
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
    ├── spog/                     # host / shell MFE (sibling of ui/, not inside it)
    └── ui/
        └── mfe-<mfe-name>/       # remote MFEs
```

## Package contract

Every **MFE** (`web-app/spog`, `web-app/ui/mfe-*`) and every **service** (`services/svc-*`) must have, at its own root:

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

## Education content: review mfe-education at the end of every change

When a change alters what a user sees or how something is computed (a label, threshold, data source, screen, field or section), check `shared/content/handbook.json` / `masterclass.json` (served by `web-app/ui/mfe-education`) before calling the work done: update or add the affected Handbook entry so it matches the live app, or state explicitly in the report that no Education change is needed and why. Every subagent playbook in `.cursor/agents/` carries the same rule.

## Git: never commit or push without permission

**Do not run `git commit`, `git push`, `git tag`, `git rebase`, `git reset`, amend, or any other history-changing git command (including `scripts/push_via_api.py`) unless the user has explicitly asked for it in the current request.** Finishing a task is not permission. Leave changes uncommitted, report what changed (`git status` / diff summary), and ask. Permission covers only the commit/push the user asked for, not later ones.
