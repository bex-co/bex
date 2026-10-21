# CLI sweep 1286 — re-checking this loop's uncommitted filings against pin `a3f66521f93d` and w5's parallel parity work (2026-10-21)

The w5 workstream ran its own Render-parity rounds over the same period (w5/m148–m153, w5/170–172), and several of them overlap this loop's filings. The w8 notes are not yet on `main`, so the other sessions could not see them. Re-checked live with `bex blueprints validate` on this pin:

| w8 note | Live result | Disposition |
| --- | --- | --- |
| 155 `fromService.envVarKey: RENDER_*` | `default-ref.yaml` valid | **done**, by w5/m148 |
| 159 no `RENDER` variable | present per w4/386's live probe on this pin | **done**, by w5/m148 (follow-up w4/386) |
| 161 copy of a `sync: false` source | `syncref.yaml` flat and nested both valid | **done**, by w5/170 |
| 167 registry allowlist | still refused (`cgr.dev` etc.) | **duplicate** of w5/171, blocked on the user's risk decision |
| 156 YAML aliases | `alias.yaml` still refused | open |
| 162 external redirects | `redir.yaml` still refused | open |
| 165 `rootDir` trailing slash | still refused | open |
| 168 default port 3000 | unchanged (`DefaultPort = 3000`); w5/m149 makes `PORT` settable but does not change the default | open |

Notes 153, 154, 157, 158, 160, 163, 164 and 166 stay open; their live repros are in their own evidence.
