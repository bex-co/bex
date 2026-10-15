# Native clone overrides and file-only rebuild — sweep233

Published Bex 0.3.2 and unchanged Render 2.27.0/a764810a7682 against production, isolated human QA session, workspace bex-canary/tea-daif693dqjvc73e7as3g. Source a69dd557da840358a7defb7a43bf49814eafbc2a, [identity](source-identity.json); deployed API/operator SHA unknown. Four fresh owned Free Node services, all auto-deploy off. No product implementation, QA commit/push or external submission.

## Confirmed behavior

Two native-created source services reach Live. The absolute Node build/start commands take snapshots of QA_MARKER, PATH and the complete harmless qa-note.txt bytes, encoded as base64 plus SHA256. Build and runtime match each source's authored Unicode/multiline file, including its final newline; omitted PATH preserves the image default on these observed paths.

Each client then creates a new service through native services create --from <exact-owned-source-id>, with its own explicit marker and replacement file; Bex supplies PATH empty, Render /usr/bin:/bin. The pin copies service settings, not hidden env/file values, so those values are explicitly supplied and no hidden inheritance is expected. [Six actual unchanged native requests and complete replies](unchanged-native-clone-wire.json) comprise two source GETs and one clone POST per client: all GET200/POST201. [Checks](inherited-config-and-explicit-overrides.json) confirm repo, branch, rootDir and native commands are inherited and all override bytes are exact before reaching Bex. Both clones reach Live with exact build/runtime marker, PATH and new file hashes.

Change only the harmless file content on each owned clone using PATCH /v1/services/<id>/environment, secretFiles plus saveMode save_only. There is no service-update file flag in the pin, so this is labeled API setup. Both return200/rolledOut:false. Keep marker, PATH, commands and original commit unchanged, then create each native deploy with --commit <original-commit>. Both eventually become Live with the updated file hash/base64 at build and runtime. Source services still expose their original marker/files. This accepts native file transport and file-only cache refresh alongside empty/narrow PATH on these observed Node paths.

## Bounded wait and queue qualification

Bex deploys create --wait exceeded the harness's240-second bound, exit124; no second deploy/cancel request was issued. Native build logs reported Build queued / Waiting for platform build capacity, while the previous runtime remained served. Passive history/runtime reconciliation observed the exact same recorded deploy proceed through build_in_progress to Live with correct new bytes. This is not a successful native wait receipt and not a new cache/CLI defect. Render's native asynchronous create exits0 and is separately reconciled to Live. The complete [ledger](owned-native-runtime-file-updates-cleanup.json) preserves commands, responses, queue events, timeout and final runtime evidence. Both native instance reads return one owned instance; no SSH session is attempted.

## Source verification and limits

projectNativeBuildEnv in lego/operator/internal/controller/app_controller.go:1574–1623 hashes the current app UID, merged env/literals and file bytes with nativeEnvInputToken, bumping the opaque revision when input changes. nativeDockerfile uses that revision in the loader COPY/RUN cache boundary while mounting file bytes for the authored command. Two focused current operator parents pass: TestNativeSecretFilesTransport and TestNativeDockerfileEnvRevisionBustsCacheKey; [events](file-cache-test-events.jsonl). No full suite, real-DB suite, Docker integration or deployed OCI identity is claimed.

This is partial live acceptance for [w8/m61](../../done/m61/README.md). Other runtimes/surfaces and its predeploy Job shell lookup still gate completion. [Metadata](metadata.json), [driver](native-clone-file-driver.py.txt). All file/marker values in the receipt are deliberately harmless QA fixtures; actual credential/email values are redacted.

## Cleanup

Both clones are deleted and verified absent before their logical source parents. All four exact-owned services and 6 recorded deploy children are removed. Native acknowledgments are followed by detail404, list absence, former runtime404, baseline service presence and quota termination zero. No persistent Blueprint, datastore, group or project is created. Owned forwarding relays/copied configs are removed; the ongoing hunt retains only its isolated session. No new defect filed.
