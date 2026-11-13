# w4 · m357 — A new public Postgres keeps its private-CA certificate: the copied external URL carries `sslmode=verify-full` and fails on a stock client ("root certificate file … does not exist"), a regression of w2/m245's publicly trusted URL

**Worker:** worker4 **Goal:** a new public Postgres serves a publicly trusted certificate within minutes of becoming Available, and its connection info is again Render's `postgresql://user:pass@<id>.db.bex.co:5432/db` with no `sslmode` and no CA, so a copied URL connects from any stock client. While issuance is pending, the user is told so. A new public Key Value gets its TLS listener (and so any external access) within the same window; round 254 showed it has none at all while issuance stalls. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Diagnose on production why a new Database's public certificate never issues | 30m | — |
| [t002](t002.md) | Restore issuance for new public databases, with a guard so QA churn cannot exhaust it again | 1h30m | t001 |
| [t003](t003.md) | Say on the Connections card, REST and GraphQL when the publicly trusted certificate is pending or failed | 45m | t001 |
| [t004](t004.md) | Render parity | 20m | t002, t003 |
| [t005](t005.md) | Simplify | 15m | t004 |
| [t006](t006.md) | Test coverage | 30m | t004 |
| [t007](t007.md) | Closeout (live replay on production) | 20m | t005, t006 |

## Definition of done

All on production, workspace `bex-canary`, on a Free Postgres created from New → New Postgres with Public access on (the default):

1. **Trusted within minutes.** No later than 5 minutes after the database reads Available, `openssl s_client -starttls postgres -connect <id>.db.bex.co:5432 -servername <id>.db.bex.co` shows a Let's Encrypt issuer (or the chosen public CA) with `Verify return code: 0`. Round 253 still showed `issuer=OU=<workspace>, CN=<id>` 18 minutes after creation.
2. **Render's connection info.** `GET /v1/postgres/<id>/connection-info` returns `externalConnectionString` with no `sslmode` and no `serverCaCertificate`, and `psqlCommand` is `PGPASSWORD=… psql -h <id>.db.bex.co -U <user> <db>`. GraphQL agrees.
3. **A copied URL connects.** Copy External connection string, then `psql "$(pbpaste)" -c 'select 1'` on a machine with no `~/.postgresql/root.crt` and no `PGSSLROOTCERT` returns `1`. Round 253 got "root certificate file "/Users/…/.postgresql/root.crt" does not exist".
4. **Pending is visible.** Between Available and trusted, the Connections card says a publicly trusted certificate is being issued and how to connect meanwhile. REST and GraphQL carry the state.
5. **Key Value too.** A new public Key Value's external endpoint accepts TLS with a publicly trusted certificate no later than 5 minutes after it reads Available: `redis-cli --tls --sni <id>.kv.bex.co -h <id>.kv.bex.co -p 6379 PING` answers `NOAUTH Authentication required.`, and a copied External URL answers `PONG`. Round 254's store had no TLS listener at all 10 minutes after creation (`SSL_connect failed: unexpected eof while reading`). Round 251's store, created at 01:57, connected within a minute.
6. **Fixtures.** Every fixture is deleted, and each GET answers 404.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs w4` loop, round 253, 2026-11-13 02:23–02:42 UTC (`muse.env`, workspace `bex-canary`; pin `8acf1d116941`). The probes are under § Finding.
- **Governing docs:** [ADR009](../../../docs/ADR009-postgresql-management.md) § Publicly trusted server certificates (w2/m245).
  - "Production sets `letsencrypt-prod`", so "a copied external URL connects with no CA file", as Render's do (`frankfurt-postgres.render.com`, Let's Encrypt).
  - The private-CA contract is for local and dev clusters, or while `status.externalCertificate` is `Pending`.
  - The same section warns: "Let's Encrypt allows 50 new certificates per week per registered domain (`bex.co`) … A `*.db.bex.co` wildcard would lift the bound, but it needs a Cloudflare DNS-01 token … file it as a follow-up if the budget binds."
- **Goal linkage:** Render parity on a core datastore journey (ADR008, ADR018 Postgres row). A user moving from Render pastes the external URL into an app. With `sslmode=verify-full` and a private CA, node-postgres, pgjdbc, Prisma and libpq without a downloaded CA all refuse.
- **Expected outcome:** every new public database connects from a copied URL, as on 2026-11-12 (round 210), and a pending certificate is never silent.
- **Why now:**
  - Major. It breaks the first connection of every new public database since at least 02:23 today.
  - The QA loops create public databases every round, which is exactly the churn ADR009's budget note predicted.
- **Render parity task included (t004):** connection info and the database read are REST, GraphQL, MCP and dashboard surfaces.

## Finding (round 253)

**Fixture:** Free Postgres `qa-20261113-r253-pg` (`dpg-dbr79akhd5js73aueo20`), created from New → New Postgres at 02:23:38, Database name `qa_r253`, user `qa_r253_owner`, Public access on, PostgreSQL 18. It was Available at 02:25:27 and deleted at 02:42 (`DELETE` 204, then `GET` 404).

- **Connection info** (Reveal, then `GET /v1/postgres/dpg-dbr79akhd5js73aueo20/connection-info` at 02:26:32):
  - `externalConnectionString` = `postgresql://qa_r253_owner:…@dpg-dbr79akhd5js73aueo20.db.bex.co:5432/qa_r253?sslmode=verify-full`, and `serverCaCertificate` is present.
  - The psql command reads `PGPASSWORD=… psql 'host=dpg-dbr79akhd5js73aueo20.db.bex.co port=5432 dbname=qa_r253 user=qa_r253_owner sslmode=verify-full'`.
  - The card offers "Download CA certificate" with `PGSSLROOTCERT="/path/to/dpg-dbr79akhd5js73aueo20-ca.pem"`.
- **Served certificate:** `openssl s_client -starttls postgres` → `issuer=OU=tea-daif693dqjvc73e7as3g, CN=dpg-dbr79akhd5js73aueo20`, the same as the downloaded CA's subject.
  - It did not change at any of 18 checks from 02:27:15 to 02:41:11, roughly every 30–45 s. The last check was 17.5 minutes after creation.
- **Client results** (local libpq 18.6):
  - The copied URL as is → `root certificate file "/Users/tianpan/.postgresql/root.crt" does not exist / Either provide the file, use the system's trusted roots with sslrootcert=system, or change sslmode…`.
  - With `PGSSLROOTCERT` set to the downloaded CA: `qa_r253_owner|qa_r253|PostgreSQL 18.6 …`, `CREATE TABLE`, `INSERT 0 2`, `2`.
  - The same URL with `?sslmode=require` → `ok-require`.
- **The regression window.** [w4/610](../610.md)'s round 210 (2026-11-12 16:56–17:10 UTC, pin `77531cc764d3`) recorded `psql "$(pbpaste)"` connecting straight from the copied External connection string, and a psql command of the Render form `PGPASSWORD=<pw> psql -h <id>.db.bex.co -U qa_r210_user2 qa_r210`. That is the `PubliclyTrusted` contract.
- **The code path.** `externalURLQuery` (`lego/backend/internal/postgres/service.go:1185-1186`) returns the Render form only when `d.Status.ExternalCertificate == PubliclyTrusted`. Otherwise it appends `sslmode=verify-full` and the CA. The operator sets `PubliclyTrusted` once the `<db>-public-tls` Certificate is Ready, its chain verifies and CNPG serves it (`lego/operator/internal/controller/database_public_tls.go:154-210`, `:313-331`), and reports failures as the `PublicCertificateReady` condition (`:97-100`, `:210-217`). Neither the REST read nor GraphQL `Database` exposes that state (REST keys: `public` only; GraphQL: `Cannot query field "externalCertificate"`).
- **No code change explains it.** `git log` shows no change to `database_public_tls.go` or `postgres/service.go` between the two pins. `BEX_DB_CLUSTER_ISSUER` is still set in `lego/operator/config/prod/kustomization.yaml:111`.

## Unverified

- **The cause.** No production cluster access this round (no kubeconfig), so the Certificate, Order and Challenge, and the Database's `PublicCertificateReady` reason, were not read. Candidates, in order:
  1. Let's Encrypt's 50-per-week limit for `bex.co`, exhausted by QA churn. Every round creates a public Postgres or Key Value, each a new certificate.
  2. The HTTP-01 solver route for `dpg-*.db.bex.co` on port 80. `curl http://dpg-dbr79akhd5js73aueo20.db.bex.co/.well-known/acme-challenge/x` answered `404 page not found`, which cannot tell a missing solver from an unknown token.
  3. The operator never creating the Certificate.
- **crt.sh** returned `[]` for `%.db.bex.co`, and its Key Value entries stop at 2026-10-07, though round 251's store verified. Its index lags, so it could not count the week's issuance.
- **Key Value as a control.** Round 251's `qa-20261113-r251-kv` (created 01:57) connected with `redis-cli --tls` and no CA file, so a `*.kv.bex.co` certificate issued 26 minutes before this database's did not. That fits candidate 1 (the budget ran out between them) as well as a Postgres-only fault.

## Round 254 corroboration — Key Value is hit harder (2026-11-13 02:47–02:58 UTC, pin `8acf1d116941`)

Two fixtures were created by REST at 02:47:04: Free Postgres `qa-20261113-r254-pg` (`dpg-dbr7k9qshdos73f2f4f0`), and Free Key Value `qa-20261113-r254-kv` (`red-dbr7ka4hd5js73aueoag`, `ipAllowList: [0.0.0.0/0]`, `public: true`). Both were deleted at 02:58 (`DELETE` 204, then `GET` 404). Each endpoint was polled with `openssl s_client` every ~30 s.

| UTC | Postgres issuer | Key Value issuer |
| --- | --- | --- |
| 02:47:14–02:48:48 | none (still creating) | none |
| 02:49:19 → 02:57:32 | `OU=tea-daif693dqjvc73e7as3g, CN=dpg-dbr7k9qshdos73f2f4f0` (private CA), 16 checks | **no peer certificate** (`SSL routines::unexpected eof while reading`), 15 checks |

- **The Key Value read `status: available`, `public: true`** with an `externalHost`, and its dashboard reads Public access Yes. Yet `redis-cli --tls --sni red-dbr7ka4hd5js73aueoag.kv.bex.co … PING` failed at every attempt with `SSL_connect failed: unexpected eof while reading`, the same bare close as a blocked allowlist.
  - Re-saving the allowlist from the dashboard (adding `::/0`, toast "Allowlist updated.") at 02:55:24 changed nothing over the next 66 s. The REST create path is not the cause.
- **Why Key Value has no fallback.** [ADR021](../../../docs/ADR021-keyvalue-management.md) (lines 92–97): the external listener opens only once cert-manager has written the `<id>-kv-tls` Secret ("Once cert-manager has written the Secret, the operator … runs `CONFIG SET tls-cert-file …`"). With issuance stalled, a new public store's external access is simply off, while every surface says it is public. Postgres at least falls back to its private CA.
- **Control: existing certificates still serve.** The workspace's older `qa-20261110-81b5-kv` (`red-dbpf5q5me2kc73f2l0og`, not created by this run; only its public endpoint's certificate was read) presents `issuer=C=US, O=Let's Encrypt, CN=YR2`, `notBefore=Nov 10 09:36:13 2026 GMT`. So serving works, and **new issuance** for `*.db.bex.co` and `*.kv.bex.co` is what broke. That happened between round 251's store (01:57, trusted within a minute) and round 253's database (02:23).
- **What it does to the cause list.** Both datastore domains fail together, which points away from a Postgres-only operator fault (candidate 3) and toward the shared issuance path: the `bex.co` Let's Encrypt budget (candidate 1) or the cluster's ACME/HTTP-01 solver (candidate 2). t001 still has to read the Orders.
- **Scope added to the tasks:** t002 covers Key Value issuance with the same fix, and t003 also shows the pending state on the Key Value Connections card, REST and GraphQL. A pending public store must not read as reachable.
