# CLI sweep 737 — duplicate entries in env/secret PUT bodies (live, 2026-10-19)

Disposable free web service `qa737-web` (`srv-dbbchrd4c21c738t02rg`). Deleted and verified absent (REST 404).

- Env-vars PUT with the same key twice (`first`, then `second`) → 200 with a single `DUP=second` entry: last wins, map semantics.
- Secret-files PUT with the same name twice → 400 `secret file "dup.txt" is named twice`, and nothing was stored (single-file GET 404). The two surfaces deliberately differ: env vars are a key→value map, secret files a named set.
- Env entry missing `key` or `value` → 400 `invalid request body at /0`; a non-array body → 400 `request body does not match the Render API schema`.

Recorded as an intentional asymmetry (no upstream oracle contradicts either side). No bug filed.
