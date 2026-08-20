import { createFileRoute } from "@tanstack/react-router";
import { translatedTitleHead } from "@/common/lib/document-head";
import { oryThemeStyle } from "@/common/lib/ory/theme-styles";
import VerificationPage from "@/features/auth/pages/verification-page";

export const Route = createFileRoute("/auth/verification")({
  component: VerificationPage,
  validateSearch: (search: Record<string, unknown>) => ({
    flow: typeof search.flow === "string" ? search.flow : undefined,
    // Deep-link continuity (ADR075 D3, w6/m42): verification success continues
    // to /auth/login carrying the guarded `next` — from this param when linked
    // directly, else from the same-tab relay the sign-up page stashed.
    next: typeof search.next === "string" ? search.next : undefined,
  }),
  head: ({ match }) => ({
    ...translatedTitleHead("auth.verificationTitle", match),
    styles: [oryThemeStyle],
  }),
});
