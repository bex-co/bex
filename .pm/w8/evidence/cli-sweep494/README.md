# CLI sweep 494 — rename round trip, ship closed out

**Verdict: no bug filed.** One free web service was created, renamed, and deleted by ID with a HTTP `404` absence proof; the baseline was restored exactly.

## Ship close-out (this sweep)

Deploy run `39120931451` concluded `success` (the Docker Hub TLS flake diagnosed in sweep 487 did not recur on the re-run; all test jobs green, build + deploy completed). `bash scripts/ship-gates.sh origin/main` returned `ship-gates: ok` (exit 0), and the held stack was pushed: the rebased m116 filing plus evidence commits through sweep 493, landing at `bba498efd` on `origin/main`.

## Journey

| step | observed result | assessment |
| --- | --- | --- |
| `bex services update <id> --name …-renamed` | Exit `0`; API read shows the new `name` with the original `slug` unchanged | Render-consistent: the onrender subdomain (slug) is immutable after creation; only the display name changes. |
| Guessed `--autoscaling-*` flags | `unknown flag` at exit `1`; no autoscaling flags exist on `services update --help` | The pinned CLI simply has no autoscaling surface here; harness guess, not a defect. (Per the loop's standing rule, no plan mutation was attempted in its place.) |

## Safety and unrun checks

- No plan mutation; only the exact owned ID was deleted; secrets unrecorded.
- Deploy lag means deployed revisions can precede local `main`.
