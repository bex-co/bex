# CLI sweep 530 — `git://` repo URLs refuse at create

**Verdict: no bug filed.** Same isolated session, `bex-canary`, HEAD build `218dffcd1`, 2026-10-19T09:56Z. Refusal-only; nothing created.

`services create --repo git://github.com/bex-co/bex.git …` → exit `1`, `400: repo must be an https/ssh/git URL`. The refusal itself is right (the unauthenticated git transport is dead on GitHub since 2022 and bex's clone pins HTTPS for submodules), but the message's own wording lists "git" as an accepted form while rejecting the `git://` scheme — in `store.ValidRepo` the accepted no-scheme "git" form is the SCP shape (`git@host:path`). Recorded as a wording nuance inside w8/130's remit (its fix already asks that the non-GitHub/SSH remainder refuse "with a message that names the supported forms"), not filed separately.

Also confirmed en route: `blueprints` exposes only `validate` at this pin — there is no CLI blueprint list/get/apply surface left untested.

- Deploy lag means deployed revisions can precede local `main`.
