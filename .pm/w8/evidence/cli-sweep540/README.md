# CLI sweep 540 — Key Value update dialects

**Verdict: no bug filed.** Same isolated session, `bex-canary`, HEAD-built CLI, 2026-10-19T10:43–10:45Z. One free Key Value created → `available` → update probes → deleted by ID (named not-found verified). No connection credentials printed.

| probe | observed result |
| --- | --- |
| `--plan bogusplan` | Exit `1`, server `400: invalid request body at /plan` (client passes plan through; server names the field). |
| `--plan 5g` | Exit `1`, `400: plan must be one of free\|starter\|standard` — schema admits the spec name, bex's semantic layer refuses by name with its real catalog (same two-layer dialect as the blueprint probes in sweep 520). Critically, no paid plan was ever accepted anywhere in this sweep. |
| `--ip-allow-list cidr=999.1.1.1/32,…`, `notcidrformat` | Exit `1` **client-side** with the exact parse fault (`invalid CIDR address`, `must start with cidr=`); no request sent. |
| `--ip-allow-list … --clear-ip-allow-list` together | Exit `1` client-side: mutually exclusive, as documented. |
| `--persistence-mode off` | Exit `0`; status `config_restart` → `available` within ~20s with `persistenceMode: off` read back — a real restart, not a paper update. |
| `--plan free` (same plan, no-op) | Exit `0`, stays `available`, no spurious restart. |
| Readback after all refusals | Plan/policy/allow-list unchanged (`free`, `allkeys_lru`, `[]`) — every refusal was side-effect-free. |

Cleanup: exact-owned `red-dbav8um05jqs73cjk3ag` deleted by ID; re-`get` answers the named not-found.

- Unrun: `--plan starter` upgrade leg (paid — out of hunt rules), rename collision against an existing Key Value name (create-side 409 covered in sweep 502), update by name under `--environment` narrowing.
- Deploy lag means deployed revisions can precede local `main` (production pin `8e57d4eaa121` at sweep time).
