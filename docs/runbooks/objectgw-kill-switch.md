# Runbook: objectgw kill switch (stop public object-storage reads)

**What it is:** `bex-system/bex-objectgw` serves the public `pub-<obk-xid>.<base domain>` hosts of public object-storage buckets, read-only, from the shared Wasabi account. The kill switch is its replica count in the production overlay: set it to `0` in Git and Argo CD stops every public bucket host at once. Design: [ADR112 § D4](../ADR112-object-storage.md#d4--public-access-dev-url-enables-a-read-only-gateway-at-pub-obk-idonbexco). Board: `w21/m417`.

## When to use it

- `ObjectStoragePublicEgressCritical` fires: more than 900 GB served from the object store this month.
- `ObjectStoragePublicEgressHigh` has fired and the Billing and metering dashboard shows public gateway traffic (the "Public gateway bytes out by origin" panel, `origin="wasabi"`) is the growing part.
- Wasabi warns about, or limits, the account.

Why this is serious: Wasabi's egress is free only while monthly downloads stay at or below the active storage volume, and this account also holds the platform's backups and restore paths ([ADR031](../ADR031-platform-data-backup.md)). A limit on the account would reach them. The two alert thresholds (500 GB, 900 GB) are first values chosen conservatively below the 1 TB account minimum; the production walk's measurement (`w21/m417` t008) resets them.

Check first that public reads are the cause. The kill switch stops only the `pub-` hosts. If the "Object storage egress: month to date" panel keeps climbing while "Public gateway bytes out by origin" is flat, the bytes are leaving through presigned Contract A or Contract B downloads, which objectgw does not serve; the switch does not help, and the lever is a workspace's cap or the bucket itself.

## The change

One reviewed commit to `lego/operator/config/prod/kustomization.yaml`. Argo CD syncs production from that overlay with `selfHeal`, so `kubectl scale` is reverted within minutes and is not a kill switch. Find the patch targeting `bex-objectgw` and set its replicas to `0`:

```yaml
- target:
    group: apps
    version: v1
    kind: Deployment
    name: bex-objectgw
  patch: |-
    - op: replace
      path: /spec/replicas
      value: 0
```

Merge to `main`. Argo syncs `bex-operator` on its own; to hurry it, refresh the `bex-operator` Application in Argo CD. Nothing else changes: the Service, the PodDisruptionBudget and the routes stay in place, so no manifest has to be re-created later.

Confirm:

```bash
kubectl -n bex-system get deploy bex-objectgw   # READY 0/0
kubectl -n bex-system get pods -l app.bex.co/component=objectgw   # none
```

## What a visitor sees

A request to any `pub-` host still reaches Traefik, which still matches the route (`objectgw-public`) and holds a Service with no ready endpoint. Traefik answers its own no-backend response, which I expect to be `503 Service Unavailable` with a plain body. **That is an expectation, not a measurement:** it has not been verified on production. Measuring the real answer is task `w21/m417` t008; replace this paragraph with the measurement when it exists. It is not objectgw's own `404` for a private or unknown host, and an exact `404` would need a runtime flag store that the backend does not have.

Plain-HTTP requests are still redirected to HTTPS first, by the route's middleware.

## What is not affected

- **Contract A and B object routes** (Render's `/v1/objects` and the Cloudflare-shaped `wrangler r2` routes) live in bex-api and keep working, including presigned reads and writes. Only the `pub-` hosts stop.
- Private buckets, bucket settings (the public flag stays set), CORS and lifecycle rules, the hourly usage sweep and the shadow meters.
- Every other platform host: the route is a `HostRegexp` for `pub-` plus a 20-character id, so service hosts, static sites and the activator fallback are unaffected.

## Restore

Set the same patch value back to `2` in a reviewed commit and merge. Do not leave production at `0` after the incident: the file's comment says so, and the buckets' owners see their public URLs fail until it is back.

```bash
kubectl -n bex-system rollout status deploy/bex-objectgw
kubectl -n bex-system get pods -l app.bex.co/component=objectgw -o wide
```

Restore only when the egress source is understood (a runaway bucket made private or removed, or the per-bucket caps raised or lowered) and the month-to-date total is not about to cross the threshold again. The total is month to date: it stops growing while the switch is on and resets on the first of the month.

## What to check afterwards

- The month-to-date panel is flat. It is a total, so "stopped" means the line is level, not that it falls.
- A `pub-` host answers as described above, and a service host (any `<slug>.<base domain>`) is unaffected: `curl -sI https://<slug>.<base domain>/`.
- `kubectl -n bex-system logs deploy/bex-api --since=1h` shows no errors from the object routes.
- After restore, `bex_objectgw_requests_total{result="ok"}` rises on the "Public gateway requests by result" panel, and `unavailable` does not.
- If Wasabi has already warned, the second response is the one ADR112 D1 records: move tenant data to a separate Wasabi account, which changes only the endpoint and credential values. It is a separate task and must not wait for this one.
