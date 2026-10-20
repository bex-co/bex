# CLI sweep 1120 — method-override headers are refused at the edge (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- `X-HTTP-Method-Override`, `X-HTTP-Method`, and `X-Method-Override` on a GET → edge 403 (error code 1010), and the service still reads normally afterwards. Verb-smuggling headers never reach the application, and the override is never honored.
