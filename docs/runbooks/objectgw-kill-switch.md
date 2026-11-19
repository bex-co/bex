# Runbook: objectgw kill switch (stop public object-storage reads)

**What it is:** `bex-system/bex-objectgw` serves the public `pub-<obk-xid>.<base domain>` hosts of public object-storage buckets, read-only, from the shared Wasabi account. The kill switch is its replica count in the production overlay: set it to `0` in Git and Argo CD stops every public bucket host at once. Design: [ADR112 § D4](../ADR112-object-storage.md#d4--public-access-bex-s3-public-enables-a-read-only-gateway-at-pub-obk-idonbexco). Board: `w21/m417`. **There are two switches**: this one (public `pub-` reads) and the S3 API's, `bex-system/bex-s3gw` (authenticated reads and writes on `objects.<base domain>`, [ADR112 § D3](../ADR112-object-storage.md#d3--the-s3-compatible-api-sigv4-verified-by-bex-the-bytes-streamed-through-bex-s3-and-bex-s3-as-the-command)), set the same way and described under "The S3 API switch" below. Find which one the egress is leaving through before using either.

## When to use it

- `ObjectStoragePublicEgressCritical` fires: more than 50 GB served from the object store this month (about 75% of the shared Wasabi account's free-egress allowance).
- `ObjectStoragePublicEgressHigh` has fired and the Billing and metering dashboard shows public gateway traffic (the "Public gateway bytes out by origin" panel, `origin="wasabi"`) is the growing part.
- Wasabi warns about, or limits, the account.

Why this is serious: Wasabi's egress is free only while monthly downloads stay at or below the active storage volume, and this account also holds the platform's backups and restore paths ([ADR031](../ADR031-platform-data-backup.md)). A limit on the account would reach them. The two alert thresholds (30 GB warning, 50 GB critical, each after five minutes) come from the production walk of 2026-11-16 (`w21/m417` t008): Wasabi's free egress is monthly egress at or below the account's active storage, which measured 67 GB for the whole shared account, so they are about 45% and 75% of that allowance. The caps in `lego/backend/internal/objectgw/limits.go` are sized from the same number (one workspace flat out moves about 30 GB an hour). When the account's storage changes materially, measure it again (below) and reset both thresholds and the sizing test in the same commit.

Check first that public reads are the cause. The kill switch stops only the `pub-` hosts. If the "Object storage egress: month to date" panel keeps climbing while "Public gateway bytes out by origin" is flat, the bytes are leaving through Contract A's presigned downloads (Render's `/v1/objects`) or through the S3 API on `objects.<base domain>`, which objectgw does not serve; this switch does not help. For the S3 API use its own switch below (the lever for one workspace is revoking its access keys, `bex s3 keys revoke`, or its cap or the bucket itself).

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

## The S3 API switch

`bex-system/bex-s3gw` serves `objects.<base domain>` for stock S3 clients (`aws`, `rclone`, `boto3`, `bex s3`), authenticated with an access key, and streams their bytes through the same shared Wasabi account. It has its own switch for the same reason as the gateway above: Argo CD syncs production from `lego/operator/config/prod/kustomization.yaml` with `selfHeal`, so `kubectl scale` is reverted within seconds and is not a kill switch.

**Use it when** `ObjectStoragePublicEgressCritical` fires or Wasabi warns, and the Billing and metering board shows the egress leaving through S3: "S3 bytes in and out" (`direction="out"`) is the growing part while "Public gateway bytes out by origin" is flat; or `S3ListingBytesHigh` and `S3EgressBudgetRefusals` fire together. The per-workspace pacing (8 MiB/s a workspace on each replica) and the monthly egress budget (the larger of the bytes a workspace stores and 5 GiB) are what bound one workspace; this switch bounds the sum, which only a person decides to do.

One reviewed commit: the patch targeting `bex-s3gw` in the same file, next to the gateway's.

```yaml
- target:
    group: apps
    version: v1
    kind: Deployment
    name: bex-s3gw
  patch: |-
    - op: replace
      path: /spec/replicas
      value: 0
```

Merge to `main` and let Argo sync `bex-operator`. The Service, the PodDisruptionBudget, the ServiceAccount and the route stay in place, so a restore is the same one-line change. Uploads in flight are cut: the pods' 35-minute grace period lets a stream finish when the Deployment rolls, but a scale to zero starts it for every pod at once, so announce it.

```bash
kubectl -n bex-system get deploy bex-s3gw   # READY 0/0
kubectl -n bex-system get pods -l app.bex.co/component=s3gw   # none
```

**A client sees** Traefik's own `503` with the plain body `no available server` for every request on `objects.<base domain>` except the `/cloudflare/client/v4/` prefix, which stays on bex-api until w21/m500 removes it. S3 SDKs retry a 503 and then fail with their service-unavailable error, which is the message an operator wants a customer to read during an egress incident; nothing is lost, because the keys, buckets and objects are untouched. **Not affected:** `bex-objectgw` and every `pub-` host, Contract A's routes in bex-api (`/v1/objects`) and, until w21/m500 removes them, the Cloudflare-shaped routes under `/cloudflare/client/v4/`, the hourly usage sweep, the shadow meters and every access key (rows and OpenBao secrets).

**Restore** by setting the value back to `2`, merging, and checking `kubectl -n bex-system rollout status deploy/bex-s3gw`. After it, `bex_s3gw_requests_total{code="OK"}` rises on the "S3 requests by code" panel and `ServiceUnavailable` does not, and `aws s3 ls` with a key answers. Restore only when the egress source is understood: a workspace's key revoked, or its loop stopped.

## What a visitor sees

A request to any `pub-` host still reaches Traefik, which still matches the route (`objectgw-public`) and holds a Service with no ready endpoint. **Measured on production (2026-11-16, w21/m417 t008):** Traefik answers its own `503 Service Unavailable` with `Content-Type: text/plain; charset=utf-8` and the body `no available server`, for the host of an existing public bucket and for any other `pub-` host alike (no objectgw header, no CORS header), from the first half second after the last pod stops. For the first seconds, while the pods are still terminating and Traefik still holds their addresses, a request can instead get `502 Bad Gateway` (body `Bad Gateway`, no content type). It is never objectgw's own `404`, and an exact `404` would need a runtime flag store that the backend does not have. A browser page that fetches the public URL sees a failed request (no CORS headers on either answer).

Plain-HTTP requests are still redirected to HTTPS first (`301`, measured with the pods stopped), by the route's middleware. Every other host (a service, a static site) kept answering `200` during the measurement.

The measurement used `kubectl scale`, which Argo CD's `selfHeal` reverts in 4 to 11 seconds, so it is no kill switch (see above); the answer is the same for the Deployment at `0` in Git, because only the Service's endpoints change. How long Argo takes to apply the commit was not measured.

## What is not affected

- **Contract A's object routes** (Render's `/v1/objects`) live in bex-api and keep working, including presigned reads and writes, and so does the S3 API on `objects.<base domain>` (`bex-s3gw`, which has its own switch, "The S3 API switch" above). The Cloudflare-shaped routes that remain until w21/m500 removes them are bex-api's too and also keep working. Only the `pub-` hosts stop.
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

## Measuring the account's allowance again

Wasabi's rule is monthly egress at or below the account's active storage (its pricing FAQ; it does not say the 1 TB minimum charge counts). To read the active storage, list every bucket of the account with the root credential (`TF_STATE_ACCESS_KEY` and `TF_STATE_SECRET_KEY` in `.env`, never printed) and add up the sizes, for example with the pinned aws-cli image of `scripts/object-storage-s3-credentials.sh`:

```bash
aws --endpoint-url https://s3.wasabisys.com s3api list-buckets --query 'Buckets[].Name' --output text
# for each bucket, against its own region's endpoint (s3.<region>.wasabisys.com):
aws s3 ls s3://<bucket> --recursive --summarize | tail -2
```

On 2026-11-16 the sum was 67.4 GB across 12 buckets. Set the warning at about 45% of the total and the critical alert at about 75%, update `TestTheBandwidthCapsLeaveTheAllowanceAnHourOfWarning` with the new total, and rerun `promtool test rules` over `deploy/gitops/base/rules/object_storage_egress_alerts_test.yml`.
