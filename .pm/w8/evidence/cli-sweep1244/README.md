# CLI sweep 1244 — name reuse after delete; logs leak across incarnations (live, 2026-10-20)

Three successive free `docker.io/ealen/echo-server:0.9.2` services named `qa-20261020-4fc180-reuse` (`srv-dbbibd9q56vc73bn8dgg`, `srv-dbbibk9q56vc73bn8dig`, `srv-dbbict9q56vc73bn8dkg`), each created with `bex services create` and deleted with `bex services delete --confirm` or by ID over REST (all GET 404). The ledger audit shows no survivor.

- Name reuse works: recreation 1.6 s after `services delete` succeeded, the same `https://<name>.onbex.co` went live in 22 s, and the old service's deploys and events stay with the old id.
- **Finding → [w8/144](../../144.md):** the new service's logs (CLI and REST) include its deleted predecessors' app and request lines, including a planted marker path. 13 of 14 lines in the newest service's history predate its own creation.
