import oryElementsCss from "@ory/elements-react/theme/styles.css?inline";

/**
 * Ory Elements' theme stylesheet as a route head() style entry. Only the
 * routes that actually render Ory flow components (`/auth/login`,
 * `/auth/sign-up`, `/auth/forgot-password`, `/auth/verification`,
 * `/auth/reset-password`, `/settings`) inject it — every other page's SSR
 * document skips the ~66 KB sheet.
 *
 * Layer order is preserved by the router's match ordering: head styles are
 * emitted root match first, leaf route match last (`matches.flatMap` in
 * `@tanstack/react-router`'s headContentUtils), so this entry always lands
 * after the root's appCss entry and `@layer ory-elements` is still declared
 * after Tailwind's `base` — the same relative order the root used to hardcode
 * (see the order comment in `routes/__root.tsx`). Client-side navigation is
 * covered too: `HeadContent` derives the tags reactively from the live
 * matches, so entering an Ory route injects the sheet and leaving it removes
 * it.
 */
export const oryThemeStyle = { children: oryElementsCss };
