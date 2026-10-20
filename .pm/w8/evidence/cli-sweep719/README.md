# CLI sweep 719 — static site end-to-end (live, 2026-10-19)

Fixture `qa719-static` (`srv-dbbc23qvfpec73dm0lvg`), created from the public repo (`github.com/bex-co/bex`, root `examples/static-site`, publish `.`) with no `--plan` (sweep 645/646 dialect). Deleted in sweep 720 and verified absent (REST 404).

- First public hit: 503 (activator), retry: 200 with the example `index.html` — while the create deploy still read `update_in_progress`; it settled `live` ~25s later with the triggering commit id recorded.

No bug filed.
