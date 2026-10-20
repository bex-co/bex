# CLI sweep 1002 — region immutability and port range (live, 2026-10-20)

Two disposable free web services created and deleted (204s). No new bug filed.

- `region` is not a PATCH field at all: both the current value (`fsn1`) and a bogus value get the identical coded 400 `request body contains unknown field "region"` — region is fixed at create, and the PATCH schema refuses the field rather than silently ignoring it.
- Port validation: `0` and `70000` → 400; `8080` → 200 and reads back.
