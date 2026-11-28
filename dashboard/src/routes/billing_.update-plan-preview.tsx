import { createFileRoute, redirect } from "@tanstack/react-router";
import {
  planFromSearch,
  validateUpdatePlanSearch,
} from "@/features/workspaces/lib/update-plan";

/**
 * Render's plan preview (w3/m708) chooses which custom domains stay active
 * when the new plan caps them hard: its plan page opens it only for that, a
 * Hobby target on the legacy plans' hard limit of two. bex's plans include
 * custom domains and meter the ones above (ADR030 §16), a soft limit, so its
 * plan page never opens the preview: a link to it goes on to the
 * confirmation with the same plan, and one with no plan to the plan page,
 * where Render's also sends it (docs/render-artifacts/workspace-plan-change.md
 * § Preview).
 */
export const Route = createFileRoute("/billing_/update-plan-preview")({
  validateSearch: validateUpdatePlanSearch,
  beforeLoad: ({ search }) => {
    throw redirect(
      planFromSearch(search.plan)
        ? { to: "/billing/update-plan-confirm", search, replace: true }
        : {
            to: "/billing/update-plan",
            search: { next: search.next },
            replace: true,
          },
    );
  },
});
