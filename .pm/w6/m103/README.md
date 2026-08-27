# w6 · m103 — Fix i18n language-switch regression and the SSR i18n singleton race causing hydration mismatches for non-English sessions

**Worker:** worker6 **Goal:** switching the dashboard's UI language actually renders in that language on the very next interaction, and every full page load for a non-English session serializes SSR HTML in that session's own language — never another concurrent request's — so hydration never mismatches on language. **Status:** in progress (Bug A fixed + regression-tested via triage 2026-08-26; Bug B / SSR per-request isolation still open — see Triage update)

## Background (found live, 2026-08-25/26 `/qa-find-bugs` hunt, 11th run)

**Repro, live on production, workspace `tea-d98210cbbpdc73dcrkvg`:**

1. Signed in, opened the account menu ("P" avatar, top-right) → **Language: English** → **中文**. This is `user-nav.tsx`'s menu, the only language switcher reachable from the authenticated dashboard (the separate globe-icon `LanguageSwitcher` component only mounts in the pre-login `auth-page-shell`).
2. Immediately after the click, the page stayed fully in English (snapshot confirmed: "Overview", "Projects", etc.) — `i18n.language` and the `i18nextLng` cookie/localStorage were already `"zh"` (`browser_evaluate` confirmed), but no visible text changed.
3. A full page reload (`page.goto`) then rendered fully in Chinese ("概览", "项目", "未分组资源", …) — translation coverage itself is complete and correct — but logged, every single time, on every subsequent reload:
   ```
   Error: Minified React error #418; visit https://react.dev/errors/418?args[]=text&args[]=
   ```
   Evidence: `.playwright-mcp/console-2026-08-26T07-39-44-641Z.log`, `.playwright-mcp/console-2026-08-26T07-40-04-542Z.log`, `.playwright-mcp/console-2026-08-26T07-44-08-287Z.log`, `.playwright-mcp/qa-i18n-hydration-mismatch-1.png`.
4. **Control case, confirmed clean:** switching back to English and reloading produced zero console errors (`.playwright-mcp/page-2026-08-26T07-43-44-704Z.yml`) — English is `DEFAULT_LANGUAGE` (`dashboard/src/i18n/config.ts:5`), so it never diverges from the SSR default and the bug never fires for it. This isolates the defect to any *non-default* language, not to Chinese specifically.
5. Reproduced a second time by writing `i18nextLng=zh` directly via `localStorage`/`document.cookie` (bypassing the menu entirely) and reloading — same #418, confirming the mechanism is the persisted-locale-vs-SSR-render mismatch itself, not something specific to the menu click path.

## Root cause — two distinct, real bugs sharing the generic #418 error code

### Bug A — `user-nav.tsx`'s language switch skips the lazy-load contract

`dashboard/src/common/components/user-nav.tsx:96-98`:

```ts
const handleLanguage = (lang: SupportedLanguage) => {
  persistLanguage(lang);
  void i18n.changeLanguage(lang);
};
```

`w9/m60` (done) made every non-default locale **lazy-loaded** (`ensureLanguage()` in `dashboard/src/i18n/init.ts:31-38` registers the catalog via a dynamic `import()` before `changeLanguage` may safely run — the file's own comment: "Callers must `await` this before `changeLanguage(lang)` (w9/m60 t003) so the UI never renders raw keys"). Exhaustive grep (`changeLanguage(` across `dashboard/src`, excluding tests/comments) finds exactly 4 call sites:

| call site | awaits `ensureLanguage` first? |
| --- | --- |
| `dashboard/src/features/i18n/language-switcher.tsx:29` | yes |
| `dashboard/src/routes/__root.tsx:34-35` | yes |
| `dashboard/src/i18n/use-language-hydration-sync.ts:29-31` | yes |
| **`dashboard/src/common/components/user-nav.tsx:98`** | **no — the only one** |

`user-nav.tsx` is the account-menu switcher — the only one reachable once signed in. Calling `changeLanguage("zh")` without the catalog registered falls back to `fallbackLng: DEFAULT_LANGUAGE` (`init.ts:19`) — i.e. English text renders — while `i18n.language` is now internally `"zh"`. Because `__root.tsx`'s `beforeLoad` guard is `if (i18n.language !== language)` (`__root.tsx:33`), the *next* SPA-internal navigation sees `i18n.language === "zh"` already and skips `ensureLanguage` again — the zh catalog can end up never loading for the rest of that session. This exactly matches step 2 of the repro: the click alone produced no visible change.

**Why no existing test caught this:** `dashboard/src/test/setup.ts:34-38` globally preloads the zh bundle for every test (`i18n.addResourceBundle("zh", "translation", zhResources, true, true)`) specifically to keep synchronous test assertions working after `w9/m60` made loading lazy in production — which also means any unit test for `user-nav.tsx`'s language switch runs in an environment where the exact difference this bug depends on (bundle not yet loaded) doesn't exist.

### Bug B — the SSR i18n instance is one process-wide singleton with no per-request isolation

`dashboard/src/i18n/init.ts:17-29` creates **one module-level `i18n` object** (`i18n.use(initReactI18next).init(...)`), imported by both the server and client bundles. There is no custom `entry-server`/`ssr.tsx` in this app (TanStack Start's default SSR handler is used) and grep confirms **zero** calls to `i18next`'s `createInstance()`/`cloneInstance()` anywhere in `dashboard/src` — every request handled by the same Node process mutates the *same* shared instance.

`dashboard/src/routes/__root.tsx:19-36` (`beforeLoad`, runs per-request on the server):

```ts
const language = detectLanguage(); // dashboard/src/i18n/detect-language/server.ts:28-41 — cookie/URL/Accept-Language
...
if (i18n.language !== language) {
  await ensureLanguage(language);
  await i18n.changeLanguage(language); // mutates the SHARED singleton
}
```

The comment directly above this claims: *"Applied before this route's component renders (both on the server and on the client's initial hydration pass) so the first render on each side uses the same language — no hydration mismatch."* That reasoning assumes one request is in flight at a time. Under real concurrent traffic (the default/majority case is English), a concurrent request for a different session can call `i18n.changeLanguage("en")` on the *same* shared instance between this zh-session request's `changeLanguage("zh")` call and the moment React actually serializes the HTML — so the HTML sent to the zh session can reflect whichever request's language call won the race, independent of that request's own cookie. This is exactly the "SSR renders English, client hydrates to zh" signature reproduced in steps 3–5 above, and is i18next's own documented SSR anti-pattern (shared instance vs. a request-scoped clone).

`dashboard/src/common/lib/document-head/index.ts:174-211`'s `globalMetadata(origin, language)` and the per-route `head()` resolvers (e.g. `services.new.tsx:63-67`) take an explicit `language`/read `i18n.t()` off the same ambient singleton — so **the entire app's SSR output** (every route, not just the ones this hunt clicked through) rides on this one shared, racy instance. Blast radius: sitewide, every authenticated and unauthenticated page, for any session whose language differs from whatever the SSR process most recently rendered for a different concurrent request.

## Adjacent classes (other #418 sources — not this bug, not to be folded in)

- `w6/m102` (open, filed 2026-08-25/26, same day) — `formatRelativeAge`/`formatRelativeUntil`'s `Date.now()`-at-render-time race, 15+ components. Unrelated mechanism (elapsed-time-since-render vs. this milestone's locale-at-render-time); do not merge scope.
- `w6/030` (open inbox note) — an unroot-caused #418 on `/env-groups/<id>`, guessed (not confirmed) to be a timestamp-formatting cause. **Unverified whether it's actually this milestone's Bug B** (an env-group page rendered during a language-mismatched request would show the identical symptom) — t002/t003 below should re-check `/env-groups/<id>` specifically once Bug B is fixed, but this milestone does not claim to fix `w6/030` sight unseen.
- `w1/done/m81/done/t002.md` — already-fixed sitewide #418 from `global-search.tsx` reading `navigator.platform` ungated during render. Confirmed still fixed and present in the current tree (checked live via `w6/030`'s own note); unrelated to this milestone.

## Triage update (2026-08-26)

Both bugs were re-verified against the live tree by direct reads (not taken on trust from the research pass); the milestone is **real, not deleted**. State after triage:

- **Bug A — confirmed and fixed.** `changeLanguage(` still has exactly 4 call sites and `user-nav.tsx` was the lone one skipping `ensureLanguage`. Fixed by making `handleLanguage` `await ensureLanguage(lang)` before `i18n.changeLanguage(lang)`, matching `language-switcher.tsx` verbatim (`dashboard/src/common/components/user-nav.tsx`). t003(a) landed: `dashboard/src/common/components/__tests__/user-nav.test.tsx` now has a switch test that strips `test/setup.ts`'s blanket zh preload first, so it **fails against the pre-fix component and passes after** (verified both directions) — a non-tautological regression guard. `yarn typecheck` / `eslint` / `prettier` / the file's `vitest run` are green.
- **Bug B — confirmed real, still open.** No `createInstance`/`cloneInstance` anywhere and no custom SSR entry (default TanStack Start handler): the shared module-level `i18n` is genuinely process-wide, so the concurrency race is a real latent defect worth the request-scoped-instance fix in t002.
- **Root-cause refinement for t002 (important — don't fix the wrong thing).** The _deterministic, every-single-reload_ #418 in steps 3–5 is **not** the concurrency race (a race would be intermittent). Its actual mechanism: `<html lang>` is threaded correctly through router context (`shell-component.tsx` reads `useRootContext().language`, which is dehydrated → correct on both sides), but the visible **text** comes from the global `i18n` singleton whose language is set as a _side effect_ of `beforeLoad`'s `changeLanguage`. That side effect runs on the server but is not reproduced on the client's initial hydration (dehydrated-context restore doesn't re-run it), and non-default catalogs are lazy `import()`s, so the client cannot render the zh catalog synchronously on first paint → whole-tree mismatch for any non-default session. t002 must therefore cover **both** (i) request-scoped SSR isolation (the concurrency race) and (ii) getting the client's first hydration render into the detected language synchronously — e.g. inline the detected non-default catalog into the SSR payload so it's available before hydrate — not just clone the server instance.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | **✅ done (2026-08-26)** — Fix Bug A: `user-nav.tsx`'s `handleLanguage` must `await ensureLanguage(lang)` before `i18n.changeLanguage(lang)`, matching the contract the other 3 call sites already follow | 20m | — |
| t002 | Fix Bug B: give SSR per-request i18n isolation (e.g. an `i18next.cloneInstance()`/request-scoped instance threaded through `beforeLoad`/render instead of mutating the shared module-level `i18n` singleton) so one request's rendered language can never be clobbered by a concurrent request's `changeLanguage` call | 60m | — |
| t003 | Regression tests: **(a) ✅ done (2026-08-26)** — a `user-nav.tsx` test that does NOT rely on `test/setup.ts`'s global zh-bundle preload, proving the switch actually awaits the catalog load before changing language (fails pre-fix, passes post-fix); (b) still open — a concurrency-shaped test simulating two overlapping SSR requests for different languages and asserting neither request's rendered HTML reflects the other's language | 40m | t001, t002 |
| t004 | Render parity | 20m | t003 |
| t005 | Simplify | 15m | t004 |
| t006 | Test coverage | 20m | t004 |
| t007 | Closeout | 10m | t006 |

## Definition of done

- **Bug A, live-verifiable:** sign in, open the account menu, switch Language to 中文 — the visible UI (not just `i18n.language`/the persisted cookie) updates to Chinese without requiring a full page reload first.
- **Bug B, live-verifiable:** with the fix deployed, issue two rapid concurrent `curl`/fetch requests to the same route with different `i18nextLng` cookies (or `Accept-Language` headers) and confirm each response's server-rendered HTML `<title>`/body text matches its *own* request's language, never the other's — repeatable under load, not just once.
- **No new hydration mismatch:** reloading with a non-English `i18nextLng` cookie logs zero React #418 (or any hydration-mismatch) console errors — checked via `browser_console_messages` on at least 5 consecutive reloads (the current bug reproduces on every single one, so 5 clean reloads is a real signal, not survivorship).
- **Existing behavior holds:** the English control case (already clean pre-fix) stays clean — no regression introduced for the default-language path.
- **Test coverage proves the specific regression class:** the t003 test for Bug A must fail against the pre-fix `user-nav.tsx` (i.e., without `test/setup.ts`'s blanket zh preload standing in the way) and pass after — a test that would have caught this exact regression, not a tautological one.
- **`w6/030` re-checked, not re-guessed:** t003 (or t006) navigates to `/env-groups/<id>` under a non-English session pre- and post-fix and records in the milestone whether that note's symptom disappears — if it does, close `w6/030` as a duplicate of Bug B; if not, `w6/030` stays open exactly as filed.
- **Unverified, carried forward:** whether any other route's SSR output was ever observed serving a mismatched language under real production concurrency prior to this fix (Bug B is reasoned from the shared-singleton architecture and reproduced via the reload-race proxy in steps 3–5, not captured as an actual two-concurrent-request packet trace against production — t002/t003 should attempt that trace as part of verifying the fix).

## Source + Goal linkage

- **Source:** live `/qa-find-bugs` hunt of `dashboard.bex.co`, 11th run, 2026-08-25/26. Evidence: `.playwright-mcp/console-2026-08-26T07-39-44-641Z.log`, `.playwright-mcp/console-2026-08-26T07-40-04-542Z.log`, `.playwright-mcp/console-2026-08-26T07-44-08-287Z.log`, `.playwright-mcp/qa-i18n-hydration-mismatch-1.png`, `.playwright-mcp/page-2026-08-26T07-43-44-704Z.yml` (clean English control). Root cause independently verified by direct reads of `user-nav.tsx`, `language-switcher.tsx`, `__root.tsx`, `i18n/init.ts`, `i18n/detect-language/server.ts`, `i18n/use-language-hydration-sync.ts`, and an exhaustive `changeLanguage(` grep (4 real call sites, quoted above) — not taken solely from the research pass.
- **Goal linkage:** i18n/locale switching is a bex-original enhancement (Render's own dashboard has no equivalent multi-language feature — nothing to diverge from there), shipped by `w9/m60`. A feature that silently doesn't apply on first use (Bug A) and that corrupts server-rendered output under ordinary production concurrency (Bug B) undermines that investment for any workspace member who isn't reading in English — a real, currently-shipped, tenant-facing correctness bug, not a hypothetical.
- **Expected outcome:** switching language works on the first click without a reload; SSR output for any request always reflects that request's own language regardless of concurrent traffic; the class of bug (missing-`ensureLanguage` regressions, shared-singleton SSR races) has a test that would catch a repeat.
- **Why now:** live, currently-reproducing, 100%-repeatable defect on production for the one language-switch entry point every authenticated user actually has; Bug B is an architectural correctness issue (shared mutable state on a request-serving path) that gets worse, not better, as traffic grows — cheap to fix now against 5 real call sites versus discovering it later as an intermittent, hard-to-reproduce production incident.
- **Render parity (t004):** included because the fix touches a dashboard UI surface (`dashboard/`) end to end. Scope: confirm (as this hunt already found) that no REST/GraphQL/MCP surface represents language/locale state at all — it is purely client-persisted (cookie + localStorage), so there is no backend field to keep in sync. Render itself has no equivalent locale-switching feature to compare against (its dashboard is English-only) — record this as the parity finding (a bex-original surface, not a Render-parity gap) rather than skipping the task.
