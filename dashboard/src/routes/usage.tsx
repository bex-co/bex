import { createFileRoute } from "@tanstack/react-router";
import { redirectPreservingSuffix } from "@/common/lib/render-alias";

/**
 * `/usage` was this page's original home (w5/m70 renamed it to `/billing`,
 * matching Render's workspace IA now that the money surface carries payment
 * onboarding, lifecycle, and credits — not just metered usage). The shim keeps
 * bookmarks, deep links, and Stripe return URLs minted before the rename
 * working, query string included. The usage API surface (REST `/v1/usage`,
 * GraphQL `usage`, MCP `get_usage`) is unrenamed — ADR023's bex extension.
 */
export const Route = createFileRoute("/usage")({
  staticData: { chrome: true },
  beforeLoad: ({ location }) => {
    redirectPreservingSuffix("/billing", location);
  },
});
