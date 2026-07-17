import { createFileRoute } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import SettingsPage from "@/features/auth/pages/settings-page";

/**
 * Alias for /settings — Kratos's recovery flow redirects here once its code
 * is verified, landing the user on the settings flow's password field.
 * Kept as its own route (rather than a redirect) so it works whichever URL
 * Kratos is configured to send recovered users to.
 */
export const Route = createFileRoute("/auth/reset-password")({
  component: SettingsPage,
  beforeLoad: requireAuth(),
  head: () => ({
    meta: [{ title: "Reset password — bex" }],
  }),
});
