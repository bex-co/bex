import { createFileRoute } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { translatedTitleHead } from "@/common/lib/document-head";
import { oryThemeStyle } from "@/common/lib/ory/theme-styles";
import SettingsPage from "@/features/auth/pages/settings-page";
import { AccountSettingsPageSkeleton } from "@/common/components/route-skeletons";

export const Route = createFileRoute("/settings")({
  staticData: { chrome: true },
  component: SettingsPage,
  pendingComponent: AccountSettingsPageSkeleton,
  beforeLoad: requireAuth(),
  // GitHub's cross-site install callback redirects failures here with one
  // bounded reason code. Keep it advisory: unknown values render the generic
  // connect error and never flow into markup or backend requests. `returnTo`
  // (w2/m66) is the path a RequiresSshKey CTA came from — round-tripped back
  // after a key is saved; it is validated through safe-next.ts before any
  // navigation, so an off-origin value can never become an open redirect.
  validateSearch: (
    search: Record<string, unknown>,
  ): {
    flow?: string;
    git_error?: string;
    returnTo?: string;
    addKey?: boolean;
  } => {
    const validated: {
      flow?: string;
      git_error?: string;
      returnTo?: string;
      addKey?: boolean;
    } = {};
    if (typeof search.flow === "string") validated.flow = search.flow;
    if (typeof search.git_error === "string") {
      validated.git_error = search.git_error;
    }
    if (typeof search.returnTo === "string") {
      validated.returnTo = search.returnTo;
    }
    // `addKey` (w2/m66) opens the add-key form on arrival from a RequiresSshKey
    // CTA. It rides the query string (not the fragment) so the SSR render and
    // the client agree — the dialog opens without a post-hydration effect.
    if (search.addKey === true || search.addKey === "true") {
      validated.addKey = true;
    }
    return validated;
  },
  head: ({ match }) => ({
    ...translatedTitleHead("auth.settingsTitle", match),
    styles: [oryThemeStyle],
  }),
});
