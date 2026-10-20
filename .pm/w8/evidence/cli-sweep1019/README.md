# CLI sweep 1019 — image update requires the full image object (live, 2026-10-20)

Three disposable free web services created and deleted (204s). No new bug filed.

- PATCHing a bare `imagePath` string → 400 unknown field; PATCHing `image: {imagePath}` without `ownerId` → 400. The full object the official CLI always serializes — `image: {imagePath, ownerId: ""}` → 200 and the service reads back the new image (`hello-app:2.0`). The earlier refusals were harness shape errors, corrected against `rest.go`'s `imageRef` contract.
