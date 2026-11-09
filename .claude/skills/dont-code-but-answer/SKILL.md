---
name: dont-code-but-answer
description: Answer-only mode. Investigate the codebase as needed to answer the user's question, but make no code or file changes. Use only when the user explicitly invokes /dont-code-but-answer or $dont-code-but-answer.
disable-model-invocation: true
---

# Don't code — answer

The user wants an answer, not a change. Treat the text sent with this invocation as the question; if nothing was sent, answer the user's most recent question. Stay in this mode for follow-up questions until the user explicitly asks you to make a change.

## Hard limits

- Do not create, edit, move, or delete any file: source, config, tests, docs, generated output, or scratch files inside the repo.
- Do not run commands that change state: installs, builds, codegen, formatters, linters with `--fix`, migrations, snapshot updates, or git commands that commit, check out, stash, reset, rebase, or push.
- Do not change remote state: deploys, API writes, PRs, issues, or comments.
- Do not hand the work to a subagent or background job that would do any of the above.
- This holds even when the fix is a one-liner. Describe it; don't make it.

## Allowed

- Read files, search the codebase, and inspect history (`git log`, `git show`, `git blame`, `git diff`).
- Run read-only commands that observe behavior without writing to the working tree.
- Look up external documentation when the question needs it.

Investigate enough to answer correctly. Don't guess about code you can read.

## How to answer

- Lead with the direct answer in the first sentence or two, then the evidence.
- Ground non-obvious claims in the code with `path/to/file.ts:42` references.
- If the answer implies a change, say what you would change and where: the file, the function, and the gist of the edit. A short snippet is fine when it is clearer than prose; keep it illustrative, not a full implementation.
- When there are real tradeoffs, recommend one option instead of surveying all of them.
- Say plainly what you could not verify.
- Stop after answering. Don't start implementing; the user will ask for the change if they want it.
