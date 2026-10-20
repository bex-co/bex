# CLI sweep 902 — log label filters are open-set filters (live, 2026-10-20)

Disposable free web service created, probed, deleted (204, then GET 404). No new bug filed.

- `level=`, `method=`, `statusCode=`, and `instance=` filters with unmatched values → 200 with an empty `logs` page (and coherent window cursors), not 400s: label **values** are an open set (instances/hosts/codes the deployment happens to have), unlike label **names**, which are the fixed vocabulary sweep 828 verified. A typo yields an honestly empty page rather than a fabricated match.
