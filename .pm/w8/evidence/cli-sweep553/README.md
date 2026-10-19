# CLI sweep 553 — cancel an auto-dispatched update deploy, then recover

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, HEAD-built CLI at `d98d07f62`, production pin still `f4e38371055f`, 2026-10-19T11:59–12:02Z. One owned free web service was created, put through cancel + recovery, deleted by exact ID, and verified absent (public URL `404`, deploy read `404`, list omission).

## Journey

1. Created an echo-image service and waited for `live` (`200`).
2. A combined source update (`--repo … --branch main --root-directory examples/hello-go`, as in sweep 550) auto-dispatched a `service_updated` deploy, `dep-dbb0audk8aos739580lg`.
3. Canceled it by ID immediately: `Deploy dep-… successfully canceled`, status `canceled`. The previous image release stayed `live`/`Running` and kept serving throughout; the saved Git source read back truthfully with `undeployedChanges: true` at `rev-1`.
4. Canceling the same deploy again refused by name: `409: deploy "dep-…" is already canceled`.
5. Recovery: a fresh `deploys create` of the same saved source built `examples/hello-go`, reached `live`, body flipped to hello-go's `OK`, and `undeployedChanges` cleared. A user cancel is therefore a pause on the saved source, not a wedge.

## Notes

- Sweep 550 read `undeployedChanges` as absent immediately after the same combined update; here, after the canceled deploy settled, it read `true` until the recovery deploy cleared it. The difference is timing/state (immediate post-update read vs post-cancel settled read), and both reads were internally consistent; no contradiction was observed within either settled state.
- Cancel during `build_in_progress` (rather than `created`) is the w1/m152 blast-radius territory, covered by that milestone's envtest/live work and not repeated here.
- Deploy lag: production remains behind the w8/129–131 fixes on `main`.
