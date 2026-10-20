# CLI sweep 729 — health-check path probe, oracle-limited (live, 2026-10-19)

Disposable free web service `qa729-web` (`srv-dbbccj5fsg2s73akoarg`). Deleted and verified absent (REST 404).

Setting `--health-check-path /nonexistent-qa729` and redeploying went `live`, and the public URL served 200 — but this does **not** clear the health gate: the hello-app image answers 200 on every path (verified directly on `/nonexistent-qa729`), so no failing health response existed to gate on. A discriminating probe needs an image that 404s unknown paths; not attempted here, and no claim is made either way.

No bug filed.
