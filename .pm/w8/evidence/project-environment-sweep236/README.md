# Project/environment formats and rename — sweep236

Published Bex 0.3.2 and unchanged Render 2.27.0/a764810a7682 against production, isolated human QA session in bex-canary/tea-daif693dqjvc73e7as3g. Source 195e2ebb1f1a086180bc674928d9c05a45a86153; deployed API revision unknown. No product changes, QA commit/push or external filing.

The pin exposes project/environment listing only. API setup creates one owned project and three empty environments atomically, with plain, Unicode café/snowman and comma-bearing names. No policy field, service/datastore membership, compute resource or paid-plan mutation is sent. Both clients list projects and that exact project's environments in JSON/YAML/text before and after an API rename. All 36 native reads exit 0; after excluding the first 12 redaction diagnostics, all 24 repeated-before/after reads preserve names, JSON/YAML data equality and nonempty text. The renamed environment retains its ID and project, and both clients show its new name. [Final comparisons](final-format-comparisons.json), [complete commands, API setup/rename and final inventory](native-formats-rename-and-final-inventory.json).

## Excluded harness artifact

The original private wrappers replaced the owner email after serialization with bracketed redaction text. Unquoted YAML parsed that placeholder as a sequence, while quoted JSON kept a string, causing two false project format mismatches. The environment reads were unaffected. A YAML-safe scalar placeholder in the private wrappers corrects the artifact; repeated actual native reads agree. No product defect, fix or source edit is claimed. [Disposition](private-wrapper-redaction-disposition.json).

Public evm- environment aliases are the current documented mapping of durable env- keys at lego/backend/internal/id/environment.go:25–39, not malformed IDs. No new defect is confirmed. [Metadata and limits](metadata.json), [finite driver](grouping-driver.py.txt). Native GETs were not intercepted; API setup/rename/cleanup requests and replies are complete. No browser/MCP/full-surface parity or full suite is claimed.

## Cleanup

All three exact-owned environments were enumerated as empty, deleted by API 204 and verified API 404 before deleting the project. The project detail had no environments; its owned GraphQL serviceIds/databaseIds/keyValueIds census was also empty. Parent delete returns 204, detail 404 and list absence; baseline project IDs remain. [Owned grouping ledger](owned-grouping-setup-cleanup.json). No other hosted resource or persistent Blueprint is created. Actual credential/email values are redacted; harmless names remain. The ongoing hunt retains only its isolated session.
