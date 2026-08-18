# m7 live agent attach and decision runbook

This is the interactive native closeout for `w11/m7`. It complements automated
protocol tests; it does not replace them with screenshots or mocked streams.

## Required setup

- A disposable repository installed in the test workspace. Only a generated
  `bex-agent/verify-*` branch may be writable.
- A workspace member bearer with `can_operate` and `can_create`, a provisioned
  BYO model key, the live agent-session gateway, and the Active-tier sandbox
  idle grace needed by the verifier's resume/live-turn leg.
- The approved test identity already signed into the current development build.
  Do not copy auth storage between simulators.
- Push enabled for that identity. A physical signed iOS/Android build is still
  required to qualify APNs/FCM delivery; Expo MCP may use the durable in-app
  Notifications inbox to exercise the same exact authenticated route and UI.

Never put the bearer or model credential in an `EXPO_PUBLIC_*` variable, a
command transcript, screenshot, PM note, or source file.

## Start the native walkthrough

1. Start the SDK 57 development server and open the signed-in app through Expo
   MCP. Record the device model, viewport, OS, theme, locale, Dynamic Type size,
   and whether the software keyboard is enabled.
2. Navigate to Sessions and leave the app ready to receive a notification.
3. In a separate operator shell, supply the secrets out of band and run:

   ```sh
   BEX_LIVE_VERIFY=1 \
   BEX_API_URL=https://api.bex.co \
   BEX_API_TOKEN=... \
   BEX_VERIFY_REPO=owner/disposable-repo \
   BEX_VERIFY_DECISION_MODE=manual \
   BEX_VERIFY_PUSH_INBOX=1 \
   bash scripts/agent-session-verify.sh
   ```

The script creates only generated verification sessions/branches/PRs and
cancels its sessions on exit. The disposable repository's branches and PRs are
left in place for audit and must be cleaned up separately. It deliberately
pauses at each ACP permission instead of answering on the caller's behalf. It
refuses every persistent/bypass permission kind and accepts only the exact
advertised `allow_once` option chosen in mobile.

## Expo MCP interaction matrix

At each `WAIT:` line from the verifier:

1. Open the native Notifications inbox through Expo MCP and confirm there is one
   `agent_needs_decision` item for the generated session. Tap the item—do not
   navigate to the session from the list as a substitute for the deep link.
2. Confirm the exact session opens, durable history appears before the live
   tail, and the typed permission panel is visible. The notification must not
   reveal prompt, schema, diff, repository content, or transcript text.
3. With the software keyboard open where applicable, choose the displayed
   `allow_once` action exactly once. Persistent approval or transcript prose
   must never be offered as a response.
4. During the live follow-up turn, background the app, wait for the stream to
   advance, then foreground it. Confirm the UI shows a distinct reconnecting
   state, obtains a fresh read ticket, resumes after its cursor, and does not
   duplicate the accepted user turn or assistant parts.
5. Scroll upward while output is streaming and confirm new chunks do not pull
   the reader back to the end. Return to the bottom and confirm following
   resumes. Expand large plan/diff/tool/terminal parts and verify unknown parts
   fail closed without dumping their raw payload.
6. Let the verifier retry the same turn idempotency key. Confirm mobile still
   shows one user turn and one continuation, then observe completion and the
   phase-1 result/PR fallback.

Repeat the affected states in light/dark and English/Chinese on the large signed
in phone. Repeat on a compact signed-in phone rather than transferring the first
device's credentials. Physical APNs/FCM qualification is recorded separately
under the m5/m6 push gate.

## Evidence to record

- Generated session ID and disposable PR number only; no ticket, token, prompt,
  transcript, or decision response value.
- Expo MCP device/viewport/theme/locale/text-size matrix and the routes/states
  actually interacted with.
- Verifier PASS lines for action mismatch, nonce reuse, monotonic cursor replay,
  one idempotent durable turn, exact needs-decision inbox route, allow-once
  decision continuation, and ticket/session mismatch.
- Any unreachable state or missing setup remains unchecked in
  `.pm/w11/m7/t013.md`; static code/tests are not replacement evidence.
