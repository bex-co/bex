# Previews non-goal + image deploy logs + KV text — sweep476

Passing functional coverage, no new issue. Same isolated session as sweeps 454–475, `bex-canary`, HEAD checkout build (`95d4794b0`), 2026-10-18T07:49–07:53Z. Inline loop (no cron).

`services update --previews automatic` refuses by name (400 "services previews is not supported by this platform") — the recorded deliberate non-goal. An image service's `--type build` history carries its deploy lifecycle (`Deploy queued → Deploying image … → Your service is live`) rather than an error or a misleading empty. `keyvalues get -o text` renders the full field set, including the honest internal-only wording for an empty allowlist ("external connections blocked").

Cleanup: exact-owned `srv-dba7jsmonb0s73deelh0` and `red-dba7jsuonb0s73deeli0` deleted by ID (404 / named not-found verified); baseline byte-identical. No plan mutations. Session remains active; next inline sweep is 477; m115 is done upstream (fix independently verified in sweep475), m116 + evidence 465–476 remain uncommitted pending `/ship`.
