# CLI sweep 746 — `skills list` panic re-confirmation, `upgrade --check` (live, 2026-10-19)

Local CLI surfaces; no server state touched.

- `bex skills list` panics in every output mode (json/yaml/text), exit 2, nil-pointer in `tui.(*StackModel).Push` from the pinned upstream `cmd/skillslist.go:40` — `GetStackFromContext` has no stack in non-interactive execution. This is the **already-recorded** upstream defect (`docs/cli-compatibility-checklist.md:58`, `lego/cli/go.mod` pin `a764810a7682`), re-confirmed live at the current pin; on the pin-bump re-check list per w8/done/015. Not refiled.
- `bex upgrade --check` → clean refusal: `self-update is unavailable for a dev build; install a released bex-cli/v* binary`.

No new bug filed.
