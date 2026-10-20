# CLI sweep 1042 — CLI clone from a disposable hits the known w8/075 region wall (live, 2026-10-20)

One disposable free web service created as the clone source and deleted (204; both name lists empty — the clone never existed). No new bug filed.

- `bex services create --from {disposable} --name qa1042-clone` fails client-side: `region must be one of: frankfurt, ohio, oregon, singapore, virginia`. The pinned CLI validates the cloned `fsn1` region against Render's region list before any API write. This is the already-tracked [w8/075](../blocked/075.md) blocked item (upstream CLI), re-confirmed live on the new pin — not a new filing.
