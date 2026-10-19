# CLI sweep 545 — nonexistent `--root-directory` fails by name

**Verdict: no bug filed.** Same isolated session, `bex-canary`, HEAD-built CLI at `5ae176831`, 2026-10-19T11:02–11:04Z.

One repo-backed free web fixture created with `--root-directory examples/nope-zzz-545` (a path absent from `bex-co/bex`):

- Create accepts (exit `0`) — like a bad branch (sweep 528), path existence is only provable after clone.
- The deploy settles `build_failed` in ~1 minute (mostly queue): `failureReason` and build logs both carry `the root directory "examples/nope-zzz-545" does not exist in the repository at "5ae176831…"` — the operator's source-path check naming the path and the exact commit, in the clone step, before any tenant build time is burned.

Deleted by exact ID (`srv-dbavhmfmnaes738pfp9g`).

Board state at sweep time: all four loop filings are now fixed on `main` — w8/131 landed as `5ae176831` ("a missing Docker Hub image settles at the pull-failure grace") — while the deployed image pin remains `8e57d4eaa121`, so 129/130/131 live verifications all queue behind the next pin.

- Unrun: `--root-directory` with `..` traversal (refused at create by `ValidRootDir` — read in code, not re-probed), root directory that exists but lacks a Dockerfile under the Docker runtime.
- Deploy lag means deployed revisions can precede local `main`.
