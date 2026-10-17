# Original Flask/Gunicorn defaults on Python3.13 — sweep448

[Results and actual versions](results.json) · [complete native creates, HTTP, config/log evidence and cleanup](complete-original-default-native-HTTP-logs-and-cleanup.json) · [final inventory](final-inventory.json).

This adds Python3.13 hosted acceptance to [existing113](../../../blocked/113.md). Both unchanged native clients create fresh explicit Free Python services, POST201/exit0, in the isolated human-QA workspace. The original sweep362 Flask source and start command are unchanged; the build changes only its external observer marker, and PYTHON_VERSION changes from3.12 to3.13. Public main and immutable app/requirements hashes are freshly reverified. Source HEAD78f5d15a5c20924d98519e1495812a635fe124a8; actual serving revision remains unknown.

Both first children become Live. Six unique normal requests return HTTP200 and the original Hello, World! body. Actual build/runtime observers find the supplied GUNICORN_CMD_ARGS, preload_app=true and accesslog="-", without an authored override or startup prefix. All six exact GET paths appear as Gunicorn200 application access lines. These are actual application logs, distinct from edge request logs. Actual Python/Gunicorn/Flask versions are retained in results rather than inferred from unpinned requirements.

Six REST/GraphQL/MCP source/runtime/Free/command readbacks and two exact API-only PYTHON_VERSION reads pass. Both parents, both first children and former URLs are absent after native delete0; original baseline identities and current inventories/quotas/termination reconcile. Relays and copied configs are removed. No unrelated resource, product code, deployment, commit or push is changed.

This accepts the Python3.13 first-service default variant only. Minor-only PYTHON_VERSION is the existing Bex selector behavior, not a claim about Render's current host or identical Python patches. Group/Secret overrides, static and broader generated-artifact/phase gates remain open. Prior implementation/local evidence is inherited rather than rerun; no fresh full local generated image or dashboard/Blueprint journey is claimed. Item113 stays blocked, with its composite acceptance boxes unchanged. No new issue.

[Consolidated default Python-line evidence](all-supported-default-Python-lines.json) covers3.10–3.14:ten native first deployments and30 HTTP/access-line pairs. Prior sweeps are inherited; group/Secret/static/full-artifact/phase gates remain open.
