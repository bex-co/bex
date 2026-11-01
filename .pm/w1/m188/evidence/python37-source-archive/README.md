# Published Python 3.7 Bullseye supplier — 2026-10-30

The actual immutable supplier passed **amd64 and arm64** identity, runtime and report review at `2026-10-30T10:37:44.757832+00:00`:

`ghcr.io/bex-co/bex-cnb-run@sha256:41a74a7d36f4b04cd87d8539c74736babb217e51f5dd25b778a957d9288274a0`

This approves supplier evidence only, not production adoption or a security waiver. The production catalog remains empty. Complete catalog review/adoption, exact generated-image replay (including default Python 3.14.8) and original hosted tool acceptance remain open.

## Verified policy

[Source policy](source-policy.json) and the [compact receipt](receipt.json) preserve producer `bfdeb62211ae5c4a875f5474a51c63c036e1d4dd` separately from verification `4330878c280bc9748444c8e1e50ff7388df1228a`. [Actual run 41029603442 / job 123977642604](https://github.com/bex-co/bex/actions/runs/41029603442/job/123977642604) completed both runtime architectures, both scans and all signing gates. All 22 non-documentation recipe blobs/modes and the exact official base match. OCI labels bind the producer; accepted certificates bind the verifier.

All five complete signed bundles passed official Cosign 3.1.4 with the exact image subject, GitHub issuer, workflow identity/SHA, transparency inclusion and explicit RFC3161 timestamps. Wrong-source and wrong-subject controls failed for the expected policy errors. Every SPDX and Trivy report independently binds its actual architecture/config/filesystem. [Actual byte proofs](actual-layer-proof/receipt.json) reproduce each exact Syft Docker-export root from verified config bytes and compressed/decompressed layer hashes/sizes; architecture union alone is insufficient.

| Architecture | SPDX packages | Trivy findings | Critical findings |
| --- | ---: | ---: | ---: |
| amd64 | 717 | 17,655 | 249 |
| arm64 | 714 | 17,654 | 249 |

The signed scans retain the custom Brotli backport CVE-2026-111497 flags. They are not suppressed. Every finding remains in the full signed predicates. Decoder/runtime gates passed separately; scans do not replace those probes and no finding waiver is asserted.

## Full payload retrieval and replay

[Immutable transport metadata](full-predicate-retrieval.json) records all bundle manifests/blobs, sizes, complete signed-statement and decoded-predicate hashes, log/timestamp material and private gzip custody. Full report bodies are outside Git. The [cleanup receipt](raw-duplicate-cleanup.json) verifies lossless archives before removing completed raw duplicates.

Retrieve and strictly verify every complete predicate to a fresh scratch directory outside this repository:

```sh
python3 .pm/w8/m91/evidence/published-python37-20261030/tools/retrieve-and-verify.py \
  --cosign /private/tmp/bex7-native-acceptance-novb586u/cosign \
  --output /tmp/bex-python-3.7-bullseye-full-predicates
```

The helper verifies anonymous immutable bytes, strict official cryptography and complete payload hashes. Its sorted-JSON hash is a documented semantic-document hash, not the original scanner whitespace. Bundle and signed-statement SHA256 values bind original signed bytes. Authenticate another platform's official Cosign binary before supplying its hash with `--expected-cosign-sha256`.

The [executed generic driver](executed-tools/native-supplier-review-v2.py) supports the next reviewed native lines without hardcoded candidate digests. From a checkout, replay with the recorded independently verified Cosign 3.1.4 available at the policy's recorded path:

```sh
python3 .pm/w8/m91/evidence/published-python37-20261030/executed-tools/native-supplier-review-v2.py \
  --policy-module .pm/w8/m91/evidence/published-python37-20261030/executed-tools/native-review-policy.py \
  --name python-3.7-bullseye --sha 4330878c280bc9748444c8e1e50ff7388df1228a --run 41029603442 \
  --repo-root "$PWD" --output-root /tmp/bex-native-review --cache-root /tmp/bex-native-byteproof-cache
```

The driver validates actual CI/source/recovery inputs, directly verifies immutable bundles, derives Docker roots only from actual bytes when needed, and rechecks anonymous config/layer access. It writes full payloads only to scratch. [Engineering evidence](engineering-checkpoint.json), [rejection guards](policy-tests/generic-driver-rejections.json) and byte-identical small crypto logs are retained; `.txt` suffixes avoid the repository's `*.log` ignore and [log custody](LOG-CUSTODY.json) maps original names. No dispatch, cancellation, catalog change, private key, credential copy or production mutation occurred.
