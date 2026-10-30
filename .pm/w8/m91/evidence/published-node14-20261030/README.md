# First published native supplier — Node 14, 2026-10-30

The actual published Node 14 supplier passed identity, runtime and report review on **linux/amd64 and linux/arm64** at `2026-10-30T09:59:53Z`:

`ghcr.io/bex-co/bex-cnb-run@sha256:4fec31cbb186e340cd7ce25ac1d0705f78735631f793b75629d653c6601f83fc`

This is supplier evidence, not production adoption or a security waiver. The production supplier catalog remains empty. Other suppliers, exact generated-image replay (including Python 3.14.8), shared phases and the original fresh two-client hosted PostgreSQL DoD remain open.

## Source and acceptance policy

- The reviewed official base is `node:14-bullseye@sha256:c0bff0d29a742f40650d5f0305dd581351c10954e6cb6676fc96f47590b9666e`.
- Producer `bfdeb62211ae5c4a875f5474a51c63c036e1d4dd` built the exact index. Its earlier scan deadline did not approve it. Verification source `4330878c280bc9748444c8e1e50ff7388df1228a`, [run 41029596739 / job 123976756119](https://github.com/bex-co/bex/actions/runs/41029596739/job/123976756119), completed successfully after rerunning both real runtime architectures, both scans and all signing steps.
- All 22 non-documentation recipe blobs/modes and the exact official base match between producer and verifier. OCI labels bind the producer; certificate policy binds the verification workflow SHA. [Source policy](source-policy.json), [original/current job and identity records](identity/ci-runtime-scan-signing-gates.json) and the [full compact receipt](receipt.json) preserve these separate roles.
- All five immutable bundles passed official Cosign 3.1.4 checks for the exact image-index subject, workflow identity, GitHub issuer, source SHA, transparency inclusion and explicit RFC3161 timestamps. A wrong source SHA failed on the actual certificate. No transparency or certificate checks were ignored.
- Every SPDX and Trivy report independently binds its actual architecture/config/filesystem. Syft 1.51.1's Docker-source root digest is not the registry child digest. [Actual layer proofs](actual-layer-proof/receipt.json) hash the anonymous compressed bytes and decompressed DiffIDs/sizes, then reproduce the exact Stereoscope 0.3.1 Docker-export manifest. The derived amd64 root matches the signed report exactly; architecture union alone is never accepted.
- All 39 unique runtime-image config/layer blobs were fully SHA/size checked anonymously. The final pass rechecked public access to the previously verified layer bytes and read both config blobs. [Parser rejection guards](policy-tests/docker-root-policy-tests.json) reject altered config/child/DiffID/size/index/aliases, foreign roots and cross-architecture roots.

## Findings retained

| Architecture | SPDX packages | Trivy findings | Critical findings |
| --- | ---: | ---: | ---: |
| amd64 | 1,147 | 18,496 | 250 |
| arm64 | 1,144 | 18,495 | 250 |

Both signed SPDX reports identify `libbrotli1` and `libbrotli-dev` as `1.0.9-2+deb12u1+bex11u1`. Trivy still reports `CVE-2026-111497` as affected on the custom backport with no fixed version. Every finding remains in the complete signed reports. The reviewed workflow's actual decoder/runtime gates passed; a scan is not a behavioral probe, and this record claims neither zero CVEs nor a finding waiver.

## Retrieve and verify the complete predicates

[Full-predicate retrieval metadata](full-predicate-retrieval.json) records every immutable manifest/blob digest and size, raw signed-statement SHA256, complete decoded-predicate hash, timestamp/log metadata and private scratch archive identity. [Bundle manifests](bundle-manifests/fallback-referrer-index.json) are small durable metadata; the complete contents total 250,833,715 bytes, preserved in five lossless gzip archives totaling 46,711,707 bytes and immutable public OCI objects. Inactive uncompressed scratch duplicates were removed for host disk recovery only after verifying every gzip and decompressed hash/size; the [cleanup receipt](raw-duplicate-cleanup.json) records this custody change. No complete report body is copied into Git.

Use the independently verified official Cosign 3.1.4 binary and a fresh output directory outside this repository:

```sh
python3 .pm/w8/m91/evidence/published-node14-20261030/tools/retrieve-and-verify.py \
  --cosign /private/tmp/bex7-native-acceptance-novb586u/cosign \
  --output /tmp/bex-node14-full-predicate-replay
```

The helper obtains only anonymous pull authorization, verifies immutable manifest and bundle bytes, runs the strict official bundle verifier, checks the raw signed statement hash, and writes each complete decoded predicate to scratch. Its sorted-JSON hash is explicitly a deterministic semantic-document hash, not a claim about the original scanner's whitespace. Raw bundle and signed-statement hashes bind the original signed bytes. Other platforms must first authenticate their official Cosign binary and supply its verified hash with `--expected-cosign-sha256`.

The native GHCR referrers endpoint returned 404 while its SHA-verified fallback index contained all five bundles. Generic registry discovery timed out or reported no valid bundles. Reading the immutable bundle objects directly bypassed discovery only; strict official cryptographic verification remained mandatory. [Executed tools](executed-tools/) preserve the exact first-review implementation, including its Node 14-specific paths. [Engineering checkpoint](engineering-checkpoint.json) records the bounded transport/schema investigation. No public dummy transparency entry, private key, registry credential, workflow dispatch, catalog change or production mutation was used for this review.
