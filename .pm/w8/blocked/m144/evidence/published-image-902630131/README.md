# Reviewed bounded CSI artifact

Reviewed 2026-10-29 local / 2026-10-30 UTC. [Factory run 40988719522](https://github.com/bex-co/bex/actions/runs/40988719522) completed successfully for source `9026301316233effaaee71e5ba49730c093bd446`, including the patched-code kernel regression. The candidate tag is `ghcr.io/bex-co/bex-operator:hcloud-csi-9026301316233effaaee71e5ba49730c093bd446`; deployment selects the immutable index in [review.json](review.json).

The review used anonymous GHCR reads with an empty Docker configuration. It recomputed SHA-256 for the index, both Linux platform manifests and their configurations, checked architecture, entrypoint, source/revision/version labels, and verified the exact expected GitHub workflow identity and source SHA with Cosign v3.0.6. The Cosign executable's SHA-256 was `5fadd012ae6381a6a29ff86a7d39aa873878852f1073fc90b15995961ecfb084`, authenticated against the checksum in the pinned installer action `6f9f17788090df1f26f669e9d70d6ae9567deba6`.

The signature and SPDX attestation verification used these constraints with `cosign verify` and `cosign verify-attestation --type spdxjson` against the recorded index:

```text
--certificate-identity=https://github.com/bex-co/bex/.github/workflows/hcloud-csi-image.yml@refs/heads/main
--certificate-oidc-issuer=https://token.actions.githubusercontent.com
--certificate-github-workflow-sha=9026301316233effaaee71e5ba49730c093bd446
```

The verified SPDX 2.3 statement names the index digest as its subject. Its 84 package names include cryptsetup `2.8.6-r0` and e2fsprogs `1.47.4-r0`. The selected package records and canonical statement hash are retained in `review.json`; the complete raw SBOM is omitted from this small review record. This proves the signed index-subject SBOM produced by the workflow; it does not establish independent SBOM coverage of each architecture.

Both exact child digests were anonymously pulled and executed with `--platform linux/amd64` and `--platform linux/arm64`, `--network none`, and argument `-h`. Each container advertised controller/node modes and had inspected exit code 0. The help/pull outputs are retained alongside this record. Pulling the index successively for both architectures hit the local Docker engine's `cannot overwrite digest` error, so the final execution checks use the two child digests already authenticated by the signed index. Each disposable review container was removed.

The checksum-authenticated, repository-pinned hcloud-csi 2.22.0 chart was rendered with the reviewed values and the installer's platform node selector/tolerations. [Actual rendered images](actual-chart-rendered-image-pins.txt) show exactly the reviewed index digest for both controller and node, with no appended tag. The installer protocol test and all eight pin/readiness unit tests passed.

`deploy/hcloud-csi/reviewed.values.json` was written only after these checks passed. Its rollout remains subject to the installer's committed-and-clean guard. No production Helm release, StorageClass default, existing PVC/PV, provider volume or tenant file was changed by this review. Production driver readiness, operator adoption, existing-disk inventory/migration and the original real capacity/grow/remount/snapshot/metrics/billing/surface acceptance remain open in m144.

The archived help text normalizes indentation only, preserving command/option text. Raw stdout remains under `/tmp/loopx-w8-csi-raw-help/`; execution results and authenticated image identities are unchanged.
