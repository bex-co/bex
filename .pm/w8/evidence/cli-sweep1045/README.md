# CLI sweep 1045 — path normalization discipline (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- Trailing slash on a subresource (`…/env-vars/`) and uppercase subresource (`…/ENV-VARS`) → 404: paths are exact, consistent with sweep 960.
- A doubled slash in the path (`/services//{id}`) → 200 after the standard Go mux path-cleaning redirect — framework normalization, recorded only.
