# CLI sweep 721 — git-built web service end-to-end (live, 2026-10-19)

Fixture `qa721-go` (`srv-dbbc332vfpec73dm0m40`), created via CLI from the public repo (`--repo https://github.com/bex-co/bex --branch main --runtime docker --root-directory examples/hello-go`, free). Deleted in sweep 722 and verified absent (REST 404).

- Deploy progressed `created → queued → build_in_progress → live` in about 3.5 minutes; the public URL went 404 (no route yet) → 503 (activator) → 200 serving the app's `OK`.

The git build pipeline works from the pinned CLI. No bug filed.
