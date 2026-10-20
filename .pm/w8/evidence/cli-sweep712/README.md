# CLI sweep 712 — REST service-create refusal walk (live, 2026-10-19)

Nothing created (final name-filtered list `[]`).

| Body | Result |
| --- | --- |
| `{}` | 400 `invalid request body at /name` |
| `{"name"}` only / `{"name","type":"web_service"}` | 400 `one of repo or image is required` |
| image body without the nested image `ownerId` | 400 `invalid request body at /image/ownerId` (the pinned schema requires the CLI's nested owner field; ADR018 records its workspace validation) |

Each layer names the missing piece in order. No bug filed.
