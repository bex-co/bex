# PRFAQ — bex v1.0 launch (hosting)

**Status:** Draft for review · 2026-08-20 · Owner: TPM/PM **Scope:** the `bex/v1.0.0` platform train ([ADR058](ADR058-release-engineering.md)) — hosting only. Cloud coding-agent sessions and agent sandboxes (pillar 5) are explicitly **not** part of this launch. **Placeholders:** launch date and customer quote are TBD and marked `[…]`.

---

## Press release

### bex 1.0: git push to production, on infrastructure you control, at 30% less

**The open-source Render alternative reaches 1.0 — web services, static sites, cron jobs, managed Postgres and Key Value, custom domains with automatic TLS, and push-to-deploy from GitHub — self-hostable on a €10 server or your own Kubernetes cluster, and API-compatible with Render so your existing tooling, blueprints, and even Render's official CLI just work.**

San Francisco — `[launch date]` — Today the bex project announced bex 1.0, the first generally available release of its open-source cloud application platform. bex gives developers the experience that made Heroku, Render, and Railway beloved — connect a repository, `git push`, get a running HTTPS service — as Apache-2.0 software they can run themselves, or as a hosted service at bex.co priced 30% below Render across compute, databases, and build minutes, and 90% below on bandwidth.

## Problem

Developers don't want to operate Kubernetes; they want a URL. But until now that experience could only be rented: closed platforms, someone else's cloud, someone else's egress markup, and no exit that doesn't mean rebuilding your deployment story from scratch. The alternative — running Kubernetes yourself — trades a monthly bill for a part-time job. Teams have been forced to choose between convenience they don't own and ownership they can't afford to operate.

## Solution

bex removes the choice. One Go operator and one API turn a declarative app description into running, health-gated, TLS-terminated services on any Cluster API-provisioned cluster — a single Hetzner box, a hundred of them, or a laptop running the local development cluster. Every capability is available three ways from day one: a Render-compatible REST API, GraphQL, and an MCP server so AI agents can deploy, inspect, roll back, and scale services as first-class operators rather than screen-scraping a dashboard. Because bex speaks Render's API shapes and its `render.yaml` blueprint format, existing toolchains transfer instead of restarting: Render's own official CLI runs unmodified against bex.

> "Platforms like Render proved what developers actually want, and then locked it behind a metered door," said `[spokesperson, title]`. "bex 1.0 is the same product as software: the code that runs our hosted cloud is the code you can run tonight. If we ever disappoint you, your exit is `git clone`, not a migration project."

## Try it out now

Getting started takes minutes. On hosted bex.co: sign up, verify your email, add a card, connect your GitHub account, and deploy — from a repository with a `render.yaml`, or straight from an existing Render blueprint. Self-hosting: apply the versioned install artifact to your cluster and operate the identical platform, with a documented upgrade path and version-skew policy from 1.0 onward. Either way you get **web** and **private services**, **static sites**, **cron jobs**, **managed PostgreSQL** with automated backups and point-in-time recovery, managed Key Value (Valkey), custom domains with automatic certificates, environment groups and secrets, live logs and metrics, workspace roles for teams, SSH and a browser shell into running instances, auto-sleep for idle services, and metered usage with a real-time cost estimate on every surface.

"`[Customer quote — design partner: migrated N services from Render in an afternoon by pointing the same render.yaml at bex; cut hosting bill by X%; kept their existing CLI scripts.]`" said `[customer name, title, company]`.

bex 1.0 is available today. The platform is Apache-2.0 on GitHub; hosted bex.co is open for signup. Documentation, the migration guide, and the install artifact are at `[docs URL]`.

---

## External FAQ

**Q: What exactly can I host on bex 1.0?** Five service types, matching Render's model: **web services** (public HTTPS on `<name>.onbex.co` or your custom domain), **private services** (internal-only, addressable as `<slug>:<port>`), **static sites** (build → object-storage origin with redirects, rewrites, and custom headers), **cron jobs** (schedule → managed runs with history, manual trigger, and cancel), and **background workers**. Plus two managed datastores: **PostgreSQL** (plans, public or internal endpoints, automated backups, point-in-time recovery, disk autoscaling) and **Key Value** (Valkey, Redis-compatible, nightly snapshots on paid plans).

**Q: How compatible with Render is it, really?** bex serves Render's REST API shapes (verified against Render's OpenAPI spec), reads unmodified `render.yaml` blueprints as its canonical deploy contract, and passes Render's **official, unmodified CLI** — bex maintains a public per-command compatibility checklist and a capability-by-capability parity ledger with evidence for every row. Where bex extends beyond Render (usage API, cost estimates, MCP), those are additive. Known divergences are documented, not hidden.

**Q: How do I migrate from Render?** Point bex at the same repository and the same `render.yaml`. Blueprints compile fail-closed: anything bex can't honor is refused up front with a named error rather than silently dropped. Environment variables and secrets import through the same env-group model; custom domains move by re-pointing DNS after bex issues certificates. Your Render CLI scripts and API integrations keep working against bex's endpoint.

**Q: What does it cost?** Hosted bex.co prices at **30% below Render** on workspace plans (Pro $17.50/mo vs $25), compute, Postgres, Key Value, and build minutes, and **90% below on bandwidth** ($0.015/GiB vs $0.15/GB) — self-hosted-class egress economics, passed through. Usage is metered hourly and every surface (dashboard, REST, GraphQL, MCP) shows an itemized month-to-date cost estimate. Self-hosting is free forever: Apache-2.0, no open-core gates — the hosted product runs this repo's code.

**Q: Is hosted bex.co free to try?** Hosted bex.co is a paid product: signup is card-free, but a payment method must be bound before deploying — including free-tier resources. If you'd rather not add a card, the self-hosted platform is the first-class free path, not a demo. (A dedicated no-card demo mode is on the roadmap.)

**Q: What do I need to self-host?** A Kubernetes cluster — bex provisions its own via Cluster API (Hetzner supported first; a Docker-based local cluster for development) or runs on one you bring. The install artifact ships with the 1.0 release train; from 1.0, releases carry upgrade paths, CRD/database migrations, and a documented version-skew policy.

**Q: Can AI agents use bex?** Yes — that's a founding thesis. Every action a human can take is an API call; state is machine-readable (`phase` / `revision` / `url`); and the full verb set (deploy, status, logs, metrics, rollback, suspend, scale, env vars) is exposed over MCP with OAuth 2.1, so Claude Code, Cursor, and similar tools operate bex natively. Note the distinction: v1 makes bex _operable by_ your agents; it does not include bex's own hosted coding-agent product (see below).

**Q: What's not in 1.0?** The pillar-5 agent products — **hosted coding-agent sessions and E2B-style agent sandboxes** — are in development but excluded from this launch. Also out of scope: multi-cloud abstraction layers, enterprise SSO/SAML/SCIM, and Render-style Workflows. Deploy previews per pull request are `[confirm status before publishing]`.

**Q: How is my app isolated from other tenants?** Each workspace gets its own Kubernetes namespace with default-deny network policy, per-tenant resource quotas, per-workspace registry credentials and ACLs, secrets in OpenBao (never in git or the database), and optional image signing with admission verification. The platform has been through **eighteen consecutive external security-review rounds**, each triaged in a public ADR with per-finding dispositions and regression tests — the audit trail ships in the repo.

**Q: Why should I believe the platform is durable?** Because the exit costs nothing. The hosted service and the self-host artifact are the same code at the same version; your app definition is a portable `render.yaml`; your data has documented backup and restore runbooks with recorded drills. bex competes on price and openness precisely because lock-in isn't the business model.

---

## Internal FAQ

**Q: Why launch hosting-only and hold back the agent products?** Two different maturity bars. Hosting is parity-ledgered, security-reviewed through round 18, billed end-to-end through Stripe, and exercised by the official Render CLI as a fifth compatibility surface. Agent sessions are functional but still moving fast (hibernation, archive, model-proxy custody all landed within weeks) and would drag an unstable contract into a 1.0 promise. Per ADR058, 1.0 _means_ upgrade obligations; we take them on only for surfaces we're prepared to keep stable. The agent products launch on their own announcement when they clear the same bar — and "AI-native" still headlines v1 truthfully via the MCP/API-first story.

**Q: What precisely does "v1.0" commit us to?** The ADR058 train: one shared version `bex/v1.0.0` covering operator + bex-api + ssh-gateway + dashboard + the self-host install artifact; tag-only version source; immutable tag-triggered releases with checksums and keyless signing; cross-version upgrade paths (CRD schema + control-plane DB migrations) and a kubectl-style ±1-minor skew policy. The CLI leaves `bex-cli/v0.x` and joins the shared number. Our own cluster keeps deploying continuously from `main` by digest; the train casts tested digest sets into versions for external operators and must not slow internal cadence.

**Q: What are the launch gates?**

1. **Self-host artifact** exists, installs on a fresh cluster from docs alone, and upgrades from a prior RC (the ≈1.0 bar from ADR058).
2. **Open-signup preconditions** from ADR075: email verification enforced at login, `BEX_REQUIRE_PAYMENT_METHOD=all` live, the onboarding continuity fixes shipped — enabled _before_ external users exist, so nothing is grandfathered.
3. **Billing end-to-end** on real Stripe: metered usage → sealed export → invoice, webhook intake verified, tax gate configured or explicitly deferred.
4. **Migration-gated security deferrals resolved or consciously carried**: the ADR074 workspace-scoped artifact-identity runbook (ADR055 F2/F3) should complete before external tenants share the registry/S3 namespaces — this is the highest-risk open item and needs an explicit go/no-go.
5. **Parity ledger and CLI checklist re-verified** at the release-candidate digest set; divergences published.
6. **Restore drills current** for etcd, OpenBao, Postgres, and Key Value per ADR031's re-drill cadence.

**Q: Is the pricing sustainable?** The 30% discount is a deliberate wedge priced off Render's public sheet (snapshot-based; we re-verify on their changes). The 90% bandwidth discount reflects genuine unit economics — Hetzner-class egress vs. cloud markup — not a subsidy. Margin risk concentrates in support and free-tier abuse; the card-required gate (ADR046/075) blunts the latter by design. The estimate surface is advisory; Stripe is authoritative, so price-sheet changes never corrupt invoices.

**Q: What are the top launch risks?**

1. **Shared-namespace artifact identity** (ADR055 F2/F3): pre-ADR074 Apps share registry/S3 identity scoped by app name. Mitigation: complete the migration runbook pre-launch (gate 4).
2. **Compatibility drift**: Render evolves its API/CLI after our snapshot. Mitigation: the pinned upstream CLI + checklist re-run per release; parity ledger owns divergences.
3. **Single-operator custody**: `.env` credential custody and several runbooks assume one trusted operator (ADR079 #1 residual). Acceptable at launch scale; must be revisited before team growth.
4. **Support surface**: self-host users will file issues against clusters we can't see. Mitigation: install doctor/diagnostics in the artifact; docs-first support policy; known-good Hetzner reference path.
5. **Accepted residuals** (onbex.co not on the Public Suffix List; remaining digest-pinning inventory) are documented accepted risks — restate them in the security docs at launch rather than letting a researcher "discover" them.

**Q: How do we measure success?**

- **Activation funnel** (ADR075): signup → email verified → card bound → **first successful deploy**, with time-to-first-URL as the headline metric (target: minutes, median).
- **Migration proof**: Render blueprints deployed unmodified; CLI-checklist pass rate at 100% of supported rows.
- **Durability**: self-host installs completing from docs alone (tracked via issue rate per install); upgrade success across the first minor.
- **Economics**: gross margin per workspace on hosted; free-tier-only workspace ratio (card gate makes this a real signal).
- **Trust**: continued external security-scan cadence with zero unresolved highs at each release.

**Q: What are we deliberately not doing at v1?** Per the standing non-goals: no multi-cloud abstraction, no enterprise SSO/SAML/SCIM, no Workflows (ADR033 records the design but does not authorize it), no OpenChoreo/Korifi adoption (ADR070), and no un-gated free hosted tier. Look-and-see evaluators get self-host or the future demo mode — we do not soften the card wall to inflate signups.

**Q: What's the post-v1 sequence?** v1.x hardens the hosting core (upgrade paths proven in the wild, deferred security migrations closed, demo mode). The next launch-worthy announcement is the pillar-5 agent product line — coding-agent sessions, sandboxes with hibernation — which by then rides an established release train and inherits a security posture with eighteen-plus review rounds behind it.
