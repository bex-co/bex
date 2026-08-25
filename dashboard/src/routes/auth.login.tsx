import { createFileRoute } from "@tanstack/react-router";
import { translatedTitleHead } from "@/common/lib/document-head";
import { oryThemeStyle } from "@/common/lib/ory/theme-styles";
import LoginPage from "@/features/auth/pages/login-page";
import { LoginRouteSkeleton } from "@/common/components/route-skeletons";

export const Route = createFileRoute("/auth/login")({
  component: LoginPage,
  pendingComponent: LoginRouteSkeleton,
  validateSearch: (search: Record<string, unknown>) => ({
    next: typeof search.next === "string" ? search.next : undefined,
    flow: typeof search.flow === "string" ? search.flow : undefined,
    // Hydra OAuth2 authorization in progress (urls.login points here, w4/m9);
    // advisory — absent/stale degrades to the ordinary login page.
    login_challenge:
      typeof search.login_challenge === "string"
        ? search.login_challenge
        : undefined,
    // Second-factor step-up, as `requireAuth` (and the consent route) ask for it
    // when the session fetch reports `session_aal2_required` (w4/m17). Only the
    // one value we ever mint is honored — it goes straight to Kratos as a query
    // param, so nothing else from the URL is passed on.
    aal: search.aal === "aal2" ? ("aal2" as const) : undefined,
  }),
  head: ({ match }) => ({
    ...translatedTitleHead("auth.loginSubtitle", match),
    styles: [oryThemeStyle],
  }),
});
