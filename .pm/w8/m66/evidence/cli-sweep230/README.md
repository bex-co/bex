# Blueprint changes bypass canonical intent — CLI sweep230

An owned Free service starts at Bex/main. The inline API changes it to the public Express/master source, retaining the harmless command/literals and changing rootDir to repository root. It returns200 and creates a new exact-owned Blueprint row. Two subsequent explicit provided-manifest syncs on that row also return200. The desired Manifest remains Express/master, but a fresh service read returns Bex/main. The three corresponding deployments are Canceled. Default-source manual --wait deployments through published Bex and pinned Render succeed and return Bex commit e2ec1ea1…, proving those successes deploy the restored old source rather than the requested Express source. No Cancel was sent. Desired source POSTs and complete replies, owned Blueprint membership, current source and native results are preserved.

At the recorded current HEAD, four actual patchChangedStackService calls patch the requested CR and open a deploy, but make zero canonical IntentStore writes for source, tier, replicas and idle timeout. Fake Kubernetes/recording store diagnostics exercise the feature method; they do not connect to SQL or prove live sibling impact. The projector's actual applyOwnedSpec owns these fields and restores its persisted desired row. The ordinary source setter separately calls SetAppSource before patching. The Blueprint path's missing durable write explains why both API apply paths lose source and why each repeat finds a difference again. The precise cancellation-generation race is inferred from that restore and is not independently isolated.

The separate runtime hook receipt retains cleanup for four own services/twenty deploy records and one own Blueprint, baseline6/quota6/term0, and the preserved pre-existing Blueprint. Native POST bodies were not intercepted; API setup/sync and returned native results are captured. No paid plan/replica was changed live. Counter restart intervals belong to hook measurement and do not weaken the independent repo/branch readback.

- [Live source loss, syncs and native controls](live-source-persistence-receipts.json)
- [Four actual current-source states](current-source-results.json)
- [Feature-method diagnostic](feature-method-probe_test.go.txt)
- [Diagnostic runner](diagnostic-runner.py.txt)
- [Diagnostic events](diagnostic-events.jsonl)
- [Producer, intent writer and projector census](source-census.txt)
- [Metadata](metadata.json)
- [Full owned runtime and cleanup receipt](../../../evidence/hook-runtime-sweep230/README.md)
