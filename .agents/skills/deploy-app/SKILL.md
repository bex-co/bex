---
name: deploy-app
description: Deploy a bex App to the current Kubernetes cluster and verify it reaches Ready. Use when the user asks to deploy a bex.yml or the sample app and validate the rollout end to end.
---

# Deploy App

Read [the canonical workflow](references/workflow.md) completely, then follow it. Treat text supplied with the skill invocation as `$ARGUMENTS`. Claude command frontmatter describes the workflow but does not expand Codex permissions; obey the active Codex sandbox and approval policy. Translate references to migrated `/name` commands into Codex `$name` skill invocations when presenting them to the user.
