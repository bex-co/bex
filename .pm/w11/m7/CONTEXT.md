# w11/m7 handoff context

Updated 2026-08-16. This is a compact continuation note, not completion
evidence. Preserve the dirty worktree: milestone and mobile changes predate and
overlap this UI pass.

## Milestone state

- Milestone: live mobile agent attach and ACP needs-decision steering.
- `t003` and `t010`–`t012` are done. The milestone remains open and is still
  gated as recorded in `README.md`.
- `t013` is the required Expo MCP visual-verification task. It remains open:
  focused composer, repository-picker, locale, Dynamic Type, and fallback passes
  are recorded, but the complete live attach/stream/decision/recovery matrix has
  not been run.
- Do not mark `t013` or the milestone done from static checks, screenshots, or
  the focused composer pass.
- The authorized live closeout procedure is in `mobile/e2e/m7-live-agent.md`.
  It creates sessions, branches, draft PRs, notifications, and model usage; do
  not run it without explicit authorization.

## Latest product decisions

- The New Agent Session screen was simplified to one full-height prompt with a
  compact close/title/submit header and repository, branch, and agent controls
  directly above the keyboard.
- Repository selection uses a dedicated searchable sheet; selecting a row fills
  its default branch. Compact chips show the repository leaf name while the full
  repository identity remains in state and the mutation.
- Agent selection has an explicit Auto mode and an above-toolbar menu with
  selected-state accessibility semantics.
- Creating a session must not show the generic “Confirm action” dialog. Submit
  executes immediately. Only an explicit server-side protected-action response
  may request confirmation.
- Keep single-flight/double-tap protection, pending indication, action-bound
  retry identity, honest timeout/error outcomes, and server-required
  confirmation behavior.
- A submitted create request that fails must not place feedback inside the
  44-point header column. Composer feedback is a concise fixed-width overlay:
  “Couldn't create the session. Try again.” / “无法创建会话，请重试。”

## Current implementation

- `mobile/src/features/agent-sessions/composer/session-composer-screen.tsx`
  contains the compact full-screen composer, responsive chips, direct submit,
  and fixed-width feedback placement.
- `mobile/src/features/agent-sessions/composer/repository-picker.tsx` contains
  the searchable, keyboard-safe repository sheet.
- `mobile/src/components/menu-button/index.tsx` supports above/below placement,
  responsive window dimensions, and selected radio semantics. Applying the
  complete computed position (including `bottom`) fixed the menu jumping to the
  top of the screen.
- `mobile/src/components/safe-action/safe-action-panel.tsx` supports:
  `renderTrigger`, `emptyTriggerLabel`, `confirmationMode="server-only"`, and a
  custom feedback container. A synchronous `pendingRef` closes the double-tap
  window before React rerenders.
- `mobile/src/features/agent-sessions/composer/compose.ts` includes
  `repositoryDisplayName`.
- English/Chinese composer copy uses “Ask anything” / “想做什么？” and Auto /
  自动.
- Layout/source contracts are in
  `composer/__tests__/session-composer-layout.test.ts`; repository keyboard
  contracts are in `repository-picker-layout.test.ts`.

## Expo MCP evidence and environment

- Expo MCP's bundled official iOS XCTest driver was used from
  `mobile/node_modules/expo-mcp/dist/automation/AutomationIos.js`.
- Signed-in simulator: iPhone 17 Pro, iOS 26.5, 402×874 pt,
  `5C1827F5-7439-4CF7-BA2D-76F1CBB2214D`; Expo Go bundle id
  `host.exp.Exponent`.
- Metro has used tmux session `bex-expo-mcp` on port 8081.
- Verified: 44×44 close/submit targets; prompt and all three context controls
  with the software keyboard; searchable repository sheet; agent menu anchored
  above the toolbar; selected Auto state; no overlap or clipping.
- After HMR the app can land on Workspace unavailable. Recovery on the current
  simulator was Try again near `(201,510)`, Sessions near `(250,815)`, then New
  near `(374,88)`.
- Hardware-keyboard persistence was restored with
  `defaults delete com.apple.iphonesimulator ConnectHardwareKeyboard`; the
  running Simulator process may retain its current keyboard behavior until a
  restart.
- Useful ephemeral screenshots:
  `/tmp/bex-agent-composer-current.png`,
  `/tmp/bex-agent-composer-agent-menu-final.png`, and
  `/tmp/bex-agent-composer-direct-submit-fixed.png`.
- The malformed failure screenshot supplied by the user showed the feedback
  card squeezed into the submit column and the header displaced. The layout is
  fixed in code. It was not recreated after the fix because doing so would send
  another real create-session mutation. Direct-submit behavior is covered by the
  source contract; no production session should be created solely for a visual
  screenshot.

## Verification completed

- `yarn test`: 343 tests passed, including formatting, TypeScript, ESLint, unit
  tests, Expo dependency check, and public Expo config.
- Fresh `yarn bundle:ios` and `yarn bundle:android` exports passed after removing
  the confirmation and fixing feedback layout.
- `git diff --check` passed.
- A final Expo navigation pass showed the restored compact composer with the
  submit target at `(342,70,44,44)`, prompt from `y=122`, and repository control
  at `y=788` without keyboard. No mutation was sent during that pass.

## Next continuation

1. Preserve unrelated and pre-existing dirty changes; do not reset the
   worktree.
2. If changing composer UI again, rerun Expo MCP on the signed-in iPhone with
   the software keyboard and inspect repository selection, agent menu, pending,
   and readable failure feedback.
3. Do not reintroduce client-side confirmation for session creation.
4. Continue `t013` only with reachable real states; leave unchecked states open
   and record exact blockers.
5. Before handoff, rerun full mobile tests, both platform exports, and
   `git diff --check`.
