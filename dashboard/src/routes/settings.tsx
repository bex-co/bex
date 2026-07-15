import { createFileRoute } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import SettingsPage from "@/features/auth/pages/settings-page";

export const Route = createFileRoute("/settings")({
  component: SettingsPage,
  beforeLoad: requireAuth("/settings"),
  // GitHub's cross-site install callback redirects failures here with one
  // bounded reason code. Keep it advisory: unknown values render the generic
  // connect error and never flow into markup or backend requests.
  validateSearch: (
    search: Record<string, unknown>,
  ): { flow?: string; git_error?: string } => {
    const validated: { flow?: string; git_error?: string } = {};
    if (typeof search.flow === "string") validated.flow = search.flow;
    if (typeof search.git_error === "string") {
      validated.git_error = search.git_error;
    }
    return validated;
  },
  head: () => ({
    meta: [{ title: "Settings · bex dashboard" }],
  }),
});
