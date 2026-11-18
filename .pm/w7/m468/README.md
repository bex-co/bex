# w7 · m468 — Sign in with GitLab and Bitbucket, as on Render

**Worker:** worker7 **Goal:** bex offers all four of Render's social sign-in providers. Beside GitHub and Google, "Sign in with GitLab" and "Sign in with Bitbucket" appear on the login, sign-up and account settings pages wherever the installation sets that provider's OAuth client, and nothing changes where it does not. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Sign in with GitLab: Kratos's built-in `gitlab` provider on its own pair](t001.md) | 1h | — |
| t002 | [Choose and record Bitbucket's OIDC bridge in ADR012 §10: Dex's `bitbucketcloud` connector](t002.md) | 45m | — |
| t003 | [Run Dex in the `auth` namespace as Bitbucket's bridge, off until the Bitbucket pair is set](t003.md) | 2h | t002 |
| t004 | [Sign in with Bitbucket: a Kratos `generic` provider at Dex's issuer](t004.md) | 45m | t003 |
| t005 | [The OIDC e2e signs a person up and back in through `gitlab` and `bitbucket` stand-ins](t005.md) | 1h | t001, t004 |
| t006 | [Dashboard: GitLab and Bitbucket buttons with their logos on login, sign-up and account settings](t006.md) | 1h30m | t001, t004 |
| t007 | [Production: sign up and back in with GitLab and with Bitbucket](t007.md) | 45m | t005, t006 |
| t008 | [Render parity](t008.md) | 30m | t006 |
| t009 | [Simplify](t009.md) | 30m | t008 |
| t010 | [Test coverage](t010.md) | 45m | t008 |
| t011 | [Closeout](t011.md) | 10m | t007, t009, t010 |

## Definition of done

- With `BEX_GITLAB_OIDC_CLIENT_ID` and `BEX_GITLAB_OIDC_CLIENT_SECRET` set, the Kratos login, registration and settings flows carry a `gitlab` oidc node labelled "GitLab"; with `BEX_BITBUCKET_OIDC_CLIENT_ID` and `BEX_BITBUCKET_OIDC_CLIENT_SECRET` set, they carry a `bitbucket` node labelled "Bitbucket". Each pair works on its own, beside GitHub's and Google's.
- With neither new pair set, the fragment `scripts/auth-secrets.sh` writes is what it is today (`scripts/kratos-oidc-fragment.test.sh` pins every combination), and no Dex workload runs.
- `scripts/auth-oidc-e2e.sh` signs a new person up and a fresh browser back in through stand-ins registered as `gitlab` and as `bitbucket`, on Kratos v26.2.0.
- The dashboard's login, sign-up and account settings pages show both buttons with the provider's logo (link and unlink rows included) in every locale, and the login and sign-up skeletons match the ready state at 1440 and 390.
- ADR012 §10, ADR018 and ADR027 state the shipped providers and the Bitbucket bridge.
- On production, t007's walk passes: a person signs up with GitLab and with Bitbucket and signs back in with each.

## Source + Goal linkage

- **Source:** promoted from [w7/084](../done/084.md) on 2026-11-17 (found by w7/m465 t009).
- **Render's behavior:** [login-settings](https://render.com/docs/login-settings), fetched 2026-11-17, lists the login methods as "Any of the following account providers:" Google, GitHub, GitLab and Bitbucket, then email and password, then SAML SSO on Scale and Enterprise.
- **bex today:** `scripts/lib/kratos-oidc-fragment.sh:27-28` enables `github` and `google`, each on its own `BEX_*_OIDC_*` pair, and nothing else.
- **Premise correction (why this is a milestone, not the note's hour):** the note said "Kratos v26.2.0 ships `gitlab` and `bitbucket` providers of the same shape." Only GitLab is true.
  - Kratos v26.2.0's `supportedProviders` (`selfservice/strategy/oidc/provider_config.go:186-214`) has `gitlab` and no `bitbucket`. Kratos's `master` has none either.
  - Bitbucket Cloud is OAuth 2.0 only: `https://bitbucket.org/.well-known/openid-configuration` answers 404. Kratos's `generic` provider builds itself from OIDC discovery (`provider_generic_oidc.go`, `gooidc.NewProvider(issuer_url)`), so it cannot reach Bitbucket directly.
  - Bitbucket therefore needs an OIDC bridge. Dex has a `bitbucketcloud` connector, and the repo already runs Dex v2.41.1 as the e2e stand-in. That makes t002–t004 a new platform component, not a config line.
  - Ory Elements 1.2.1's provider logo map (`dist/theme/default/index.mjs`, `logos`) has `gitlab` and no `bitbucket`. The settings page's unlink row reads only that built-in map, so a linked Bitbucket account shows a generic "B" unless the dashboard overrides it (t006).
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)), social login ([ADR012 §10](../../../docs/ADR012-auth.md#10-social-login-sign-in-with-github-and-google-kratos-oidc-w4003-w7m465), [ADR027](../../../docs/ADR027-sso.md)).
- **Expected outcome:** a GitLab or Bitbucket user signs in to bex the way they sign in to Render. ADR018's sign-in providers match Render's four.
- **Why now:** w7/m465 just generalized the fragment to one provider per pair with labels, mappers and a fragment test, and reworked the e2e onto Kratos v26.2.0 with Dex. Adding the next two providers reuses all of it while it is fresh.
- **No user decision gates the code.** Under Render parity first, offering Render's providers is the parity choice, and the bridge is recorded as a recommendation in t002, not a sign-off. A shim in bex-api is the rejected alternative: ADR012 §10 keeps social login "pure Kratos config + Ory Elements rendering — no custom auth code". The one external step is production's OAuth clients: a GitLab application and a Bitbucket OAuth consumer, which only the user can create. t001–t006 ship with both providers off until those exist; if they are still missing at t007, that task is the gate.
- **Out of scope:** self-managed GitLab or Bitbucket Data Center instances (Render's page names the cloud providers only), GitLab and Bitbucket as Git connections for deploys (ADR018's Git connections row, a separate gap), and SAML SSO (`.pm/DO_NOT_DO.md`, Enterprise surfaces).
- **Render parity included:** the change touches the dashboard's login, sign-up and account settings pages. There is no REST, GraphQL or MCP change, because sign-in runs through Kratos's self-service flows.
