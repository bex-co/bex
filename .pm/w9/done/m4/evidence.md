# w9/m4 evidence — cli-compat verify completeness

Recorded 2026-07-15 against `.pm/w9/dev-9/` (rebuilt), bex-api at `a4771886`,
pinned CLI `render-oss/cli @ c23438e`.

## Environment note (why this milestone needed an env fix first)

The shared local `bex` kind cluster ran an **old operator image**
(`bex-operator:dev`, pre-RC5) that stripped `Database.spec.name` on its first
reconcile — so every CLI-created Postgres read its display name back as the
opaque `dpg-…` id, and get-by-name could never resolve. Confirmed by scaling
the operator to 0 (name survived) vs. running it (name wiped). Diagnosis ruled
out apiserver structural-schema pruning, CRD staleness, and CNPG/webhook
mutation. Fix: rebuilt the operator from current source
(`make docker-build IMG=bex-operator:dev`), `kind load`ed it, set the
deployment's `imagePullPolicy: IfNotPresent`, and rolled it. `up.sh` also got a
one-line robustness fix (kind emitted a `0.0.0.0` apiserver address whose cert
is only valid for `127.0.0.1`).

## `scripts/cli-compat.sh verify` — one green run, 52 assertions

`verify: all ✅ rows still hold` (exit 0). Every census family that touches
bex-api is now covered with whole-shape assertions, self-created and
trap-cleaned (no residue verified after the run: no leftover `verify-*`
App/Database/KeyValue CRs, project, or environment):

- session/identity: login, whoami (RC6), workspace current, workspaces, projects
- services: create (RC9 envelope), update, updatedAt advance, instances
  (JSON/text/suspended-`[]`/missing-nonzero), delete
- deploys: create (RC10 clearCache enum), cancel, list (RC2 nested image object)
- logs: name resolution (RC7) + empty-window no-crash (RC8)
- postgres: create `--ip-allow-list` (RC12), list/get-by-name (RC3/RC7), update
  `--plan`/`--disk-size-gb`/`--high-availability`/`--ip-allow-list`/`--clear-ip-allow-list`
  (RC11/RC12), rename with stable `dpg-…` id (RC5), suspend/resume/delete,
  missing-id real message (RC1)
- key value: full create/list/get/suspend/resume/delete (RC3/RC4/RC14)
- environments: seeded project+env decoded through the RC15 cursor envelope,
  unknown-project not-found
- logout: OAuth revoke path (`POST /v1/oauth/revoke`) with a throwaway key;
  the follow-up `client_credentials` grant then fails, proving the Hydra client
  was deleted (the harness key is never touched)

## `scripts/cli-compat.sh mutation-check` — legs proven non-vacuous

`mutation-check: every family's leg failed loudly against its regressed shape`
(exit 0). A response-mutating proxy (`mutation-proxy.py`) reintroduces one
fixed regression per family; the matching verify leg must fail — the official
CLI's own decode errors nonzero, or `checkFields` reports the dropped field:

```
PROVEN: services (RC2 autoDeploy bool) — official CLI rejected the regressed shape (exit 1)
PROVEN: postgres get (RC3 cursor envelope unwrapped) — official CLI rejected the regressed shape (exit 1)
PROVEN: deploys list (RC2 image object -> bare string) — official CLI rejected the regressed shape (exit 1)
PROVEN: logs (RC8 blank nextStartTime crash) — official CLI rejected the regressed shape (exit 1)
PROVEN: keyvalues get (RC14 nested owner/options flattened) — whole-shape check caught dropped field(s)
PROVEN: environments (RC15 cursor envelope unwrapped) — whole-shape check caught dropped field(s)
```

## Reproduce

```sh
bash .pm/w9/dev-9/up.sh
kubectl -n dev-9-auth port-forward service/hydra-public 59090:4444 &
bash .pm/w9/dev-9/bootstrap-key.sh
(cd cli && go build -o ../.pm/w9/dev-9/bin/render .)
scripts/cli-compat.sh verify          # 52 assertions, exit 0
scripts/cli-compat.sh mutation-check  # 6 families proven non-vacuous, exit 0
```
