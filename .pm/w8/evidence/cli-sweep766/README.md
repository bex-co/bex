# CLI sweep 766 — secret-file name edges (live, 2026-10-19)

Disposable free web service `qa766-web` (`srv-dbbcoka720pc73c0kmr0`). Deleted and verified absent (REST 404).

`PUT /secret-files` name probes — all 400 `SECRET_FILE_INVALID` naming the config-key rule:

| Name | Result |
| --- | --- |
| `/etc/passwd` | refused (no absolute paths) |
| `sub/dir.txt` | refused (no slashes) |
| `""` | refused |

An empty `content` with a valid name → 200 (an empty file is storable). Extends sweep 660's traversal probe.

No bug filed.
