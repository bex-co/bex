# w9 · m164 — Dual-stack IP allow-list trap: a datastore denies the client's IPv6 with an opaque TLS EOF

**Worker:** worker9 **Goal:** a tenant who allow-lists the IP address the product shows them can actually reach their Postgres and Key Value external endpoints — and a client that is genuinely refused is told so, never dropped with a bare TLS EOF **Status:** todo

## Tasks (in order)

| id   | title                                                                  | est | depends_on |
| ---- | ---------------------------------------------------------------------- | --- | ---------- |
| t001 | Prove or disprove the Key Value half of the same trap, live             | 45m | —          |
| t002 | Decide the fix: drop AAAA, make the denial legible, or surface both     | 45m | t001       |
| t003 | Implement the decision across Postgres and Key Value                    | 90m | t002       |
| t004 | Re-point `w4/m116/t004` at this cause and correct its "ruled out" line  | 30m | t001       |
| t005 | Give the verifiers a data-path deny leg they can actually fail on       | 60m | t003       |
| t006 | Render parity across the touched surfaces                               | 45m | t003, t005 |
| t007 | Simplify the code this milestone changed                                | 30m | t006       |
| t008 | Test coverage for the shipped behavior                                  | 45m | t006       |
| t009 | Closeout                                                                | 30m | t008       |

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

## Unverified

- The Key Value half. `kv-sni-proxy/main.go:145` calls the same `sniproxy.AllowedBy` and `red-<id>.kv.bex.co` resolves to the same dual A/AAAA pair, so it is very likely affected — but `bex kv-cli` is interactive-only at this pin and the bounded-PTY control was inconclusive. t001 exists to settle it.
- Which DNS layer publishes the AAAA (`lego/operator/config/manager/manager.yaml:208-211` documents the wildcard as DNS-only to a node IP; the Cloudflare/Hetzner records were not inspected).
- Whether Render publishes AAAA for its own datastore hosts.
