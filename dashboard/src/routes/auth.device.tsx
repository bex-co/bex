import { createFileRoute } from "@tanstack/react-router";
import { translatedTitleHead } from "@/common/lib/document-head";

// Pure layout for the `/auth/device/*` subtree. Hydra's registered
// `redirect_uri` for the success landing page is the sibling literal path
// `/auth/device/success` (deploy/gitops/*/values/hydra.values.yaml, not
// renameable) — a genuine child route of this one in TanStack Router's tree,
// so this file must render `<Outlet/>` for it to appear at all. Carrying no
// `component` is what does that (TanStack Router's default for an unset
// component). The GET/POST bridge to Hydra, and the confirm page itself,
// live on the sibling index route (auth.device.index.tsx) — the route tree's
// index-route processing claims the exact `/auth/device` path for whichever
// route's `fullPath` ends in `/`, so `server.handlers` has to live there, not
// here, or the platform's SSR document handler intercepts the request first
// (found the hard way: a first attempt with handlers here silently served a
// blank 200 instead of the GET handler's 302/503/confirm-view responses).
//
// `head` stays here rather than on the index route: it's the one title for
// this whole subtree's entry page, and route-heads.test.ts asserts it against
// this route module.
export const Route = createFileRoute("/auth/device")({
  head: ({ match }) => translatedTitleHead("auth.deviceTitle", match),
});
