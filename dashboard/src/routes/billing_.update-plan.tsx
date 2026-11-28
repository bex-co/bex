import { createFileRoute } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { translatedTitleHead } from "@/common/lib/document-head";
import { UpdatePlanPage } from "@/features/workspaces/components/update-plan-page";
import { UpdatePlanPageSkeleton } from "@/features/workspaces/components/update-plan-skeletons";
import { validateUpdatePlanSearch } from "@/features/workspaces/lib/update-plan";

/**
 * Render's Change plan page (w3/m708), where every upgrade link and Change
 * plan button in the dashboard lands: `?plan=` chooses a card by Render's
 * aliases and `?next=` is where Cancel and success go. Render's
 * `/w/<tea-id>/billing/update-plan` reaches it through `w.$`.
 */
export const Route = createFileRoute("/billing_/update-plan")({
  staticData: { chrome: true },
  component: UpdatePlanRoute,
  pendingComponent: UpdatePlanPageSkeleton,
  // No-arg requireAuth: `next` keeps the full href, `?plan=` included.
  beforeLoad: requireAuth(),
  validateSearch: validateUpdatePlanSearch,
  head: ({ match }) =>
    translatedTitleHead("workspaces.updatePlanPageTitle", match),
});

function UpdatePlanRoute() {
  return <UpdatePlanPage search={Route.useSearch()} />;
}
