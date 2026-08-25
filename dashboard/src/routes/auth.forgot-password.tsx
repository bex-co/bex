import { createFileRoute } from "@tanstack/react-router";
import { translatedTitleHead } from "@/common/lib/document-head";
import { oryThemeStyle } from "@/common/lib/ory/theme-styles";
import ForgotPasswordPage from "@/features/auth/pages/forgot-password-page";
import { RecoveryRouteSkeleton } from "@/common/components/route-skeletons";

export const Route = createFileRoute("/auth/forgot-password")({
  component: ForgotPasswordPage,
  pendingComponent: RecoveryRouteSkeleton,
  validateSearch: (search: Record<string, unknown>) => ({
    flow: typeof search.flow === "string" ? search.flow : undefined,
  }),
  head: ({ match }) => ({
    ...translatedTitleHead("auth.forgotPasswordTitle", match),
    styles: [oryThemeStyle],
  }),
});
