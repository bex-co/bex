import { useNavigate, useRouterState } from "@tanstack/react-router";
import { useTranslations } from "@/common/hooks/use-translations";
import { useBillingOnboarding } from "@/features/usage/hooks/use-billing-onboarding";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { useChangeWorkspacePlan } from "@/features/workspaces/hooks/use-change-workspace-plan";
import { updatePlanReturnPath } from "@/features/workspaces/lib/update-plan";
import {
  workspacePlanNameKey,
  type WorkspacePlanId,
} from "@/features/workspaces/types";

/** The plan's name as its card shows it ("Pro"). */
export function usePlanName(plan: string): string {
  const { t } = useTranslations();
  const key = workspacePlanNameKey(plan);
  return key ? t(key) : plan;
}

/**
 * What both of Render's plan pages hold (w3/m708): where Cancel, Back and
 * success go (the URL's `next`, else what the link that opened the page
 * handed over, else home), the card on file with Stripe returning to this
 * same URL, and the plan change, which on success refreshes the workspace and
 * leaves. `fallback` is the page's own words for a refusal without a reason.
 */
export function useUpdatePlanPage(next: string | undefined, fallback: string) {
  const navigate = useNavigate();
  const { refetch } = useWorkspace();
  const afterPlanUpdate = useRouterState({
    select: (s) => s.location.state.afterPlanUpdate,
  });
  const hash = useRouterState({ select: (s) => s.location.hash });
  const href = useRouterState({ select: (s) => s.location.href });
  const returnPath = updatePlanReturnPath(next, afterPlanUpdate, hash);
  const billing = useBillingOnboarding({ returnPath: href });
  const change = useChangeWorkspacePlan(fallback);

  const leave = () => void navigate({ href: returnPath });
  async function apply(workspaceId: string, plan: WorkspacePlanId) {
    if (await change.changePlan(workspaceId, plan)) {
      await refetch();
      leave();
    }
  }
  return { returnPath, billing, change, leave, apply };
}
