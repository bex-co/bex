# CLI sweep 1072 — project read carries its environment ids (live, 2026-10-20)

One disposable project with an inline environment created and deleted (204; GET 404). A pre-sweep census also re-confirmed the sweep-1071 env-group fixtures are gone from both workspaces. No new bug filed.

- The project GET returns `environmentIds` for the inline-created environment (the read-side counterpart of sweep 965's create-inline shape).
