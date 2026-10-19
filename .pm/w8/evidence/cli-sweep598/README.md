# CLI sweep 598 — `services` list dialects: positional args silently ignored (upstream), output modes honest

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI rebuilt at `1a49cb806`, 2026-10-19. Read-only against the existing baseline list; no fixture created, nothing to clean up.

- **Positional args are ignored — in the pinned client, not bex:** `bex services hello-go`, `bex services srv-daif…`, `bex services srv-000…` (synthetic), and `bex services nonexistent-name-598` all return the identical full 8-row list, exit 0. The upstream `servicesCmd` (`cmd/service.go:29` in the pinned module) declares no cobra `Args` validator, so positionals fall through to the list RunE untouched. The unmodified Render CLI would do the same against Render; bex does not fork this. Recorded so it is not rediscovered as a server-side selector bug.
- **Output modes:** `-o yaml` and `-o text` render the same set (YAML as `- service:` entries; text as the NAME/PROJECT/ENVIRONMENT/TYPE/ID table). `-o bogus` fails client-side: `invalid output format: bogus`, exit 1.
- **Environment filter:** `-e env-000…` (synthetic) and `-e bogus` both print `null`, exit 0 — an unmatched environment filter is an empty result, not an error (consistent with sweep 559's filter dialects). `--include-previews` returns the same set in this workspace (no previews exist to include).

## Unrun

- Interactive (TTY) `services` picker behavior with a positional arg.
- Pagination of the services list beyond one page (workspace too small to force it).
