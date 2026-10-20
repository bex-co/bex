# CLI sweep 1101 — image service port readback (live, 2026-10-20)

One disposable free web service created and deleted (204). Ancestry re-checked: `94533d795`, `3a2a8d8a1`, and `cac641a9e` are not in the pin (as sweep 1100 recorded). No new bug filed.

- A fresh image service reads `port: 3000` (the hello-app default the platform detected) with internal address `qa1101-web:3000`, unprompted by any port field in the create body.
