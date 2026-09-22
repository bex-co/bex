# w9 · m164 — Dual-stack IP allow-list trap: a datastore denies the client's IPv6 with an opaque TLS EOF

**Worker:** worker9 **Goal:** a tenant who allow-lists the IP address the product shows them can actually reach their Postgres and Key Value external endpoints — and a client that is genuinely refused is told so, never dropped with a bare TLS EOF **Status:** blocked (the AAAA decision; t001/t002/t004 done — Key Value proven affected 2026-09-21)

## Tasks (in order)

| id   | title                                                                  | est | depends_on |
| ---- | ---------------------------------------------------------------------- | --- | ---------- |
| t001 | Prove or disprove the Key Value half of the same trap, live             | 45m | —          | — **DONE** (proven affected) |
| t002 | Decide the fix: drop AAAA, make the denial legible, or surface both     | 45m | t001       | — research **DONE**; decision yours |
| t003 | Implement the decision across Postgres and Key Value                    | 90m | t002       | — **BLOCKED** (decision + live) |
| t004 | Re-point `w4/m116/t004` at this cause and correct its "ruled out" line  | 30m | t001       | — **DONE** |
| t005 | Give the verifiers a data-path deny leg they can actually fail on       | 60m | t003       | — **BLOCKED** (needs a pre-fix failing run) |
| t006 | Render parity across the touched surfaces                               | 45m | t003, t005 | — **BLOCKED** (follows t003) |
| t007 | Simplify the code this milestone changed                                | 30m | t006       | — **BLOCKED** (no code changed yet) |
| t008 | Test coverage for the shipped behavior                                  | 45m | t006       | — **BLOCKED** (follows t003) |
| t009 | Closeout                                                                | 30m | t008       | — **BLOCKED** |

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
