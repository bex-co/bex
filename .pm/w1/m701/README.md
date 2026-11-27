# w1 · m701 — The Environment page recommends Managed OIDC when it sees long-lived OpenAI, Anthropic, AWS or Google Cloud keys, as Render's does

**Worker:** worker1 **Goal:** a service or environment group holding a long-lived provider key shows Render's dismissible recommendation to switch to Managed OIDC, linking to bex's setup page, for each provider bex's Managed OIDC delivers. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Record Render's OIDC recommendation in `docs/render-artifacts/service-environment-page.md`; amend ADR116 D8 and ADR018 | 45m | — |
| [t002](t002.md) | Port Render's detection, tested with Render's own patterns | 1h | t001 |
| [t003](t003.md) | bex-api says which providers the dashboard may recommend | 45m | t001 |
| [t004](t004.md) | The banner and its per-provider dismissal in the Environment editor | 1h15m | t002, t003 |
| [t005](t005.md) | The banner's copy in eight locales | 30m | t004 |
| [t006](t006.md) | A public OIDC setup page on bex.co/docs with Render's four anchors | 1h | t001 |
| [t007](t007.md) | Turn each provider on once its delivery passes live | 20m | t004, t006 |
| [t008](t008.md) | Walk on dev-1 and production | 45m | t005, t007 |
| [t009](t009.md) | Render parity | 20m | t008 |
| [t010](t010.md) | Simplify | 15m | t009 |
| [t011](t011.md) | Test coverage | 30m | t009 |
| [t012](t012.md) | Closeout | 10m | t010, t011 |

Total: **12 tasks, about 7h35m**.

## Definition of done

1. The detection matches Render's rules. Tests run Render's key-name and value patterns verbatim, including:
   - the exclusions (Azure keys, admin keys, OpenRouter and Groq values);
   - a Google Cloud service-account JSON in a value or a secret file;
   - AWS session credentials suppressing AWS.
2. A service's Environment Variables and Secret Files, and an environment group's page, show Render's banner, with Render's title, text and link labels, for each enabled provider found. Static sites and the build source's tab never show it.
3. Dismissing hides the banner for those providers in that browser, in every tab, until a provider not yet dismissed is found.
4. The banner's link opens bex's public OIDC setup page at the provider's anchor.
5. All four providers are enabled, each only after its delivery passed live with the real provider: AWS (w2/m477), Anthropic and OpenAI (w2/m478), Google Cloud (w2/m479).
6. ADR116 D8 and ADR018's Managed OIDC row say the dashboard recommends OIDC.
7. Walked on production at 1440 and 390 px, in all eight locales.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w1` (2026-11-26), item 2, verified before filing.
  - Render's public dashboard bundle was read 2026-11-26 without sign-in (entry `index-D5V4ZgbE.js`, 459 chunks). Chunk hashes change with every Render deploy, so find the code by the names and strings quoted here.
  - **The banner:** the `VariablesActionButtons` chunk's banner (`kn`) and detection (`Sn`).
    - Title "Use OIDC for more secure authentication".
    - Text "You're using long-lived {providers} {API keys|keys}. For better security, we recommend switching to Render-managed OIDC to use short-lived credentials that rotate automatically.", with "API keys" only when every provider found is OpenAI or Anthropic.
    - The link reads "Set up OIDC" for one provider and "Set up {provider}" for each when there are several. It opens `https://render.com/docs/oidc#{aws|gcp|anthropic|openai}-setup`, because Render's docs helper prefixes `https://render.com/docs`. Render's dashboard has no OIDC page of its own.
  - **Dismissal:** remembered per provider in `localStorage` (`render-oidc-recommendation-dismissed`), synced across tabs through the `storage` event. While any provider found is undismissed, the banner lists every provider found.
  - **Gating:** only `isStatic`. No flag, experiment or plan check, so Hobby workspaces see it although Render's Managed OIDC needs Pro or higher.
  - **Where:** a service's Environment Variables and Secret Files, the env group page, and the linked env groups' read-only tables on the service page. bex shows linked groups as key badges, not tables, so that last place does not apply.
  - **Key names:**
    - `(?:^|_)OPENAI(?:_[A-Z0-9]+)*_(?:KEY|TOKEN|SECRET)$`
    - `(?:^|_)(?:ANTHROPIC|CLAUDE)(?:_[A-Z0-9]+)*_(?:KEY|TOKEN|SECRET)$`
    - `AWS_(?:ACCESS_KEY_ID|SECRET_ACCESS_KEY)$`
    - `GCP`, `GOOGLE` or `GCLOUD` names ending in `CREDENTIALS`, `SERVICE_ACCOUNT`, `SERVICE_ACCOUNT_KEY` or `SA_KEY`, optionally `_JSON`
  - **Values:** `^sk-ant-[a-z]+\d{2}-[\w-]{20,}$` (Anthropic), `^sk-(?:(?:proj|svcacct)-)?[\w-]*T3BlbkFJ[\w-]*$` (OpenAI), `^AKIA[0-9A-Z]{16}$` (AWS), and a JSON with `"type": "service_account"` and `"private_key"` (Google Cloud). A secret file matches by that JSON or by a service-account, `gcp` or `gcloud` `.json` name.
  - **Exclusions:** `AZURE_` keys and `ADMIN` in an OpenAI or Anthropic key name are ignored. Admin keys (`sk-admin-`, `sk-ant-admin`), `sk-or-` and `gsk_` values count as other providers, and a value's class beats its key name. An `AWS_SESSION_TOKEN` key or an `ASIA…` value suppresses AWS.
- **bex today (checked 2026-11-26):**
  - **Managed OIDC status:** the issuer has been on production since 2026-11-18 (w2/m476, done), and build time is done (w2/m480, verified against a MinIO STS).
    - AWS, Anthropic and OpenAI, and Google Cloud delivery are on production, but no real provider has accepted a bex token yet.
    - [w2/m477](../../w2/blocked/m477/README.md), [w2/m478](../../w2/blocked/m478/README.md) and [w2/m479](../../w2/blocked/m479/README.md) wait on accounts the user creates (m477 t006, m478 t004, m479 t003).
  - **The dashboard and the design:** nothing in the dashboard mentions OIDC.
    - [ADR116 D8](../../../docs/ADR116-managed-oidc.md#d8--no-new-surface) says bex adds no dashboard field for OIDC "as Render has none". A banner is not a field, but D8's "the only tenant-facing surface" sentence needs amending.
    - ADR018's Managed OIDC row ([identity-workspaces-account.md](../../../docs/ADR018/identity-workspaces-account.md)) is ✖ in every cell.
  - **No setup page:** bex.co/docs has no OIDC page (`/docs/platform/oidc` and `/docs/oidc` answered 404 on 2026-11-26).
    - That site is built from the private repository `bex-co/eden-cms-v2`, which a worker may push to only with the user's permission. [w4/730](../../w4/blocked/730.md) and [w21/008](../../w21/blocked/008.md) wait on the same permission.
    - The only setup text bex has is [`examples/oidc-ai/README.md`](../../../examples/oidc-ai/README.md), for Anthropic and OpenAI.
  - **The editors:** the service, env group and build source editors share `EnvironmentEditor` (`dashboard/src/features/services/components/service-environment-editor.tsx:367`, `dashboard/src/features/env-groups/components/env-group-editors.tsx:35`, `dashboard/src/features/build-sources/components/build-source-environment-tab.tsx:110`).
  - **The build source:** w2/m480 kept build sources (`bds-…`) out of Managed OIDC, so the build source's tab must not recommend it. Record that exclusion in ADR018.
- **Not a duplicate of:** w2/m476–m480, which deliver OIDC and add no dashboard surface. [w1/154](../blocked/154.md), which covers the same chunk's Datastore URL picker. The chunk's External URL warning is w1/m699.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)). Recommending OIDC also points users at the short-lived credentials bex already issues.
- **Expected outcome:** a user who pastes a long-lived OpenAI, Anthropic, AWS or Google Cloud key is pointed, as on Render, at the short-lived credentials bex issues, and at a page that shows how to set them up.
- **Why now:** the detection, the banner and its copy (t001–t005) do not depend on either gate, so they are built while the provider milestones wait. The banner then turns on provider by provider. It never points at a path no provider has accepted, or at a page that does not exist.
- **Gates:** both need a person, so the milestone parks after t005 with these named.
  - t006 needs the user's permission to push the bex.co docs site's source (`bex-co/eden-cms-v2`).
  - t007 needs w2/m477–m479's live tests, which need the user's cloud accounts.
- **Render parity task:** included (t009). The dashboard gains the banner. REST, GraphQL and MCP gain only the bex-only list of providers t003 serves, which Render has no counterpart for.
