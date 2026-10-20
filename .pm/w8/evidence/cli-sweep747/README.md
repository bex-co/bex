# CLI sweep 747 — skills install/update non-TTY behavior (live, 2026-10-19)

Local CLI surfaces only.

- `bex skills install --dry-run -o json` → exit 0, structured skill catalog (nothing written).
- `bex skills update -o json` → the same exit-2 nil-pointer TUI panic as sweep 746's `skills list` — both halves of the recorded upstream defect (`docs/cli-compatibility-checklist.md:58`) re-confirmed at pin `a764810a7682`. Not refiled.

No bug filed.
