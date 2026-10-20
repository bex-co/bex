# CLI sweep 1184 — region is not PATCHable through serviceDetails (live, 2026-10-20)

One disposable free web service created and deleted (204, verified 404). No new bug filed.

- PATCH `serviceDetails.region` → 400 `request body contains unknown field "region"` — for a bogus value and for the service's own current region alike. Region is fixed at create; the PATCH schema does not admit the field at all (strict decoding, sweep 1141), so region migration cannot be smuggled through a details patch.
