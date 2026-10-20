# CLI sweep 780 — static site with a build command (live, 2026-10-19)

Fixture `qa780-x` (`srv-dbbcrja720pc73c0kndg`), static site from the public repo with `--build-command 'echo build'`. Deleted and verified absent (REST 404).

Deploy progressed `queued → build_in_progress → update_in_progress`, and the public URL answered 200 while still `update_in_progress` (the static edge serves the extracted content ahead of the terminal status, as in sweep 719).

No bug filed.
