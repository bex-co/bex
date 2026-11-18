# w3 · m472 — Production access from a maintainer's machine pins the control-plane host key and never shows the Hetzner token in `ps`

**Worker:** worker3 **Goal:** `scripts/fetch-app-kubeconfig.sh` works locally on the first try with an authenticated host key, fails at once with the remedy when it cannot, and no production credential appears in a child process's argv. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Local runs pin the control-plane host key from the maintainer's stable key | 45m | — |
| [t002](t002.md) | A trust-on-first-use host-key failure stops at once and names the remedy | 20m | t001 |
| [t003](t003.md) | `HCLOUD_TOKEN` reaches curl through stdin, never argv | 30m | — |
| [t004](t004.md) | A validator refuses a production credential in a curl command line | 40m | t003 |
| [t005](t005.md) | Docs and a local live check | 30m | t002, t004 |
| [t006](t006.md) | Simplify | 15m | t005 |
| [t007](t007.md) | Test coverage | 30m | t006 |
| [t008](t008.md) | Closeout | 10m | t007 |

## Definition of done

- With no `BEX_SSH_KNOWN_HOSTS_FILE` exported and a stale `~/.ssh/known_hosts` entry for a control-plane IP, `bash scripts/fetch-app-kubeconfig.sh /tmp/x.kubeconfig` on a maintainer's machine succeeds. It pins the wildcard line derived from `BEX_CP_SSH_HOST_KEY_FILE` (`StrictHostKeyChecking=yes`, global known-hosts ignored) and never reads or writes `~/.ssh/known_hosts`.
- With neither a pin nor the stable key available, a host-key failure exits non-zero within one pass over the control planes, not after six 60 s retries. The message names `BEX_SSH_KNOWN_HOSTS_FILE`, `BEX_CP_SSH_HOST_KEY_FILE` and ADR019's recovery path.
- While `fetch-app-kubeconfig.sh` and `verify-substrate.sh` call the Hetzner API, no process's argv carries `HCLOUD_TOKEN`. A test with a stub `curl` on `PATH` proves it.
- `scripts/secret-argv-validate.sh` fails on a `curl` line that carries a name from `scripts/production-secrets.txt` or `HCLOUD_TOKEN` in an `Authorization` header argument. It runs in `scripts/ship-gates.sh` and CI, and passes on the tree.
- `cd lego/operator && make test` is unaffected, and `bash scripts/ssh-hostkey.test.sh`, `scripts/fetch-app-kubeconfig.test.sh` and the new validator's test pass.

## Source + Goal linkage

- **Source:** the `/loopx w3` run of 2026-11-16/17 (ADR113's implementation, then the gate-cleared parked items). Its drained report listed findings 1 (this milestone), 2, 3 and 5. `/pmx 1,2,3,5 to w3` on 2026-11-18 filed them after research. Re-checked against `origin/main` `d41bbb9c2`.
- **Evidence, host key:**
  - Every worker in that run that needed production (m439, m441, m302, m303, m318, m366, m367) found `scripts/fetch-app-kubeconfig.sh` failing or "hanging" on stale `~/.ssh/known_hosts` entries. Each borrowed another session's `/private/tmp/*/prod.kubeconfig` after checking its server and CA.
  - The local `.env` sets neither `BEX_SSH_KNOWN_HOSTS` nor `BEX_SSH_KNOWN_HOSTS_FILE` (names-only check, 2026-11-18). So `scripts/lib/ssh-hostkey.sh` falls back to `StrictHostKeyChecking=accept-new` against `~/.ssh/known_hosts`.
  - That file holds pre-rotation keys for reused control-plane IPs (`.pm/w6/blocked/m469/t005.md` names line 73). `accept-new` refuses a changed key, and `fetch-app-kubeconfig.sh:93-125` then retries every host-key failure six times, 60 s apart. A stale local entry never heals by waiting.
  - The authenticated answer already exists. Since w6/m153 t008 every control-plane node serves one stable host key (`BEX_CP_SSH_HOST_KEY_FILE`, `.env.example:49-55`), and the pin is one wildcard line, `* ssh-ed25519 <public key>`. The public half sits at `~/.ssh/bex_cp_host_ed25519.pub` on the maintainer's machine. Other sessions already hand-build that file as a workaround (`.pm/w6/blocked/m469/t005.md`, `m470/t006.md`). The scripts do not derive it.
- **Evidence, token:** `fetch-app-kubeconfig.sh:77` (`discover`) and `verify-substrate.sh:178` (`hc`) pass `-H "Authorization: Bearer $HCLOUD_TOKEN"` on curl's command line. The w3/m302 worker saw it in `ps` output during its run. `HCLOUD_TOKEN` controls the whole Hetzner project: servers, load balancers and volumes. The header comment's "The token is never printed" holds for stdout and is false for the process table.
- **Goal linkage:** production operability and credential custody ([ADR019](../../../docs/ADR019-infra-credentials.md) §Decision 5 and its local access recovery paragraph; [AGENTS.md](../../../AGENTS.md) "Run it before you write 'no production access'"). Every production check a `/loopx` worker runs starts with this script.
- **Expected outcome:** workers and maintainers reach production with one command on an authenticated host key, without borrowing another session's kubeconfig. The Hetzner token stays out of `ps`, shell history and audit logs of process arguments.
- **Why now:** the 2026-11-16/17 run hit the failure on every production-facing item. The borrowing workaround depends on another session having fetched recently, and it spreads cluster-admin kubeconfig copies across `/private/tmp`. The argv exposure happens on every local run.
- **Render parity omitted:** maintainer and CI tooling only; no REST/GraphQL/MCP/UI surface changes.
