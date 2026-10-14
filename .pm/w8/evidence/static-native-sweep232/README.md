# Native static build and publish — sweep232

Functional QA through published Bex 0.3.2 and unchanged Render 2.27.0/a764810a7682, isolated human QA session, workspace bex-canary/tea-daif693dqjvc73e7as3g. [Source identity](source-identity.json) is a11a77272e9a85debb788c4c6eb1aa53f7e8ac54; deployed API/operator/static-server SHA is unknown. No product changes, QA commit/push or external submission.

## Observable acceptance

Two fresh owned Free static sites were created natively from public bex-co/bex, main, root examples/hello-go, with auto-deploy disabled. Static creation omits runtime, plan and region because the pinned type-specific validator disallows those flags; Bex's static builder selects its toolchain. The absolute Node build program writes harmless JSON artifacts containing the exact PATH and Unicode/multiline QA_NOTE.

- Published Bex creates with PATH empty. Render creates with PATH /usr/bin:/bin. Both become Live and all five routes return exact decoded authored values: root index, snapshot.json, note%20spaced.json, Unicode snowman filename and a stable snapshot query URL.
- The publisher preserves interior spaces in the directory name. Native argument outer whitespace is intentionally normalized by OptionalNonZeroString in pinned pkg/types/util.go:19 and servicecreate.go:214–234; the two [actual native POST captures](unchanged-native-create-wire.json) show the normalized path before reaching Bex. Backend normalization agrees. This is not a new path-trimming defect. The unused outer-space directory in the build program is not the configured publish target.
- Each client changes only its owned Publish Directory to another existing output directory, observes the distinct NEIGHBOR marker at all unchanged URLs, then restores the original directory and original marker. Four native updates exit zero. Source commands and env are unchanged; no unchanged OCI-image/rebuild identity claim is made. Native PATCH bodies were not intercepted.
- Two native request-log reads contain all five original HTTP requests for their exact owned static service. Eight bounded native filters match the original record IDs exactly: encoded space, encoded Unicode, their OR and a missing-path negative, with GET/status200 and the inclusive recorded end. [Filter oracles](native-request-filter-oracles.json).
- Observed success responses use public, max-age=0, must-revalidate. ETag/Last-Modified were not advertised, so conditional checks were not run. HTTP republication at unchanged URLs is measured; the browser cache/HEAD/custom-header parity and full [w4/m160](../../../w4/blocked/m160/README.md) DoD remain unverified. No workstream4 mutation or premature closeout.

Complete [native/config/artifact/log/cleanup receipts](owned-native-artifacts-logs-cleanup.json), [driver](native-static-driver.py.txt) and [metadata](metadata.json). Static artifacts certify the build environment; a static site has no tenant runtime process, so runtime PATH is not claimed. This adds partial static-build acceptance to w8/m61, which remains open for its other domains and predeploy Job failure.

## Setup dispositions and verification

Two local setup attempts rejected runtime=static and runtime=node on static_site before any service request. Exact-name inventories prove no hosted resource; the second capture has no POST. [Excluded argument refusals](excluded-local-argument-refusals.json) are harness mistakes, not product bugs. The supported creation then succeeds once per client with exact ownership and quota proofs.

Four current operator parent suites pass: TestNativeStaticSiteBuild, TestNativeEnvironmentKeepsApplicationPATH, TestNativeDockerfileKeepsToolchainPATH and TestPublishJobShape. [Events](operator-test-events.jsonl). No full suite or Docker integration is claimed. Request-log client addresses and any actual QA credential/email values are redacted; harmless fixture values remain.

## Cleanup

Both services and 6 recorded deploy children are removed after enumeration and exact-ID native deletion. Detail/list/former runtime are absent, baseline services remain and service quota termination reaches zero. No persistent Blueprint, datastore, environment group or project is created by this sweep. Owned forwarding relays and copied configurations were removed after their finite captures. The ongoing hunt retains only its isolated session. No new issue filed.
