import { createFileRoute } from "@tanstack/react-router";
import { redirectPreservingSuffix } from "@/common/lib/render-alias";

/** Render's sign-in URL (`dashboard.render.com/login`) — habit + docs links;
 *  bex's page lives under `/auth/login`. Query/hash survive the redirect. */
export const Route = createFileRoute("/login")({
  beforeLoad: ({ location }) => {
    redirectPreservingSuffix("/auth/login", location);
  },
});
