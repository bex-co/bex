import { createFileRoute } from "@tanstack/react-router";
import { translatedTitleHead } from "@/common/lib/document-head";
import AuthErrorPage from "@/features/auth/pages/auth-error-page";
import { AuthErrorRouteSkeleton } from "@/common/components/route-skeletons";

// Kratos's error ui_url (deploy/gitops/base/values/kratos.values.yaml,
// common/lib/ory/config.ts).
export const Route = createFileRoute("/auth/error")({
  validateSearch: (search: Record<string, unknown>): { id?: string } => ({
    id: typeof search.id === "string" && search.id ? search.id : undefined,
  }),
  component: AuthErrorRoute,
  pendingComponent: AuthErrorRouteSkeleton,
  head: ({ match }) => translatedTitleHead("auth.errorTitle", match),
});

function AuthErrorRoute() {
  const { id } = Route.useSearch();
  // Keyed by id: a new error starts unread, never showing the last one's text.
  return <AuthErrorPage key={id} id={id} />;
}
