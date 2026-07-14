---
name: ship
description: Safely bring main up to date, commit intended pending changes, and push to origin/main. Use when the user explicitly asks to ship the current main branch or invoke the repository's ship workflow.
---

# Ship Main

Read [the canonical workflow](references/workflow.md) completely, then follow it. Treat text supplied with the skill invocation as `$ARGUMENTS`. Claude command frontmatter describes the workflow but does not expand Codex permissions; obey the active Codex sandbox and approval policy. Translate references to migrated `/name` commands into Codex `$name` skill invocations when presenting them to the user.
