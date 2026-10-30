# Published Python 3.9 Bookworm supplier — 2026-10-30

Actual immutable candidate reviewed at `2026-10-30T15:24:44.738273+00:00`:

`ghcr.io/bex-co/bex-cnb-run@sha256:6b4ec79defb88e6646bc0bb6038b2d162761decc930d22aaf812e68098ed609b`

This capsule approves supplier identity/runtime/report evidence only. It changes no production catalog, recovery authorization or hosted acceptance status. Complete review and adoption of all 27 lines and generated exact-image replay (including default Python 3.14.8) remain separate.

## Source and cryptography

[Actual run 41063720126 / job 124090465681](https://github.com/bex-co/bex/actions/runs/41063720126/job/124090465681) passed both runtime architectures, both scans and the signing gate. [Source policy](source-policy.json), [exact executed receipt](receipt.json) and [producer/reviewer lineage](identity/producer-verification-provenance.json) bind producer `bfdeb62211ae5c4a875f5474a51c63c036e1d4dd` separately from verifier `fd358d2010f8b5e2715250d7148a505ba12c1cb6`. All 22 non-documentation recipe blobs/modes and the exact official base match. OCI labels retain the producer; every accepted certificate binds the verifier.

All five complete signed bundles passed official Cosign 3.1.4 with the exact image subject, GitHub issuer, workflow identity/SHA, Rekor v2 transparency inclusion and signed RFC3161 timestamps. Actual wrong-source and wrong-subject controls rejected. Complete byte-identical command results and logs are retained in [crypto](crypto/) and [log custody](LOG-CUSTODY.json).

## Independent architecture reports and actual bytes

| Architecture | SPDX packages | Trivy findings | Critical findings |
| --- | ---: | ---: | ---: |
| amd64 | 719 | 12,638 | 184 |
| arm64 | 717 | 12,638 | 184 |

Every finding remains in the complete signed predicates; [per-architecture summaries](source-policy.json) include the actual Brotli findings without suppression. This is no zero-CVE claim or security waiver. Runtime decoder/transformation checks passed separately; scans do not replace them.

Each registry-source SPDX report directly binds its actual candidate architecture/config/child. Each Trivy report independently matches the immutable index, actual config, platform and full filesystem DiffID list. No architecture-union acceptance was used. [Archived configs/manifests](identity/) retain the exact source and candidate identities.

All 35 unique actual image config/layer blobs were read anonymously and fully SHA/size checked, totaling 1,212,994,198 bytes, with no prior blob receipt reused. [Exact blob provenance](anonymous-image-blob-provenance.json) records the original full-read timestamps and descriptors. Official ancestry uses the verified config DiffID prefix, with descriptor variance recorded where applicable.

No Docker-export alias was needed. No manual decompressed-layer hashing was performed or claimed. Compressed image bytes were streamed and discarded after hashing; complete raw image layers were not retained. [Engineering evidence](engineering-checkpoint.json) and the [actual zero-exit receipt](engineering/process-exit.json) preserve that scope.

## Full predicates and replay

[Immutable full-predicate transport metadata](full-predicate-retrieval.json) records public OCI bundle manifests/blobs, original signed-statement hashes, complete decoded-predicate hashes, log/timestamp material and private gzip custody. Complete raw signed bundles persist privately as lossless gzip and public immutable OCI blobs. [Raw-duplicate cleanup](raw-duplicate-cleanup.json) verifies full decompressed SHA/size equality before removing only completed own raw duplicates. No full report body enters Git.

Freshly retrieve and strictly verify all complete predicates outside this repository:

```sh
python3 .pm/w8/m91/evidence/published-python39-20261030/tools/retrieve-and-verify.py \
  --cosign /private/tmp/bex7-native-acceptance-novb586u/cosign \
  --output /tmp/bex-python39-full-predicates
```

The decoded-predicate sorted-JSON hash is a semantic-document hash; original bundle and signed-statement SHA256 values bind original signed bytes. Authenticate another platform's official Cosign binary before supplying its expected hash. The small [fresh reader smoke](retrieval-helper-tests.json) retrieves/verifies the actual image-signature bundle only; it does not claim to repeat the full report replay.

The [unchanged executed generic driver](executed-tools/native-supplier-review-v8.py), [canonical policy](executed-tools/native-review-policy.py), [43 prior rejection guards](policy-tests/generic-driver-rejections.json) and exact [guard source](executed-tools/native-supplier-guard-tests-v8.py) remain archived. Replay with separate private output/cache paths:

```sh
python3 .pm/w8/m91/evidence/published-python39-20261030/executed-tools/native-supplier-review-v8.py \
  --policy-module .pm/w8/m91/evidence/published-python39-20261030/executed-tools/native-review-policy.py \
  --repo-root "$PWD" --name python-3.9-bookworm --sha fd358d2010f8b5e2715250d7148a505ba12c1cb6 --run 41063720126 \
  --output-root /tmp/bex-python39-review --cache-root /tmp/bex-python39-private-cache --layer-workers 2
```

The guard source records its original private fixture paths; earlier approved capsules retain the corresponding public byte-proof metadata. [Attempt provenance](review-attempt-provenance.json) and full small execution/crypto logs preserve actual internal retries and controls without claiming retries were absent. No dispatch, cancellation, credential copy, catalog change or production mutation was performed by this review.
