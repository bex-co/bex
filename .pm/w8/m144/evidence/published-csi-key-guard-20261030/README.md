# Published missing-key CSI guard — 2026-10-30

[Factory 41071547850](https://github.com/bex-co/bex/actions/runs/41071547850) succeeded at source `15ed18b94a75d5d7705f6ce195cb6caae80cb5fe`, including the real encrypted bounded-filesystem kernel test (22 seconds). The reviewed index is `ghcr.io/bex-co/bex-operator@sha256:e9516afe323745061edebf8e075a8922b904f9e412e29448b136c227e34b0a7c`; its source contains exact patch `035bcc3eca78ac1a6f1520ef863cabab66567dc7583fe53eb5b9de550df4d3ec`.

Ten anonymous raw index/child/config hashes, both architecture/source/version labels and original-base DiffID prefixes passed. Cosign 3.1.4 verified the exact CSI-workflow identity, GitHub issuer and source SHA. The signed SPDX 2.3 subject matches this index; its 84-package inventory covers amd64/x86_64 only. Both child images were separately pulled with empty authentication and actually executed for driver help, cryptsetup, required blkid flags and dumpe2fs, with all eight commands exiting zero. Owned probe resources were removed.

The committed controller/node pin is updated to this qualified digest. Actual scoped production rollout and old-driver termination are still required; no paid fixture or new platform Ready observation is established by publication. Raw small verification results are retained losslessly with original/stored hash custody.

The committed pin passed the existing eight pin/readiness unit cases, installer protocol checks and complete client-side GitOps validator. The validator explicitly skipped its optional FGA CLI model-drift check because the CLI is absent locally; all raw output is retained. These checks do not claim a production rollout.
