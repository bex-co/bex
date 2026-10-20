# CLI sweep 1245 — what a same-name successor inherits (live, 2026-10-20)

Two free echo-server services named `qa-20261020-443e5f-inherit` (`srv-dbbihfecbnls73997fb0`, then `srv-dbbihopq56vc73bn8du0`), created with `bex services create` and deleted (GET 404). The ledger audit shows no survivor.

- The first was created with `--env-var QA_INHERIT=… --secret-file qa1245.txt:<local file>`. It was also given a custom domain on the reserved, non-resolvable `.invalid` TLD and a notification override of `none`.
- Configuration is not inherited by the successor: it has no env vars, no secret files, no custom domains, default notification settings, and the process sees no `QA_INHERIT`.
- **Metrics are inherited.** The successor's cpu, memory, instance-count and http-requests series carry pre-creation points, including the deleted pod shown as an instance of the new id. This is added to [w8/144](../../144.md) (same name-keyed root cause as its logs).
