import { createFileRoute } from "@tanstack/react-router";
import { translatedTitleHead } from "@/common/lib/document-head";
import { oryThemeStyle } from "@/common/lib/ory/theme-styles";
import RegisterPage from "@/features/auth/pages/register-page";
import { RegistrationRouteSkeleton } from "@/common/components/route-skeletons";

export const Route = createFileRoute("/auth/sign-up")({
  component: RegisterPage,
  pendingComponent: RegistrationRouteSkeleton,
  validateSearch: (search: Record<string, unknown>) => ({
    // Deep-link continuity through the auth boundary (ADR075 D3, w6/m42):
    // sign-up honors the same guarded `next` as login, relayed across the
    // verification hop by features/auth/lib/auth-next.ts.
    next: typeof search.next === "string" ? search.next : undefined,
    flow: typeof search.flow === "string" ? search.flow : undefined,
    // OAuth2 flows can register a brand-new user too (Kratos links the
    // registration flow to the Hydra request the same way as login, w4/m9).
    login_challenge:
      typeof search.login_challenge === "string"
        ? search.login_challenge
        : undefined,
  }),
  head: ({ match }) => ({
    ...translatedTitleHead("auth.registerTitle", match),
    styles: [oryThemeStyle],
  }),
});
