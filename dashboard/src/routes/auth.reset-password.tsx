import { createFileRoute } from "@tanstack/react-router";
import { translatedTitleHead } from "@/common/lib/document-head";
import { oryThemeStyle } from "@/common/lib/ory/theme-styles";
import { requireAuth } from "@/common/lib/auth/auth";
import SettingsPage from "@/features/auth/pages/settings-page";
import { AccountSettingsPageSkeleton } from "@/common/components/route-skeletons";

/**
 * Alias for /settings — Kratos's recovery flow redirects here once its code
 * is verified, landing the user on the settings flow's password field.
 * Kept as its own route (rather than a redirect) so it works whichever URL
 * Kratos is configured to send recovered users to.
 */
export const Route = createFileRoute("/auth/reset-password")({
  staticData: { chrome: true },
  component: SettingsPage,
  pendingComponent: AccountSettingsPageSkeleton,
  beforeLoad: requireAuth(),
  head: ({ match }) => ({
    ...translatedTitleHead("auth.resetPasswordTitle", match),
    styles: [oryThemeStyle],
  }),
});
