# CLI sweep 645 — service-type gates on the free plan (live, 2026-10-19)

No fixture created; zero `qa645` services in list.

Via the pinned CLI (`services create --plan free --image …`):

| Type | Result |
| --- | --- |
| `background_worker` | 400 `a background_worker requires a paid plan; plan must be one of starter\|standard\|pro\|pro_plus\|pro_max\|pro_ultra` |
| `private_service` | 400 `a private_service requires a paid plan; …` (same wording) |
| `static_site` with `--plan` | client-side: `--plan is not supported for static_site` |
| `bogus_type` | client-side: `invalid argument "bogus_type" for "--type" flag: must be one of "web_service", "private_service", "background_worker", "static_site", "cron_job"` |

Server paid-plan gates and client enum/flag validation are consistent and named. No bug filed.
