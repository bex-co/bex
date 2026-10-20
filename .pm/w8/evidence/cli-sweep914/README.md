# CLI sweep 914 — server-owned fields cannot be spoofed via PATCH (live, 2026-10-20)

Disposable free web service created, probed, deleted (204, then GET 404). No new bug filed.

- `suspenders`, `suspended`, and `phase` in a PATCH body → each 400 `unknown field` by name; the read-back state is untouched (`not_suspended`, real phase, `suspenders: []`). Lifecycle state is writable only through its verbs (suspend/resume, sweep 797), never by writing the read model back.
