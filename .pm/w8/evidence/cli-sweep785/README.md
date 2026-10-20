# CLI sweep 785 — malformed image references at create (live, 2026-10-19)

Refusal-only; nothing created.

| Image | Result |
| --- | --- |
| `…@sha256:zzzz` | 400 `image digest is invalid: invalid checksum digest length` |
| `hello world` | 400 `image reference is malformed: invalid reference format` |
| `''` (empty flag) | client `either repo or image is required` |

Reference parsing refuses at create with the registry library's reasons, instead of failing the first pull.

No bug filed.
