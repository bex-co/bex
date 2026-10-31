# Render disk contract review — 2026-10-29

Re-read [Render persistent disk documentation](https://render.com/docs/disks) during this drain. Capacity increases are online and grow-only. Disks attach to paid web/private/worker services, remain exclusive to one running instance, and are unavailable during builds and pre-deploy commands. Snapshot restores replace later filesystem changes. These contracts support bounded runtime capacity with online growth, while preserving current attachment and restore rules.

This documentation review is not live Bex capacity, metrics, billing, migration, or cross-surface acceptance. Those checks remain open in w8/m144.
