# Published Python 3.8 Bookworm supplier — 2026-10-30

The actual immutable supplier passed **amd64 and arm64** identity, runtime and report review at `2026-10-30T12:34:54.092194+00:00`:

`ghcr.io/bex-co/bex-cnb-run@sha256:3dd6306d5fc472b2dd7814635f933c6318d17754cb6fa94bd129801f8eefcd02`

This approves supplier evidence only, not production adoption or a security waiver. The production catalog remains empty. Complete catalog review/adoption, exact generated-image replay (including default Python 3.14.8) and original hosted tool acceptance remain open.

## Verified policy

[Source policy](source-policy.json) and the [compact receipt](receipt.json) preserve producer `bfdeb62211ae5c4a875f5474a51c63c036e1d4dd` separately from verification `c8ca76473129d092a8d74ada5e09c5c4705987cd`. [Actual run 41033903530 / job 123990866360](https://github.com/bex-co/bex/actions/runs/41033903530/job/123990866360) completed both runtime architectures, both scans and all signing gates. All 22 non-documentation recipe blobs/modes and the exact official base match. OCI labels bind the producer; accepted certificates bind the verifier.

All five complete signed bundles passed official Cosign 3.1.4 with the exact image subject, GitHub issuer, workflow identity/SHA, transparency inclusion and explicit RFC3161 timestamps. Wrong-source and wrong-subject controls failed for the expected policy errors. Every SPDX and Trivy report independently binds its actual architecture/config/filesystem. [Actual byte proofs](actual-layer-proof/receipt.json) reproduce each exact Syft Docker-export root from verified config bytes and compressed/decompressed layer hashes/sizes; architecture union alone is insufficient.

| Architecture | SPDX packages | Trivy findings | Critical findings |
| --- | ---: | ---: | ---: |
| amd64 | 719 | 14,792 | 229 |
| arm64 | 717 | 14,792 | 229 |

The signed scans contain no Brotli findings; SPDX records libbrotli1 and libbrotli-dev 1.0.9-2+deb12u1. All other findings below remain present. This is not a zero-CVE claim. Every finding remains in the full signed predicates. Decoder/runtime gates passed separately; scans do not replace those probes and no finding waiver is asserted.

## Full payload retrieval and replay

[Immutable transport metadata](full-predicate-retrieval.json) records all bundle manifests/blobs, sizes, complete signed-statement and decoded-predicate hashes, log/timestamp material and private gzip custody. Full report bodies are outside Git. The [cleanup receipt](raw-duplicate-cleanup.json) verifies lossless archives before removing completed raw duplicates.

Retrieve and strictly verify every complete predicate to a fresh scratch directory outside this repository:

```sh
python3 .pm/w8/m91/evidence/published-python38-20261030/tools/retrieve-and-verify.py \
  --cosign /private/tmp/bex7-native-acceptance-novb586u/cosign \
  --output /tmp/bex-python-3.8-bookworm-full-predicates
```

The helper verifies anonymous immutable bytes, strict official cryptography and complete payload hashes. Its sorted-JSON hash is a documented semantic-document hash, not the original scanner whitespace. Bundle and signed-statement SHA256 values bind original signed bytes. Authenticate another platform's official Cosign binary before supplying its hash with `--expected-cosign-sha256`.

The [executed generic driver](executed-tools/native-supplier-review-v8.py) supports the next reviewed native lines without hardcoded candidate digests. From a checkout, replay with the recorded independently verified Cosign 3.1.4 available at the policy's recorded path:

```sh
python3 .pm/w8/m91/evidence/published-python38-20261030/executed-tools/native-supplier-review-v8.py \
  --policy-module .pm/w8/m91/evidence/published-python38-20261030/executed-tools/native-review-policy.py \
  --name python-3.8-bookworm --sha c8ca76473129d092a8d74ada5e09c5c4705987cd --run 41033903530 \
  --repo-root "$PWD" --output-root /tmp/bex-native-review --cache-root /tmp/bex-native-byteproof-cache
```

The driver validates actual CI/source/recovery inputs, directly verifies immutable bundles, derives Docker roots only from actual bytes when needed, and rechecks anonymous config/layer access. It writes full payloads only to scratch. [Engineering evidence](engineering-checkpoint.json), [rejection guards](policy-tests/generic-driver-rejections.json) and byte-identical small crypto logs are retained; `.txt` suffixes avoid the repository's `*.log` ignore and [log custody](LOG-CUSTODY.json) maps original names. No dispatch, cancellation, catalog change, private key, credential copy or production mutation occurred.

## Bounded recovery and failed attempts

The [failed-attempt record](failed-attempts/provenance.json), exact historical client sources and byte-identical execution/crypto logs retain the earlier TLS timeouts, public-token expiry evidence, TUF transport failures and truncated gzip stream. None received approval. The final verifier preserves every original source, subject, signature, timestamp, report and byte gate. Positive Cosign retries use identical commands only for observed TUF transport failures; policy mismatches are never accepted.

The final [resource policy](engineering/layer-file-resource-policy.json) caps this case's own compressed layer files at 2 GiB and retains at least 2 GiB free. The client can resume bounded prefixes through exact validated HTTP ranges; interrupted-prefix continuation and rejection paths are exercised by the guard suite. The final live pass did not need a nonzero-prefix continuation. Complete compressed SHA/size checks precede local decompression and diffID verification; only then are files removed. [Cleanup](engineering/own-layer-file-cleanup.json) records 31 files / 1,143,671,089 bytes removed after proof, and the own layer directory is empty. Two earlier genuine complete layer receipts were reused, including the common empty layer. All 35 actual config/layer blobs passed fresh anonymous read/HEAD checks. The final process exited 0.

[43 guard results](policy-tests/generic-driver-rejections.json) and their [exact original source](executed-tools/native-supplier-guard-tests-v8.py) cover invalid identities/cache claims, range and prefix tampering, resource limits, proof-before-cleanup and cancellation of only unstarted local read futures. The source records the original private fixture paths; the corresponding Node 14 byte-proof metadata is preserved in the earlier public capsule. This archive changes no catalog, recovery authorization, workflow, production resource or hosted acceptance status.

The [fresh helper smoke](retrieval-helper-tests.json) retrieves and strictly verifies the actual image-signature bundle with the archived reader. It does not claim to replay all 205 MB of full reports. Run the small check with `tools/test-image-bundle-retrieval.py --cosign /path/to/independently-verified-cosign`; the full replay command above retrieves every predicate.
