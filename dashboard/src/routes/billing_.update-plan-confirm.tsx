import { createFileRoute, redirect } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { translatedTitleHead } from "@/common/lib/document-head";
import { UpdatePlanConfirmPage } from "@/features/workspaces/components/update-plan-confirm-page";
import { UpdatePlanConfirmPageSkeleton } from "@/features/workspaces/components/update-plan-skeletons";
import {
  planFromSearch,
  validateUpdatePlanSearch,
} from "@/features/workspaces/lib/update-plan";

/**
 * Render's plan confirmation (w3/m708). It confirms the plan `?plan=` names;
 * without one there is nothing to confirm, so it goes back to the plan page,
 * as Render's does.
 */
export const Route = createFileRoute("/billing_/update-plan-confirm")({
  staticData: { chrome: true },
  component: UpdatePlanConfirmRoute,
  pendingComponent: UpdatePlanConfirmPageSkeleton,
  validateSearch: validateUpdatePlanSearch,
  beforeLoad: (args) => {
    requireAuth()(args);
    if (!planFromSearch(args.search.plan)) {
      throw redirect({
        to: "/billing/update-plan",
        search: { next: args.search.next },
        replace: true,
      });
    }
  },
  head: ({ match }) =>
    translatedTitleHead("workspaces.updatePlanPageTitle", match),
});

function UpdatePlanConfirmRoute() {
  const search = Route.useSearch();
  const plan = planFromSearch(search.plan);
  return plan ? <UpdatePlanConfirmPage plan={plan} search={search} /> : null;
}
