import { useQuery } from "@apollo/client/react";
import { WorkspacePlanChangeBlockersDocument } from "@/graphql/definitions";

/** One refusal changeWorkspacePlan would give (bex-api's
 *  workspacePlanChangeBlockers, w3/m708). */
export interface PlanChangeBlocker {
  kind: string;
  /** What to clear: workspace names, "email:role" pairs, or the holders of
   *  inbound IP rules; empty for a counted kind. */
  items: string[];
  /** The target plan's own number the kind is about, when it has one. */
  limit: number | null;
}

export interface UsePlanChangeBlockersResult {
  blockers: PlanChangeBlocker[];
  /** True until the blockers for the asked plan are known. */
  loading: boolean;
}

/**
 * What keeps the workspace from moving to `plan`, by bex-api's own checks, so
 * the plan page can name each blocker before Select plan as Render's does.
 * Skipped (`plan` null) when nothing is asked: no plan chosen, the current
 * plan, or a caller who cannot change it. A failed read lists nothing: the
 * mutation still refuses, and its refusal is shown where it is sent.
 */
export function usePlanChangeBlockers(
  workspaceId: string | undefined,
  plan: string | null,
): UsePlanChangeBlockersResult {
  const skip = !workspaceId || plan === null;
  const { data, loading } = useQuery(WorkspacePlanChangeBlockersDocument, {
    variables: { id: workspaceId ?? "", plan: plan ?? "" },
    skip,
    fetchPolicy: "network-only",
    errorPolicy: "all",
  });
  return {
    blockers: skip ? [] : (data?.workspacePlanChangeBlockers ?? []),
    loading: !skip && loading,
  };
}
