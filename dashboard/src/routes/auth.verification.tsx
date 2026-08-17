import { createFileRoute } from "@tanstack/react-router";
import { translatedTitleHead } from "@/common/lib/document-head";
import { oryThemeStyle } from "@/common/lib/ory/theme-styles";
import VerificationPage from "@/features/auth/pages/verification-page";

export const Route = createFileRoute("/auth/verification")({
  component: VerificationPage,
  validateSearch: (search: Record<string, unknown>) => ({
    flow: typeof search.flow === "string" ? search.flow : undefined,
  }),
  head: ({ match }) => ({
    ...translatedTitleHead("auth.verificationTitle", match),
    styles: [oryThemeStyle],
  }),
});
