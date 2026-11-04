# Complete D3 AMD64 utility baseline on 2ee5d25 — 2026-11-04

[Run 41816417411](https://github.com/bex-co/bex/actions/runs/41816417411) (#100, `workflow_dispatch`, `source=reviewed-candidate`, `profile=utilities`) passed the complete original AMD64 utility cohort on source `2ee5d257395c126fde2cc6b974be89883a032544` with the unchanged reviewed `0941` common image. Job `qualify (amd64)` 126534478405 ran 11:03–11:5xZ.

Why a new baseline: run #97 (`3a15c2a`, success) went stale before it was bound. After it, `56c12aea0`/`7991a0e57`/`98ebc1367` (w2/m254) changed `lego/operator/internal/build/{build,native}.go` and `7b140e06e` (w5/175) changed a comment in `lego/types/v1alpha1/app_types.go`, all inside the qualification inventory.

- `utilities/amd64/artifact/`: all 356 artifact members, byte for byte, extracted from artifact 13316831667 (`utility-metadata/amd64-artifact.json`; the ZIP's sha256 matched the API `digest` before extraction). The artifact's own `MANIFEST.json` lists the other 355 members; every hash was re-verified and no member is unlisted.
- `metadata/jobs-final.json`: the run's jobs API response.
- Only `candidate.source` and `candidate.utilities` change in `.github/native-common-test-candidate.json`. The image, producer/verifier, publication and approval are unchanged.
- The unchanged `validate_test_candidate()` returns `current-complete` against the clean working tree at the binding commit. Locally generated `__pycache__` files under `build/testdata` were removed first: they are ignored by Git but are counted by the inventory walk.

This is the utility prerequisite for the 27 `REQUIRED-ORIGINAL-AMD64` family cases (`../common-amd64-production-20261102/coverage-plan/`). It is not family, adoption or hosted acceptance.
