# Saved-image switch + text semantics II — sweep469

Passing functional coverage, no new issue. Same isolated session as sweeps 454–468, `bex-canary`, HEAD checkout build (`f876458d4`), 2026-10-18T07:05–07:10Z.

Log text terms are case-insensitive literals as recorded (`/MixedCase469` planted, `--text mixedcase469` matches). Saved-image switch, the sanctioned counterpart to the refused per-deploy cross-repo override: `services update --image …:1.0` persists the new saved image, opens no deploy, and the prior release keeps serving; the next explicit deploy goes `live` on 1.0 with the version flip verified over HTTP; a same-repository tag override back to `2.0` is then correctly accepted (only the tag differs from the saved image). Blueprint `sync: false` env placeholder beside a literal validates `valid:true`.

Cleanup: exact-owned `srv-dba6vfr6gvbc73encfs0` (plus its two child deploys) deleted by ID, 404-verified; one new foreign-hunt row observed read-only. No plan mutations. Session remains active; the loop continues from sweep470; m115/m116 + evidence 465–469 remain uncommitted pending `/ship`.
