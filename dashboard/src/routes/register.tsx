import { createFileRoute } from "@tanstack/react-router";
import { redirectPreservingSuffix } from "@/common/lib/render-alias";

/** Render's sign-up URL (`dashboard.render.com/register`) — bex's page lives
 *  under `/auth/sign-up`. Query/hash survive the redirect, so an emailed
 *  `?invite=<token>` (w1/m33) reaches the sign-up page's stash intact. */
export const Route = createFileRoute("/register")({
  beforeLoad: ({ location }) => {
    redirectPreservingSuffix("/auth/sign-up", location);
  },
});
