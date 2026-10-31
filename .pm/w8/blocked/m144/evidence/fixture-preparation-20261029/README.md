# Owned disk fixture preparation — 2026-10-29

This is the sanitized, source-only harness and local guard evidence. MANIFEST.json lists the twenty-one original files and hashes; private probe-token, credential/session files, pycache and symlinks are excluded. No production authentication, request or resource was created during preparation. The real local ENOSPC result uses an isolated 32Mi tmpfs and does not establish production 1Gi behavior.

The plan and Kubernetes contract distinguish public service ID from App metadata.name and the derived hashed PVC/CronJob names. Actual production UID bindings must come from the read-only inventory after a proven own HTTP201 creation. The creation gate requires actual driver/operator/class readiness; backup and node evidence use only the separate fixed, UID-bound CI actions. There is no generic remote shell input. Budget/expiry and proven-own cleanup guards remain part of the intended walkthrough.

Original production capacity, online grow, remount, snapshot/restore, metrics/billing and shared-surface DoDs remain open. This preparation record must not be treated as their completion.

Naming and creation timestamps share UTC. Refreshing a stale preparation is refused after any create intent; the actual production record still requires its own verified HTTP201 and live UID bindings.
