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

/** The kinds of unmet precondition bex-api names (core.Precondition*). */
const DOWNGRADE_PRECONDITION_KINDS = [
  "secondary_workspaces",
  "member_roles",
  "invite_roles",
  "inbound_ip_rules",
] as const;
export type DowngradePreconditionKind =
  (typeof DOWNGRADE_PRECONDITION_KINDS)[number];

/**
 * One of Render's preconditions for downgrading an org's primary below Scale
 * that does not hold yet (ADR115 D6), as ORGANIZATION_DOWNGRADE_BLOCKED's
 * `preconditions` param carries it: what to clear, by kind.
 */
export interface DowngradePrecondition {
  kind: DowngradePreconditionKind;
  /** Workspace names, "subject:role" / "email:role" pairs, or "workspace",
   *  "environment:<name>" and "service:<name>". */
  items: string[];
}

function isPreconditionKind(
  value: unknown,
): value is DowngradePreconditionKind {
  return DOWNGRADE_PRECONDITION_KINDS.some((kind) => kind === value);
}

/** The refusal's preconditions, dropping anything not shaped as one (a kind
 *  this dashboard does not know yet has no copy to show it with). */
function downgradePreconditions(
  extensions: Record<string, unknown>,
): DowngradePrecondition[] {
  const raw = extensions["preconditions"];
  if (!Array.isArray(raw)) return [];
  return raw.flatMap((entry: unknown) => {
    if (typeof entry !== "object" || entry === null) return [];
    const { kind, items } = entry as Record<string, unknown>;
    if (!isPreconditionKind(kind)) return [];
    return [
      {
        kind,
        // A refusal raced by a concurrent create names the kind alone.
        items: Array.isArray(items)
          ? items.filter((item): item is string => typeof item === "string")
          : [],
      },
    ];
  });
}

interface PlanChangeRefusal {
  /**
   * The backend's rejection message (a downgrade guard: member/service/
   * per-user-cap/role-set), shown inline in the change-plan dialog — the DoD
   * calls for a blocked downgrade to say exactly what to remove first. The
   * org refusals (w7/m464) are worded from their codes instead.
   */
  error: string | null;
  /**
   * ORGANIZATION_DOWNGRADE_BLOCKED's unmet preconditions, one line each under
   * `error`; empty for every other outcome.
   */
  preconditions: DowngradePrecondition[];
}

const NO_REFUSAL: PlanChangeRefusal = { error: null, preconditions: [] };

export interface UseChangeWorkspacePlanResult extends PlanChangeRefusal {
  /** Fires changeWorkspacePlan; resolves true on success. */
  changePlan: (id: string, plan: string) => Promise<boolean>;
  busy: boolean;
}

/**
 * Wires the workspace settings plan dialog to bex-api's `changeWorkspacePlan`
 * (w6/m12, admin-only). This hook only relays whatever the backend decides —
 * every downgrade guard (member/service/per-user-cap/role-set, and an org's
 * preconditions) is enforced server-side, never re-implemented client-side.
 */
export function useChangeWorkspacePlan(): UseChangeWorkspacePlanResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(ChangeWorkspacePlanDocument);
  const [busy, setBusy] = useState(false);
  const [refusal, setRefusal] = useState<PlanChangeRefusal>(NO_REFUSAL);

  const changePlan = useCallback(
    async (id: string, plan: string) => {
      setBusy(true);
      setRefusal(NO_REFUSAL);
      try {
        await mutate({ variables: { id, plan } });
        toast.success(t("workspaces.changePlanSuccess", { plan }));
        return true;
      } catch (err) {
        const blocked = graphQLErrorExtensions(
          err,
          "ORGANIZATION_DOWNGRADE_BLOCKED",
        );
        setRefusal(
          blocked
            ? {
                error: t("workspaces.orgDowngradeBlocked"),
                preconditions: downgradePreconditions(blocked),
              }
            : {
                error: hasGraphQLErrorCode(err, "ORGANIZATION_PLAN_ON_PRIMARY")
                  ? t("workspaces.orgPlanOnPrimaryError")
                  : mutationErrorMessage(err, t("workspaces.changePlanError")),
                preconditions: [],
              },
        );
        return false;
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { changePlan, busy, ...refusal };
}
