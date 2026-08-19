# w10 · m7 — Device-authorize page consistency: shell-styled confirm page, bex CLI branding, streamlined flow

**Worker:** worker10 **Goal:** the `/auth/device` confirm step renders inside the same `AuthPageShell` chrome as every other auth-workflow page (login → device confirm → consent → device success), the whole CLI-authorize flow reads as one streamlined experience, and every user-visible string says "bex CLI", never "Render CLI" **Status:** done

**Resolution (2026-08-19):** rebuilt the hand-rolled inline-HTML device confirm interstitial as a real `AuthPageShell` + Card React page (`device-confirm-page/`), following the `consent-page` pattern; split `auth.device.tsx` into a layout + `auth.device.index.tsx` index route (the layout-only `component` change broke the sibling `/auth/device/success` route, found live and fixed — see t001's Resolution note); swept "Render CLI" → "bex CLI" across locales, document titles, and the device-success terminal replica; verified the whole flow live via a local dev server + Playwright screenshots (`.playwright-mcp/device-confirm-*.png`, `device-success-*.png`); confirmed no REST/GraphQL/MCP surface changed. `yarn typecheck && yarn lint && yarn test` green (335/335 files, 2294/2294 tests). Code is complete and ready to ship via `/ship`.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- | --- |
| t001 | Rebuild the device confirm interstitial as an AuthPageShell React page | 45m | — | — **DONE** |
| t002 | "bex CLI" wording sweep across the device flow (locales, tests, page copy) | 30m | t001 | — **DONE** |
| t003 | End-to-end authorize-flow streamline pass (login → confirm → success) | 30m | t002 | — **DONE** |
| t004 | Render parity — device-verify surface consistency check | 20m | t003 | — **DONE** |
| t005 | Simplify — /simplify over the changed code | 20m | t004 | — **DONE** |
| t006 | Test coverage — confirm-page component + server-fn + route-head tests | 30m | t004 | — **DONE** |
| t007 | Closeout | 15m | t006 | — **DONE** |

## Definition of done

Visiting `https://dashboard.bex.co/auth/device?device_challenge=…&user_code=…` while signed in renders the confirm step inside `AuthPageShell` (theme-aware, language-switchable, same typography/chrome as `/auth/consent` and `/auth/device/success`); the confirm remains a same-origin, session-bound POST to `/auth/device` (codex-security #9 semantics byte-preserved); no user-visible string in the device flow says "Render CLI" (document titles, page copy, en + zh locales all say "bex CLI"); `yarn typecheck && yarn lint && yarn test` green, including updated `route-heads` and `hydra-device`/`hydra-consent` tests.

## Source + Goal linkage

- **Source:** user report 2026-08-18 (`/pm` invocation): the live `dashboard.bex.co/auth/device?device_challenge=…&user_code=…` page's styles differ from the rest of the auth workflow, and the flow says "Render CLI" instead of "bex CLI". Root cause located: `dashboard/src/common/server-fn/hydra-device.ts:213` (`deviceConfirmationPage`) returns hand-rolled inline-HTML with its own fixed dark `<style>` block, bypassing `AuthPageShell`/i18n/theming; `dashboard/src/features/auth/locales/{en,zh}.ts` (`auth.deviceTitle`, `auth.deviceSuccessTitle`) brand the flow "Render CLI".
- **Goal linkage:** the CLI surface (`docs/bex-cli.md`, ADR012 §8a device flow) is a first-class bex surface; the browser device-verify page is the one human-facing step of `bex login`. A visibly off-brand, unthemed interstitial undermines the polished dashboard auth experience (w5/m2 visual pattern) and the bex CLI product identity (the launcher is branded `bex` even though it wraps the upstream Render CLI — user-facing copy must follow ADR058/docs/bex-cli.md branding).
- **Expected outcome:** a user running `bex login` sees a coherent, consistently-styled browser flow end to end, in their chosen theme and language, with "bex CLI" branding throughout.
- **Why now:** user-reported papercut on the live production flow — every new CLI login hits it today.
- **Render parity note:** included (t004) — this is a user-facing UI surface; the parity check is against Render's own CLI device-verify UX and confirms the dashboard-only change needs no REST/GraphQL/MCP counterpart (the Hydra bridge endpoints are unchanged).
