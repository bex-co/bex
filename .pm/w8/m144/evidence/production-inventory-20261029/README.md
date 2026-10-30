# Production disk inventory — 2026-10-29 local / 2026-10-30 UTC

The completed read-only inventory found two App-owned disks with exact App/PVC/PV lifetime joins and no unmatched claims. Both request and report 10Gi, so the inventory contains no existing below-provider-floor claim requiring a size migration. Keep both legacy handles, bindings and tenant data unchanged. This is selected metadata evidence, not a physical inspection of their filesystems or tenant files.

The manager and both API replicas are Ready on the recorded common image. That image has no usable revision label, so the review traced its digest to GitOps writeback d6dd4852a and verified its exact deploy-workflow identity, issuer and original source SHA with Cosign. The strict check succeeded, but the source predates the admitted-capacity writer in `902630131`. New bounded-class activation must wait for a newer Ready operator containing that code. No class or tenant resource changed during this inventory.

## Fresh inventory — run 41003128965

[Read-only run 41003128965](https://github.com/bex-co/bex/actions/runs/41003128965) succeeded on source `1e9f097743c7780eb0e9b48971b503381ec126a6` at 2026-10-30 04:39:14 UTC. Its [selected inventory](inventory-41003128965.json) and [review receipt](receipt-41003128965.json) preserve the original baseline above. Both existing 10Gi App/PVC/PV identities and provider handles are unchanged; there are still exactly two disks, zero unmatched claims and zero requested below-floor disks.

The manager's observed literal `BEX_DISK_STORAGE_CLASS` remains `hcloud-volumes-luks`; the API projection is empty by design. Manager generation 1695 is observed with 1/1 updated/Ready/available replica; API generation 2806 is observed with 2/2. All three selected pods are Running, Ready and nonterminating, with actual image IDs matching the requested digest `sha256:2e9286ad4080f0ff1f607fd1e77515556c9e8f04b60c37956caf4e1d4a964538`.

The [fresh strict signature verification](operator-signature-41003128965-2e9286ad4080.json) again passes the exact `deploy.yml@refs/heads/main` identity, GitHub Actions issuer and source SHA `2ac6fd95402e940ce906873e9c9dc77c1ce53077`. Git ancestry confirms this source does not contain `902630131`. The platform is Ready on the old operator and legacy disk class; bounded-class activation and the owned capacity walkthrough still require adoption of a newer verified operator. This review changed only evidence files and read no Secret data or tenant files.
