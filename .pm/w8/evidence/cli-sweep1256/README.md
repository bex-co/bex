# CLI sweep 1256 — tenant outbound SMTP ports (live, 2026-10-20)

One free image service (`srv-dbbkbd6cbnls73997gf0`, `python:3.12-alpine` probe app) was created over REST and deleted (GET 404). TCP connect and greeting read only; no mail sent. **Finding → [w8/149](../../149.md):** ports 25 and 465 time out from tenant containers (Hetzner's default block, recorded only in platform ADR012), and 587 is open even on free. Render blocks 25/465/587 on free only and allows them on paid.

Also checked against Render's free-tier list: free compute is 100m CPU / 512Mi (`lego/types/tiers/tiers.yaml`), matching Render.
