# Native deployment-menu Dashboard acceptance — sweep211

Fresh checkout binary qa209-cb16919ab75d at cb16919ab75d1a72663109b06707fb3663fc33cb opens the correct absolute Bex web/deploy URL from a missing config and a saved equivalent :443 API host. The selected service and Live initial deployment belong to this run's Node fixture. Published Bex 0.3.2's missing-config control still opens the relative /web/<own-service>/deploys/<own-deploy> path. The CLI module tree is byte-identical between binary-build, replay-diagnosis and archiving HEADs; deployed API/operator SHA is not inferred.

Three native deploys list <own-id> --confirm -o interactive PTYs load actual production metadata, select the recorded Live deploy, select Dashboard and pass argv to a local OS opener stub. Both current cases initialize https://dashboard.bex.co without persisting the privately injected environment grant. All three exit0 without timeout/panic, PTYs are reaped and copied configs/stub are removed. No browser is launched and no deployment mutation is selected. The reused Node fixture/two child records are now cleaned, baseline/quota restored.

One initial QA attempt assumed the service palette order and selected Logs, then timed out without an opener capture. Deployment palettes list Logs first and Dashboard second; the corrected driver explicitly selects the second row. The excluded attempt was read-only and is not a product bug or passing case; its full capture remains private. Correct positive terminal receipts are retained here, including sanitized base64 bytes.

This verifies w8/090's previously unexercised current-build deploy-menu caller. Real authenticated browser arrival, a CLI release and the complete twelve shared link-shape acceptance remain unproved. The item stays release-blocked; no new issue or QA product implementation/commit/push/release is performed.

Evidence:

- [metadata.json](metadata.json)
- [native-driver.py.txt](native-driver.py.txt)
- [native-results.json](native-results.json)
- [source-identity.json](source-identity.json)
