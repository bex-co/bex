# CLI sweep 1249 — clone region wall is documented; custom-domain claims release on delete (live, 2026-10-20)

Two free echo-server services created with `bex services create` and deleted (GET 404). Custom domains used the reserved, non-resolvable `.invalid` TLD. No new bug filed.

- `bex services create --from <id>` with no `--region` fails client-side on the source's `fsn1` region. This is documented: `docs/cli-compatibility-checklist.md:69` (“a bare `services create --from` clone fails client-side and needs an explicit `--region`”) and w9/done/m56. Sweep 1042 had attributed it to w8/075, but 075 covers type/runtime-forbidden inherited fields.
- A custom domain held by service A → `409 CUSTOM_DOMAIN_IN_USE` for service B. Immediately after A is deleted (GET 404), B's add → 201. The claim is released with the service.
- Custom-domain objects use the domain name as their `id` (Render ids look like `cdm-…`). The pinned CLI addresses domains by id-or-name, so nothing breaks; not filed.
