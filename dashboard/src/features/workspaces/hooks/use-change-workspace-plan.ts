import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { ChangeWorkspacePlanDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  graphQLErrorExtensions,
  hasGraphQLErrorCode,
  mutationErrorMessage,
} from "@/common/lib/graphql-error";
import type { PlanChangeBlocker } from "@/features/workspaces/hooks/use-plan-change-blockers";
import {
  workspacePlanEntry,
  type WorkspacePlanId,
} from "@/features/workspaces/types";

/** ORGANIZATION_DOWNGRADE_BLOCKED's `preconditions` as the plan page's
 *  blockers, dropping anything not shaped as one. */
function refusedPreconditions(
  extensions: Record<string, unknown>,
): PlanChangeBlocker[] {
  const raw = extensions["preconditions"];
  if (!Array.isArray(raw)) return [];
  return raw.flatMap((entry: unknown) => {
    if (typeof entry !== "object" || entry === null) return [];
    const { kind, items, limit } = entry as Record<string, unknown>;
    if (typeof kind !== "string") return [];
    return [
      {
        kind,
        // A refusal raced by a concurrent create names the kind alone.
        items: Array.isArray(items)
          ? items.filter((item): item is string => typeof item === "string")
          : [],
        limit: typeof limit === "number" ? limit : null,
      },
    ];
  });
}

interface PlanChangeRefusal {
  /** The refusal in words, null after a success or before a try. */
  error: string | null;
  /** An org primary's unmet downgrade preconditions, drawn as blockers. */
  preconditions: PlanChangeBlocker[];
}

const NO_REFUSAL: PlanChangeRefusal = { error: null, preconditions: [] };

export interface UseChangeWorkspacePlanResult extends PlanChangeRefusal {
  /** Fires changeWorkspacePlan; resolves true on success. */
  changePlan: (id: string, plan: WorkspacePlanId) => Promise<boolean>;
  busy: boolean;
}

/**
 * The plan pages' `changeWorkspacePlan` (w6/m12, w3/m708; admin only). It
 * relays what bex-api decides, as Render's pages relay theirs: a missing card
 * is Render's "Add a valid card to upgrade to a paid plan.", an org primary's
 * unmet preconditions come back as blockers, and anything else is the
 * server's reason or the page's own fallback (`fallback`: Render's plan page
 * says "Server error during submission. Please try again", its confirmation
 * "Error updating plan."). Success is Render's plan-updated toast.
 */
export function useChangeWorkspacePlan(
  fallback: string,
): UseChangeWorkspacePlanResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(ChangeWorkspacePlanDocument);
  const [busy, setBusy] = useState(false);
  const [refusal, setRefusal] = useState<PlanChangeRefusal>(NO_REFUSAL);

  const changePlan = useCallback(
    async (id: string, plan: WorkspacePlanId) => {
      setBusy(true);
      setRefusal(NO_REFUSAL);
      try {
        await mutate({ variables: { id, plan } });
        const entry = workspacePlanEntry(plan)!;
        const name = t(entry.nameKey);
        toast.success(
          plan === "scale"
            ? t("workspaces.planUpdatedWelcome", { plan: name })
            : t("workspaces.planUpdated", { plan: name }),
          { description: t(entry.descriptionKey) },
        );
        return true;
      } catch (err) {
        const blocked = graphQLErrorExtensions(
          err,
          "ORGANIZATION_DOWNGRADE_BLOCKED",
        );
        setRefusal(
          blocked
            ? {
                error: t("workspaces.updatePlanErrorsSummary"),
                preconditions: refusedPreconditions(blocked),
              }
            : {
                error: hasGraphQLErrorCode(err, "PAYMENT_REQUIRED")
                  ? t("workspaces.updatePlanPaymentRequired")
                  : hasGraphQLErrorCode(err, "ORGANIZATION_PLAN_ON_PRIMARY")
                    ? t("workspaces.orgPlanOnPrimaryError")
                    : mutationErrorMessage(err, fallback),
                preconditions: [],
              },
        );
        return false;
      } finally {
        setBusy(false);
      }
    },
    [fallback, mutate, t],
  );

  return { changePlan, busy, ...refusal };
}
