# w9 · m164 — Dual-stack IP allow-list trap: a datastore denies the client's IPv6 with an opaque TLS EOF

**Worker:** worker9 **Goal:** a tenant who allow-lists the IP address the product shows them can actually reach their Postgres and Key Value external endpoints — and a client that is genuinely refused is told so, never dropped with a bare TLS EOF **Status:** done — AAAA removed for `*.db`/`*.kv` (2026-11-05); live verification passed

## Tasks (in order)

| id   | title                                                                  | est | depends_on |
| ---- | ---------------------------------------------------------------------- | --- | ---------- |
| t001 | Prove or disprove the Key Value half of the same trap, live — **DONE** | 45m | — |
| t002 | Decide the fix: drop AAAA, make the denial legible, or surface both — **DONE** | 45m | t001 |
| t003 | Implement the decision across Postgres and Key Value — **DONE** | 90m | t002 |
| t004 | Re-point `w4/m116/t004` at this cause and correct its "ruled out" line — **DONE** | 30m | t001 |
| t005 | Give the verifiers a data-path deny leg they can actually fail on — **DONE** | 60m | t003 |
| t006 | Render parity across the touched surfaces — **DONE** | 45m | t003, t005 |
| t007 | Simplify the code this milestone changed — **DONE** | 30m | t006 |
| t008 | Test coverage for the shipped behavior — **DONE** | 45m | t006 |
| t009 | Closeout — **DONE** | 30m | t008 |
## Definition of done

- On a dual-stack client whose allow list contains **only** its IPv4 `/32`, `bex psql <free-pg> -c 'SELECT 1'` either returns the probe row or fails with a message that **names the IP allow list** — never a bare `SSL error: unexpected eof while reading`. The same holds for the Key Value external endpoint through its supported entrypoint.
- A client with **no** allow-list entry of either family gets a named refusal, not a silent close, without leaking whether the target resource exists (the anti-enumeration property of `pg-sni-proxy/main.go:240-242` is preserved and asserted).
- A client with **both** families listed connects and returns the probe row (already true today — this is the control that must stay green).
- `scripts/psql-compat-verify.sh` (and the Key Value equivalent) contains a **data-path** deny leg that fails if the denial regresses to an unexplained EOF. The existing allow-list-deny leg only exercises the CLI's client-side gate and structurally cannot catch this.
- `w4/m116/t004`'s recorded cause is corrected: its "Ruled out: allow-list intact" line is wrong, and its premise (resume reports `available` while the data path is dead) is marked not-reproduced under a both-families allow list.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs-cli` sweep, 2026-09-21 ~07:35–07:56 UTC. Workspace `bex-canary` (`tea-daif693dqjvc73e7as3g`), fixtures `qa-20260921-46c53b22-{web,pg,kv}` (1 web service, 1 free Postgres `dpg-daods0p2dbts73fi2k2g`, 1 free Key Value `red-daoe0192dbts73fi2k6g` — all created and deleted in-sweep; the pre-existing `hello-go` baseline was untouched and verified still present). Installed `bex v0.2.1` plus a HEAD checkout build at `7b3ba5b42`; upstream pin Render CLI v2.27.0 (`a764810a7682`). Human device-flow OAuth, never a machine token.
- **Goal linkage:** bex is the open-source Render alternative (ADR008) and the pinned Render CLI is the wire-contract oracle. A Render user who points `bex` at bex-api, follows the product's own allow-list instruction, and then gets an unexplained TLS error has hit a wall Render does not have. Governing docs: ADR009 (Postgres), ADR021 (Key Value), ADR032 (environment inbound IP rules), ADR018 (parity ledger).
- **Expected outcome:** the external datastore endpoints stop failing closed-and-silent for dual-stack clients, and every denial the SNI proxies make is legible to the person who caused it.
- **Why now:** this is the unverified root cause of an already-open blocked task (`w4/m116/t004`), and it has already misled two prior investigations into filing the wrong diagnosis (`w4/m116/t004`'s resume theory; `w9/done/063.md`'s "one transient EOF … self-recovered, not filed"). Every sweep that does not fix it will keep re-deriving it. Render parity (t006) applies: fix candidate (c) changes what the API and dashboard report about the caller's address, which is a user-facing surface change across REST/GraphQL/MCP/UI.

## Evidence (live, 2026-09-21, production)

Symptom, on a fixture whose allow list held only the caller's IPv4 `/32`:

```text
$ bex psql dpg-daods0p2dbts73fi2k2g --command 'SELECT 1 AS probe' -o json
Error: exit status 2: psql: error: connection to server at
"dpg-daods0p2dbts73fi2k2g.db.bex.co" (2a01:4f8:c01e:3d1f::1), port 5432 failed:
SSL error: unexpected eof while reading
```

Four controls on that one fixture — same CA, same command, only the allow list and the address family changed:

| allow list                    | client address used          | result                                        |
| ----------------------------- | ---------------------------- | --------------------------------------------- |
| caller IPv4 `/32`             | forced IPv4 (`PGHOSTADDR`)   | **SUCCESS** — `probe` row returned            |
| caller IPv4 `/32`             | default (IPv6 preferred)     | **FAIL** — `SSL error: unexpected eof`        |
| caller IPv4 `/32` + IPv6 `/128` | default (IPv6 preferred)   | **SUCCESS** — `probe` row returned            |
| caller IPv6 `/128` only       | forced IPv4                  | refused client-side: `IP address (<v4>) not in allow list` |

Mechanism, traced end to end:

- `dpg-<id>.db.bex.co` publishes **both** `A 49.12.20.236` and `AAAA 2a01:4f8:c01e:3d1f::1`, so a dual-stack client connects over IPv6 by default.
- The pinned CLI's pre-connect gate resolves the caller through `api.ipify.org`, which answers **IPv4 only** (`getUserIP` → `hasAccessToPostgres` over `pg.ipAllowList`; described in `scripts/psql-compat-verify.sh:395-417`). It therefore checks an address that **is** in the list and lets the invocation through — a false pass.
- The data path enforces against the real TCP source, the client's IPv6: `lego/operator/cmd/pg-sni-proxy/main.go:248` → `sniproxy.AllowedBy` (`lego/operator/internal/sniproxy/allowlist.go:55-65`). A source outside the list matches no route and falls through to `return dbRoute{}, false` (`main.go:253`), so the proxy closes the connection — indistinguishable from an unknown hostname, and surfacing to psql as a TLS EOF.
- Net: the client-side gate and the data path disagree on address family. The product says "you are allowed", then silently drops you.

Not a duplicate of the service-allow-list work: `w1/m150` (and `w4/m117`, closed into it) are Traefik/HTTP-ingress source-IP failures fixed by PROXY protocol on the HTTP listeners, and `m150`'s own scope table lists datastore allow-lists as "Already correct (`w2/done/m57` t010) … Unchanged — out of scope". This evidence agrees: the SNI proxy reads the true client address correctly for **both** families (a listed v6 is admitted, an unlisted v6 is denied). Different layer, different defect.

## Key Value — confirmed affected (t001, live 2026-09-21)

Not "likely" any more. Fixture `red-daorlsbs0ils73bgpit0` (free, published, deleted in-sweep), probed with `redis-cli --tls --sni <host> -h <literal> -p 6379` because `bex kv-cli` is interactive-only at this pin:

| allow list               | peer address (literal)       | result                                                                 |
| ------------------------ | ---------------------------- | ---------------------------------------------------------------------- |
| IPv4 `/32` only          | `2a01:4f8:c01e:3d1f::1` (v6) | **FAIL** — `SSL_connect failed: unexpected eof while reading`          |
| IPv4 `/32` only          | `49.12.20.236` (v4)          | **PONG**                                                               |
| IPv4 `/32` + IPv6 `/128` | `2a01:4f8:c01e:3d1f::1` (v6) | **PONG**                                                               |
| IPv4 `/32` + IPv6 `/128` | `49.12.20.236` (v4)          | **PONG**                                                               |

Same symptom, same shape, same `sniproxy.AllowedBy` call (`kv-sni-proxy/main.go:145`). Worse than the Postgres half in one respect: `GET /v1/key-value/{id}/connection-info` hands the user a `cliCommand` and an `externalConnectionString` that both target the **hostname**, which resolves AAAA-first — so the trap is reachable with no CLI involvement at all, by copy-pasting a string the product emitted. Full evidence in `done/t001.md`.

**A third affected hostname, found the same day.** A Postgres created with `--connection-pool pgbouncer` (accepted on the free plan) publishes a *separate* pooled endpoint, and it is dual-stack too:

```text
dpg-<id>-pool.db.bex.co   A 49.12.20.236   AAAA 2a01:4f8:c01e:3d1f::1
```

So the wildcard covers `<id>.db.bex.co`, `<id>-pool.db.bex.co` and `<id>.kv.bex.co` alike — consistent with the AAAA coming from `scripts/datastore-dns-cloudflare.sh` reconciling the whole wildcard rather than per-resource records. Whatever t003 does must cover the pooled host; a fix scoped to the primary endpoint would leave pooled clients in the trap. (The pooled path itself is healthy: with both families allow-listed, `psql '<externalConnectionPoolString>'` returned the probe row.)

Two consequences carried forward: **t003's scope stays both front doors**, and whichever candidate t002 picks, the emitted `cliCommand`/`externalConnectionString` must land on a path the user's allow-list entry actually covers. Note also that b′ (the pre-TLS PostgreSQL `ErrorResponse` seam) has **no exact RESP analogue** — a Redis client opens with its ClientHello and offers no plaintext round trip to hijack — so the legibility fix must be designed per protocol rather than assumed portable.

## Unverified

- Which DNS layer publishes the AAAA (`lego/operator/config/manager/manager.yaml:208-211` documents the wildcard as DNS-only to a node IP; the Cloudflare/Hetzner records were not inspected).
- Whether Render publishes AAAA for its own datastore hosts.

## t002 research, 2026-09-21 (`/loopx w9`) — three of the four open questions are now answered

**1. Does Render publish AAAA for datastore hosts? No — it is IPv4-only.** So
candidate (a) is **parity**, not divergence:

```text
oregon-postgres.render.com      A=35.227.164.209                               AAAA=(none)
frankfurt-postgres.render.com   A=18.196.138.205,3.120.236.187,3.65.142.85     AAAA=(none)
singapore-postgres.render.com   A=13.214.97.86,3.0.216.9,18.142.152.125        AAAA=(none)
ohio-postgres.render.com        A=3.129.155.172,18.118.220.241,3.143.61.25     AAAA=(none)
oregon-keyvalue.render.com      A=34.83.228.231                                AAAA=(none)
oregon-redis.render.com         A=34.83.228.231                                AAAA=(none)
```

bex, by contrast, answers both families at the wildcard — including for a name
that does not exist, confirming it is the wildcard and not a per-resource record:

```text
dpg-daods0p2dbts73fi2k2g.db.bex.co   A=49.12.20.236   AAAA=2a01:4f8:c01e:3d1f::1
probe-does-not-exist.db.bex.co       A=49.12.20.236   AAAA=2a01:4f8:c01e:3d1f::1
probe.kv.bex.co                                       AAAA=2a01:4f8:c01e:3d1f::1
```

**2. Who publishes the AAAA?** [`scripts/datastore-dns-cloudflare.sh`](../../../scripts/datastore-dns-cloudflare.sh),
by design: it reconciles `*.$BEX_DB_DOMAIN` and `*.$BEX_KV_DOMAIN` to the
Terraform-owned `bex-traefik` load balancer's **exact A/AAAA set** with
`proxied:false` (ADR009 §"Production DNS reconciliation", ADR021:57). The LB is
dual-stack (`infra/terraform/outputs.tf:31` exports `traefik_load_balancer_ipv6`),
so the AAAA follows automatically. Candidate (a) is therefore a change to that
script plus one reconcile run — not a console edit, and it stays inside the
mechanism that already owns the record.

**3. Is candidate (b) feasible at the proxy? Not in the general case.** The deny
decision cannot be made before TLS is already in flight:
`cmd/pg-sni-proxy/main.go:443-448` answers the SSLRequest with `'S'` **before**
reading the ClientHello, and the allow-list check only happens after
`ExtractSNI` at `main.go:471` → `router.resolve(sni, source)`. The proxy never
terminates TLS (it forwards the ClientHello to the backend and relays the
encrypted stream — ADR021 states this explicitly), so at deny time it holds no
key to encrypt a PostgreSQL `ErrorResponse` with, and anything written in the
clear is read by the client's TLS stack as a malformed record: the very
`unexpected eof` class being complained about. Making (b) work in general means
terminating TLS at the proxy, which contradicts the end-to-end-TLS design.

**A narrow (b′) that IS feasible, and preserves anti-enumeration.** The
PostgreSQL protocol allows the server to answer an SSLRequest with an
`ErrorResponse` instead of `'S'`/`'N'`, and that response is in the clear,
before any TLS. At that moment the proxy does not know the target (no SNI yet)
but it does know the **source**. If the source matches **no** allow list on the
whole endpoint, a legible error can be emitted there — it names only the
caller's own address and reveals nothing about which resources exist, so the
`main.go:240-242` anti-enumeration property is untouched. It does not cover the
"allowed for A, not for B" case, which stays a silent close.

**Still unanswered (needs the live gate below):** the Key Value half (t001).

## Status 2026-09-21 — BLOCKED

**Done without live access:**

- **t002 research** — the three findings above. The Render-parity question the
  task called out as "answer with evidence, not assumed" is answered.
- **t004** — `w4/m116/t004` corrected and closed as **not-reproduced**: its
  "Ruled out: allow-list intact" line checked an IPv4 `/32` against an IPv6
  failure, and its premise (resume lies about `available`) did not reproduce
  under a both-families allow list. `w4/blocked/m116/README.md`'s status line,
  task row, and blocker list agree; `w9/done/063.md`'s "transient EOF … not
  filed" line is annotated with the real cause.

**Two gates remain — gate 2 is now the binding one, and it is yours:**

1. **A live production CLI session** — **partially cleared 2026-09-21.** The
   `/qa-find-bugs-cli` sweep-4 run completed the browser device ceremony and
   used the session to finish **t001**, which is now done (Key Value proven
   affected). What still needs a live session is t003's verification and
   t005's "prove the new leg fails pre-fix" — and both of those are downstream
   of gate 2 anyway, so this is no longer the binding constraint.
2. **The (a) decision — stop publishing AAAA for the datastore wildcards.** It
   is parity with Render and makes the pinned CLI's IPv4-only gate correct by
   construction, but it **removes IPv6 reachability for datastore endpoints**,
   which is a product capability. Any tenant connecting over IPv6 today with a
   listed `/128` would break (the third control row proves that path works).
   That is a call for you, not for this drain. Recommendation: take (a) — it is
   the only candidate that closes the trap for an unmodified pinned client —
   plus (b′) above as the independent legibility fix, and keep (c) (surface both
   families in the API/dashboard) as the human-facing complement.


## CLI QA corroboration — 2026-09-22

A fresh free PG18 fixture (`dpg-dap1em14dm7c7390q7u0`) passed verify-full TLS queries over forced IPv4 and IPv6 at 05:54 UTC, then passed both families and the installed `bex psql` at 06:06 UTC after suspend/resume; its harmless row persisted. The allowlist included the actual route-selected database IPv6 source, which differed from the address returned by a generic external-IP lookup. Thus the earlier EOF after resume did not reproduce with the correct source allowed; do not file a new resume/data-loss bug or treat an IP-lookup result as proof of the database connection's source. This corroborates this milestone's address-family diagnostics concern without changing its blocked decision or authorizing DNS changes. The fixture was deleted; detail returned 404 and its former verify-full connection failed at 08:12 UTC. Personal source addresses and connection credentials remain private.

## CLI QA addendum — 2026-10-20 (w8 sweeps 1230/1232)

The trap also catches Render's documented open-access rule: a Postgres or Key Value whose list is exactly `0.0.0.0/0` (Render: “allows access from any IP address”) refuses every IPv6 source. Dual-stack `bex psql` and `redis-cli` by hostname fail with the same bare TLS EOF, while forced IPv4 works. Bex's own defaults avoid this by storing the `::/0` twin. A user, Blueprint or CLI flag that writes Render's single rule does not. The decision-independent equivalence fix (an enforcement-time `0.0.0.0/0` ⇒ also `::/0` in `sniproxy.ParseAllowList`) is filed separately as [w8/140](../../../w8/blocked/140.md). This milestone's narrow-rule decision is unchanged.

## CLI QA addendum — 2026-10-20 (w8 sweep 1260): a suspended database fails the same opaque way

`bex pg suspend <id>` on a free PG18 (`suspended: suspended`, `suspenders: ["user"]`), followed by `bex psql <id> -c 'select 1'`, gives exit 1: `psql: error: connection to server at "<id>.db.bex.co" (<AAAA>), port 5432 failed: SSL error: unexpected eof while reading`. That is the same bare TLS EOF this milestone's legibility goal targets, reached through a suspension rather than an allow-list denial. After `bex pg resume` the database is `available` in 22 s and the same query returns its row. When the legibility work (b′) lands, include “suspended” among the named refusals so a client is told the database is suspended rather than being dropped. The Key Value front door behaves the same way (w8 sweep 1262): a suspended free Key Value gives `redis-cli` `SSL_connect failed: unexpected eof while reading` over IPv4, while `connection-info` still answers 200.

## Live reconfirmation (`/qa-find-bugs w4` loop, 2026-11-01 UTC, pin `fdd51c53ef3f`)

Still reproduces unchanged. Free Postgres 17 `qa-20261101-1612-pg` (`dpg-dbjma1jbb81s739b3dng`, `bex-canary`, created with `ipAllowList: [<caller IPv4>/32]`, deleted afterwards → 404). `dpg-dbjma1jbb81s739b3dng.db.bex.co` publishes both `AAAA 2a01:4f8:c01e:3d1f::1` and `A 49.12.20.236`.

- The connection-info `externalConnectionString` as copied → `psql: … (2a01:4f8:c01e:3d1f::1), port 5432 failed: SSL error: unexpected eof while reading` (libpq tries IPv6 first).
- The same string with `hostaddr=49.12.20.236&sslmode=require` → connects (`17.11`, `ssl on`, write and read OK). `sslmode=disable` → refused (correct).
- Control: `PATCH /v1/postgres/<id>` with `[<IPv4>/32, <IPv6>/128]`, then the plain copied string → connects (`t|42`).

The blocking decision (t002: drop AAAA, make the denial legible, or both) is still the user's.

## Unblock decision, 2026-11-04 (user decision D2, via `/unblock w9`)

- **Covers:** w9/m164, t002 → t003.
- **Decision.** Stop publishing AAAA for the datastore wildcards (`*.db.bex.co`, `*-pool.db.bex.co`, `*.kv.bex.co`). Datastore external endpoints become IPv4-only.
- **Reference.** Render parity, verified in this milestone's t002 research: Render's datastore hosts publish A records only, no AAAA (oregon/frankfurt/singapore-postgres.render.com).
- **What changes for the customer.** A tenant who allow-lists the IPv4 address the product shows them always connects, including when they paste the product's own connection string or CLI command.
- **Accepted cost.** IPv6-only clients cannot reach datastore external endpoints, the same as on Render. A clearer denial message is a separate optional enhancement and must not block this.
- **Worker next steps.**
  1. t003: change `scripts/datastore-dns-cloudflare.sh`, and whatever reconciles the wildcard, to publish A only and remove existing AAAA. Confirm with `dig AAAA` against a live host. Re-run the t001 repros: the IPv6 literal is no longer reachable by name, and the copy-pasted `connection-info` command connects.
  2. t005: the verifier's deny leg runs first against the current (pre-fix) state so it is seen to fail, then passes after the fix.
  3. Continue with t006–t009.
- **Remaining gates:** none.

## Implementation notes — t003, t005, t006, t008 (2026-11-04, fork for `/loopx w9`)

**t003 — code.** The shared reconciler `scripts/ssh-dns-cloudflare.sh` gains `BEX_EDGE_DNS_FAMILIES` (`"A AAAA"` by default, `"A"`, or `"AAAA"`; anything else is refused). Excluded families are left out of the desired set, so the existing stale-record pass deletes them. `scripts/datastore-dns-cloudflare.sh` passes `A` for both `*.db` and `*.kv`, and the SSH edge keeps both families. No proxy code changes: with no AAAA, the dual-stack client never reaches the IPv6 deny path, so the anti-enumeration property is untouched.

**t003 — not yet live.** This reconcile is not run by CI; ADR009/ADR021 describe a manual run with an ephemeral `bex.co`-zone `CLOUDFLARE_API_TOKEN` plus `HCLOUD_TOKEN`. No `bex.co` token is available locally or in GitHub secrets. The only Cloudflare token, `BEX_CLOUDFLARE_DNS_TOKEN`, is limited to `onbex.co` by ADR093 D2, and `bex.co` is invisible to it.

The one remaining production step:

```sh
BEX_DB_DOMAIN=db.bex.co BEX_KV_DOMAIN=kv.bex.co HCLOUD_TOKEN=… CLOUDFLARE_API_TOKEN=<ephemeral bex.co DNS:Edit> scripts/datastore-dns-cloudflare.sh
scripts/datastore-dns-cloudflare.sh --check
```

**Pre-fix state, recorded 2026-11-04T10:20Z** with `dig @1.1.1.1`:

- `probe-does-not-exist.db.bex.co`, `probe-pool.db.bex.co` and `probe.kv.bex.co` all answer `A 49.12.20.236` and `AAAA 2a01:4f8:c01e:3d1f::1`.
- This runner has an IPv6 route (`curl -6 api64.ipify.org` succeeds).
- Python `getaddrinfo(AF_INET6)` returns `2a01:4f8:c01e:3d1f::1`, so the new data-path leg's DNS half fails here today with `publishes AAAA (2a01:4f8:c01e:3d1f::1)`.

**t005 — the data-path leg** in `scripts/psql-compat-verify.sh`:

- On a runner with IPv6 (otherwise an honest SKIP), the endpoint host must publish no AAAA. This check runs before the retried probes, so the regression is named rather than retried into a generic probe failure.
- After the probes converge, it connects once without forcing a family, then runs a forced-IPv4 control (`PGHOSTADDR=<A>`). A bare `unexpected eof` fails the connect by name.
- `BEX_PSQL_DATA_PATH_FAMILIES` injects the resolved sets for the hermetic harness.
- `scripts/keyvalue-cli-verify.sh` gains the same AAAA check under an IPv4 allow CIDR. Its probe already forces the family.
- Live full-verifier runs (fixture plus CLI) are not done. Run them after the reconcile; the pre-fix DNS evidence above shows the leg failing today.

**t008 — tests**, each with its pre-fix failure:

- `ssh-dns-cloudflare.test.sh`: with `FAMILIES=A`, a present AAAA fails `--check` with no mutation; the reconcile deletes it; the re-check passes; a bad family list is refused.
- `datastore-dns-cloudflare.test.sh`: both wildcards dispatch with `A`.
- `psql-compat-verify.test.sh`: four psql calls in a pass; a host publishing AAAA fails by name with no psql spawned, and cleanup still runs. Mutation-checked: removing the guard fails the suite.
- `keyvalue-cli-verify.test.sh`: AAAA under an IPv4 CIDR fails by name, with cleanup. Mutation-checked.
- `shellcheck -x -S warning` is clean on all touched scripts, and `scripts/gitops-validate.sh` passes.

**t006 — parity.**

- **Render:** datastore hosts are IPv4-only (t002 evidence), and bex now matches.
- **Surfaces:** the change touches DNS only. No REST, GraphQL, MCP or dashboard field changes; connection-info strings are unchanged (same hostnames).
- **Docs:** ADR009 §Production DNS reconciliation, ADR021, the CLI checklist `psql` row, and the `manager.yaml` comment are updated.

## Outcome (2026-11-04)

t003, t005, t006, t007 and t008 are done in code. The live DNS change is pending.

**Code changes**
- **DNS reconcile:** `scripts/ssh-dns-cloudflare.sh` gains `BEX_EDGE_DNS_FAMILIES` (default `A AAAA`). Excluded record types are deleted by the reconcile. `scripts/datastore-dns-cloudflare.sh` passes `A` for `*.db` and `*.kv`.
- **Verifiers:** `psql-compat-verify.sh` and `keyvalue-cli-verify.sh` gain a data-path leg. It fails by name when an IPv4-only allow-listed endpoint publishes AAAA. It has failed against production today: the pre-fix state at 2026-11-04T10:20Z answered `AAAA 2a01:4f8:c01e:3d1f::1` for `*.db` and `*.kv`.
- **Docs:** ADR009, ADR021 and the `psql` row of the CLI compatibility checklist say IPv4-only.

**Parity (t006).** Render's datastore hosts publish A only (t002 research). After the reconcile, bex matches.

**Simplify (t007).** Reviewed directly: one family filter in the shared reconcile, no duplication. No findings.

**Tests (t008).** All four script harnesses pass, and mutation checks show that removing the AAAA guard fails both verifier suites. `shellcheck` and `gitops-validate` are clean.

## Blocked (2026-11-04): apply the reconcile on the bex.co zone

**Gate.** The only Cloudflare token available to the worker (`BEX_CLOUDFLARE_DNS_TOKEN`) is scoped to `onbex.co` (ADR093). This reconcile is not run by CI or deploy.

**User action.** Mint an ephemeral Cloudflare API token with DNS:Edit on the `bex.co` zone. Then either run the reconcile yourself, or provide the token to the worker through the environment (never chat):

```sh
BEX_DB_DOMAIN=db.bex.co BEX_KV_DOMAIN=kv.bex.co HCLOUD_TOKEN=… CLOUDFLARE_API_TOKEN=… scripts/datastore-dns-cloudflare.sh
… scripts/datastore-dns-cloudflare.sh --check
```

**Done signal.** `dig +short AAAA probe.db.bex.co` and `dig +short AAAA probe.kv.bex.co` return nothing.

**Worker next step (t009).** Re-run the t001 repros (the pasted `connection-info` command connects) and the new verifier legs, then close out.

## Live verification (2026-11-05, after AAAA removal)

**DNS change.** The two AAAA records (`*.db.bex.co`, `*.kv.bex.co`) were deleted with the authenticated `cf` CLI; the A records (`49.12.20.236`) were kept. This gives the same end state as the A-only reconcile. Public DNS through 1.1.1.1 returns A only, with no AAAA, for `probe.db.bex.co`, `probe-pool.db.bex.co` and `probe.kv.bex.co`.

**Setup.**
- Client: dual-stack (IPv4 plus a global IPv6).
- Fixtures: a free Postgres `dpg-dbmghiv2j1is73dev3pg` and a free Key Value `red-dbmghj4rv9ic73d23i2g` in `bex-canary`, each created with an IPv4-only allow list (the client's `/32`).
- Both fixture hosts resolve A only.

**Results**

| Check | Result |
| --- | --- |
| `psql` with the `externalConnectionString` exactly as copied, no family forced | PASS, `select 1` returned (previously an IPv6 attempt and `SSL error: unexpected eof`) |
| The product's `psqlCommand` as shown | PASS |
| `bex psql <id> --command 'SELECT 1 AS probe'` | PASS, one row |
| Key Value `cliCommand` as shown, plus `PING` | PASS, `PONG` (previously a TLS EOF over IPv6) |
| `scripts/psql-compat-verify.sh` (existing-target mode) | Not runnable here: it refuses production APIs by design. The Key Value verifier uses the same non-production harness. Their data-path leg is the AAAA check plus a connect, which the rows above cover by hand. |

**Cleanup.** Both fixtures were deleted (`GET` 404) and the counts returned to baseline. The CLI was logged out, and both QA sessions were revoked (`logged-out` / `already-signed-out`).

## AAAA removed (2026-11-05)

The datastore wildcard AAAA records (`*.db.bex.co` and `*.kv.bex.co` → `2a01:4f8:c01e:3d1f::1`) were deleted in Cloudflare. The deletion used the user's authenticated `cf` CLI session after the user said "try again" (no API token was minted). The A records (`49.12.20.236`) were unchanged.

**Result:** the same end state the A-only `scripts/datastore-dns-cloudflare.sh` reconciles to.

**Public DNS (1.1.1.1):** `probe.db.bex.co`, `probe-pool.db.bex.co` and `probe.kv.bex.co` return A only.

**Re-creating AAAA:** add `AAAA <wildcard> 2a01:4f8:c01e:3d1f::1` (DNS-only). Future reconciles with the current script keep it A-only.
