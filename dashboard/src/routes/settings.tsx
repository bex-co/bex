import { createFileRoute } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { translatedTitleHead } from "@/common/lib/document-head";
import { oryThemeStyle } from "@/common/lib/ory/theme-styles";
import SettingsPage from "@/features/auth/pages/settings-page";
import { AccountSettingsPageSkeleton } from "@/common/components/route-skeletons";

interface SettingsSearch {
  flow?: string;
  git_error?: string;
  git_claim_selection?: string;
  returnTo?: string;
  addKey?: boolean;
}

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
  // A rejected value comes back undefined: the router merges this over the raw
  // search, so `?git_error=1` would show a GitHub error and `?addKey=1` would
  // open the form.
  validateSearch: (search: Record<string, unknown>): SettingsSearch => ({
    flow: typeof search.flow === "string" ? search.flow : undefined,
    git_error:
      typeof search.git_error === "string" ? search.git_error : undefined,
    // git_claim_selection (ADR078 §3a) is the SUCCESS continuation of an
    // ambiguous claim, not a failure: it names a pending, single-use account
    // choice the callback already proved. Opaque id, rendered only as a lookup
    // key — never interpolated into markup or a navigation target.
    git_claim_selection:
      typeof search.git_claim_selection === "string"
        ? search.git_claim_selection
        : undefined,
    returnTo: typeof search.returnTo === "string" ? search.returnTo : undefined,
    // `addKey` (w2/m66) opens the add-key form on arrival from a RequiresSshKey
    // CTA. It rides the query string (not the fragment) so the SSR render and
    // the client agree — the dialog opens without a post-hydration effect.
    addKey:
      search.addKey === true || search.addKey === "true" ? true : undefined,
  }),
  head: ({ match }) => ({
    ...translatedTitleHead("auth.settingsTitle", match),
    styles: [oryThemeStyle],
  }),
});
