# Dashboard search shortcuts — Render's dashboard bundle, 2026-11-22

Render's changelog of 2025-06-26, [Added shortcuts to Render Dashboard search results](https://render.com/changelog/added-shortcuts-to-render-dashboard-search-results): "Jump directly to your API keys, webhook settings, compliance docs, and more—all with a few keystrokes." This capture records the shortcut list from Render's live dashboard, what bex answers for each, and the shortcuts bex leaves out (w4/711).

## Sources

Read without signing in, on 2026-11-22, from Render's public dashboard bundle (`https://dashboard.render.com/` → `assets/index-BR2q8yex.js`) and the URL helpers it imports from `assets/urls-B5ihPFNO.js`. The hashes change with every Render deploy, so find the code by the strings quoted below: the list is the function that returns `{__typename:"Shortcut",id:"overview",…}`, and the matcher is the one beside it that returns `[]` for a query shorter than 3.

## Render's behaviour

- **Where they appear.** The search results open with a **Shortcuts** group, ahead of Services, Projects, Environments, Environment Groups, Blueprints, Webhooks, Workflows and Sandbox Groups. A shortcut row shows its icon, its name and a one-line description.
- **Matching.** The query is trimmed. A query shorter than 3 characters matches no shortcut. Otherwise a shortcut matches when its name, or any of its keywords, contains the query, case-insensitively.
- **Destinations.** An in-app shortcut is a link to a page or to a section anchor of a page. A docs shortcut opens `render.com/docs/…` in a new tab (`target: "_blank"`).

### The list, in Render's order

`/u/<user>/settings` is Render's Account Settings and `/w/<workspace>/settings` its Workspace Settings.

| Shortcut | Description | Keywords | Destination |
| --- | --- | --- | --- |
| Overview | View projects and ungrouped services | dashboard, home | `/` |
| Environment Groups | Share environment variables across services | env, env vars, environment variables, secrets | `/env-groups` |
| Environment variables | Set environment variables and secrets for a service | env, env vars, environment variables, secrets, .env | docs `/configure-environment-variables`, new tab |
| Custom domains | Add a custom domain to a service | domain, custom domain, dns, ssl, certificate | docs `/custom-domains`, new tab |
| Git credentials | Connect GitHub, GitLab, or Bitbucket | git, github, gitlab, bitbucket, repo, repository | `/u/<user>/settings#account-security` |
| Compliance and documents | View compliance and documents | SOC II, SOC 2 Type 2, ISO 27001, ISO, HIPAA, HIPPA | `/documents` |
| HIPAA Compliance | View HIPAA compliance settings | hipaa, compliance | `/w/<workspace>/settings#hipaa-compliance` |
| Audit Logs | Export audit logs | audit, audit logs | `/w/<workspace>/settings#audit-logs` |
| Webhooks | Manage webhooks in Integrations | webhook, webhooks, integration, integrations | `/webhooks` |
| Metrics Stream | Manage metrics in Observability | otel, metrics stream, open telemetry | `/observability#metrics-stream` |
| Log Streams | Manage log streams in Observability | logstreams, log streams | `/observability#log-streams` |
| API Keys | Provision API keys | token, credential, api key | `/u/<user>/settings#api-keys` |
| SSH Keys | Provision SSH public keys | token, credential, ssh key | `/u/<user>/settings#ssh-public-keys` |
| CLI Tokens | Provision CLI tokens | token, credential, cli | `/u/<user>/settings#cli-tokens` |
| Container Registry Credentials | Manage credentials for external images | image, registry, docker, container, credential | `/w/<workspace>/settings#registry-credentials` |
| MCP Server | Manage your Render services from AI apps | mcp, model context protocol, cursor, claude, chatgpt | docs `/mcp-server`, new tab |
| Sandboxes docs (only for a workspace with sandbox groups) | Create a sandbox and run a command in it | sandbox, sandboxes | docs `/sandboxes`, new tab |
| Billing | Manage billing information | billing, payment, subscription, plan, invoice | `/billing` |
| Account settings | Manage your account and profile settings | settings, account, profile, user | `/u/<user>/settings` |
| Workspace settings | Manage team members and permissions | settings, workspace, team, members, users, permissions | `/w/<workspace>/settings` |

The same search also turns a pasted workflow or sandbox id (`trn-`, `tsk-`, `sbx-`, `snp-`, `flw-`, `frn-`) into an "Open …" shortcut, and a `<workflow>/<task>` query into an "Open task" shortcut. Those belong to Render Workflows and are not part of this capture.

## What bex answers

bex's search palette (`dashboard/src/common/components/dashboard-layout/global-search-dialog.tsx`) lists its pages under **Navigation**: the sidebar's destinations plus Workspace Settings and Account Settings. Render's shortcuts join that group (`search-shortcuts.ts`), with Render's keywords and matching rule:

- **A shortcut whose destination is already a Navigation page** lends that page its keywords instead of adding a second row for the same place. Overview is bex's Projects page (`/`, titled Overview), Billing is `/billing`, and Environment Groups, Webhooks and the two Settings pages are pages of their own. "home" finds Projects and "invoice" finds Billing. A page still matches its own name at any length, so the open palette lists every page, as before.
- **A section shortcut** opens the Settings page at the section, under Render's name and description: Git credentials (`/settings#integrations`, the GitHub card; bex connects GitHub only, so the description reads "Connect GitHub"), Audit Logs (`/workspace/settings#audit-logs`), API Keys (`/settings#api-keys`), SSH Keys (`/settings#ssh-public-keys`), CLI Tokens (`/settings#cli-tokens`, the Connected agents card that lists the CLI's grant and revokes it; the description reads "Review and revoke CLI tokens") and Container Registry Credentials (`/workspace/settings#registry-credentials`). Audit Logs is offered to an Admin only, because only an Admin's Workspace Settings has the section.
- **A docs shortcut** opens bex's own page in a new tab: Environment variables (`https://bex.co/docs/platform/secrets`), Custom domains (`https://bex.co/docs/platform/custom-domains`) and MCP Server (`https://bex.co/docs/mcp-server`).
- **Landing.** A Settings page holds the section the URL names at the top while the cards above it load (`useHoldHashTarget` in `section-navigation.tsx`). The router scrolls to the hash once, and on Workspace Settings the Agents cards then shrink by about 2,000 px, which used to leave `#registry-credentials` scrolled past.

Names and descriptions are in every dashboard locale. Render's keywords stay English in every locale, as product terms ("token", "registry", "mcp").

## Omissions

These shortcuts have no bex destination, so bex shows no row for them:

- **Compliance and documents** and **HIPAA Compliance**: bex publishes no compliance documents and has no HIPAA workspace setting. The Documents page waits on the user's decision (`w4/746`), HIPAA-enabled workspaces on `w7/071`.
- **Metrics Stream** and **Log Streams**: external metric and log drains are a non-goal ([`DO_NOT_DO.md`](../../.pm/DO_NOT_DO.md), "External log/metric drains"; [ADR018](../ADR018-render-parity.md) rows _Log streams (external drains)_ and _Metric streams (external)_).
- **Sandboxes docs**: bex's docs have no sandboxes page yet.
