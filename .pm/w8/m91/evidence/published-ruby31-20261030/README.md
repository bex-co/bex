# Published Ruby3.1 Bookworm supplier — 2026-10-30

The current immutable supplier is independently approved:

`ghcr.io/bex-co/bex-cnb-run@sha256:5f4dc113a6b3e98217f98b98f99824e8245c74b45dff760aaace3983cecd9bfe`

[Actual run41063733348/job124090808291](https://github.com/bex-co/bex/actions/runs/41063733348/job/124090808291) passed both runtime architectures, complete scans and signing on fresh-build sourcefd358d2010f8b5e2715250d7148a505ba12c1cb6. Producer and verifier share this source; the original official Ruby3.1 base and22 immutable recipe inputs are retained. This is supplier approval only. The production catalog and original generated/hosted acceptance remain unchanged.

The [exact verifier receipt](receipt.json), [root disposition](ROOT-REVIEW.json) and [source policy](source-policy.json) retain five complete strict Cosign3.1.4 bundles, exact image subject/GitHub issuer/workflow/source, Rekor-v2 inclusion and signed RFC3161 timestamps. Both actual wrong-source/wrong-subject controls rejected. The current attempt used seven separate owned TUF caches with unchanged production trust; independent amd64/arm64 SPDX and Trivy reports bind actual config/child/platform/DiffIDs.

| Architecture | SPDX packages | Trivy findings | Critical |
| --- | ---: | ---: | ---: |
| amd64 |755|13,690|228|
| arm64 |753|13,690|228|

Every finding remains in its full signed predicate; neither architecture reports Brotli findings. This is no zero-CVE claim or security waiver. Runtime probes passed separately.

All33 unique actual image blobs have complete SHA/size proofs totaling1,353,900,387bytes. This attempt newly read12blobs/774,272,703bytes and reused21genuine prior full reads/579,627,684bytes with exact custody and current anonymous HEAD checks. The two previously truncated blobs were read from zero and fully matched their descriptors. All12new streams completed on GET1; no live range resumption or historical cause is inferred. No Docker-export alias, manual decompressed-layer proof or retained raw image body is claimed.

[Actual process evidence](engineering/process-exit.json) records exit0 at20:31:07 UTC with no capacity stop, minimum observed free3,156,418,560bytes and maximum owned219,930,888bytes. [The frozen full attempt chain](controls/frozen-complete-attempt-chain.json) retains three rejected attempts: an unclassified size/SHA failure, a TUF/report-pair failure, and two confirmed short bodies. Later correct bytes do not diagnose, accept or waive those responses. The new complete validation proves the current immutable image independently. Three overwritten early derived gzip representations remain an explicit loss; every original signed bundle remains preserved and authoritative.

Root rechecked all175 frozen payload hashes and all five raw/full-gzip/predicate-gzip roundtrips before approval. Full raw bundles and verified gzip remain private and unchanged, alongside public immutable OCI objects. No cleanup/unlink or stale V8 exporter ran. [Full-predicate retrieval metadata](full-predicate-retrieval.json) binds original bundle/signed-statement bytes and complete semantic predicate hashes; compact [copy custody](LOG-CUSTODY.json) identifies retained metadata/source/logs. The tool source/review and11range+4concurrency+6isolation+43strict controls are in [the committed V11 tool capsule](../native-review-ranged-streams-20261030/README.md).

Retrieve and strictly verify full predicates outside the repository:

```sh
python3 .pm/w8/m91/evidence/published-ruby31-20261030/tools/retrieve-and-verify.py \
  --cosign /private/tmp/bex7-native-acceptance-novb586u/cosign \
  --output /tmp/bex-ruby31-full-predicates
```

The separate [fresh reader smoke](retrieval-helper-tests.json) covers the actual small image-signature bundle only; it does not claim to replay all report bodies or hosted acceptance. Eight of27 W8 suppliers are approved. Complete adoption and every original hosted DoD remain pending.
