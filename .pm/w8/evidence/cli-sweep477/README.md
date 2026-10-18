# Ship held on red main + name namespaces — sweep477

Passing functional coverage, no new product issue; one process outcome recorded first: the requested ship of `w8/m116` + sweeps 465–476 evidence was committed and rebased (`f62d6740e`) but **not pushed** — `scripts/ship-gates.sh` reports `main` red (deploy run [39120931451](https://github.com/bex-co/bex/actions/runs/39120931451) for `e6457568b`, failing step `test-operator / make test (operator + envtest)`, from another worker's log-shipper pin). This loop's commit is markdown-only and does not fix that failure, so per the ship rules the push waits for a green main; nothing was force-pushed or gate-skipped.

Coverage: `postgres list -o text` renders the table (name/project/environment/plan/region/status/ID) truely; resource names are per-kind namespaces — renaming a Key Value onto a live web service's exact name succeeds, and each kind-scoped resolver still finds its own resource by that shared name (`keyvalues get` → the `red-` id; `deploys list` → the service's rows), no cross-kind shadowing.

Cleanup: exact-owned `srv-dbai4fpn2qrc73fmllpg`, `red-dbai4g3c1vqc73a22ddg`, `dpg-dbai4frc1vqc73a22dc0` deleted by ID (404/list absence verified); one baseline delta is concurrent foreign churn. No plan mutations. Session remains active; next inline sweep is 478; `f62d6740e` awaits a green main to push.
