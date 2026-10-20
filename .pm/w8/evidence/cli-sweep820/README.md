# CLI sweep 820 — ssh CLI shape + deploy-hook non-exposure (live, 2026-10-20)

Disposable free web service created and deleted (204, then GET 404). No new bug filed. Interactive SSH was not opened against any service (it is an interactive session surface; only its CLI flag shape was probed).

- `bex ssh <id> --command …` → `Error: unknown flag: --command` with usage — the pinned CLI's ssh takes a positional service/instance and `--ephemeral`/`--plan`-style flags, no one-shot command flag. Launcher pass-through, as in sweep 814.
- The service GET carries no `deployHook*` field: the deploy-hook URL is a secret-bearing trigger URL and is not echoed on the read object (the `RegenerateDeployHook` verb exists per ADR018's event-vocabulary classification; the URL itself is revealed only through its dedicated flow, never recorded here).
