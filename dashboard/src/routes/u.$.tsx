import { createFileRoute } from "@tanstack/react-router";
import { redirectPreservingSuffix } from "@/common/lib/render-alias";

/**
 * Render's canonical account-settings shape: `/settings` there redirects to
 * `/u/{usr-id}/settings` (live capture 2026-07-16,
 * docs/render-artifacts/dashboard-routes.md § Workspace/user/create scheme).
 * bex's account settings are caller-scoped at `/settings`, so ANY id lands
 * there — per the m39 rule, an alias never validates the id in the URL.
 */
export const Route = createFileRoute("/u/$")({
  beforeLoad: ({ location }) => {
    redirectPreservingSuffix("/settings", location);
  },
});
