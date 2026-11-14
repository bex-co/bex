import { useCallback, useState } from "react";
import { useMutation, useQuery } from "@apollo/client/react";
import { toast } from "sonner";
import {
  SetWorkspaceOverlappingDeployPolicyDocument,
  WorkspaceOverlappingDeployPolicyDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";

/** Render's two Overlapping Deploy Policy values. */
export const OVERLAPPING_DEPLOY_POLICIES = ["wait", "override"] as const;
export type OverlappingDeployPolicy =
  (typeof OVERLAPPING_DEPLOY_POLICIES)[number];

export interface UseOverlappingDeployPolicyResult {
  /** The saved policy, or null while unread or when the read failed. */
  policy: OverlappingDeployPolicy | null;
  /** The first read is in flight and there is nothing to show yet. */
  loading: boolean;
  saving: boolean;
  /** Saves the policy; resolves true on success. */
  save: (policy: OverlappingDeployPolicy) => Promise<boolean>;
}

/**
 * A workspace's Overlapping Deploy Policy (w4/653): Render's Workspace
 * Settings → Overlapping Deploy Policy. Every member reads it; Admins change
 * it. The mutation answers the normalized Workspace, so the card re-renders
 * without a refetch.
 */
export function useOverlappingDeployPolicy(
  workspaceId: string,
): UseOverlappingDeployPolicyResult {
  const { t } = useTranslations();
  const { data, loading } = useQuery(WorkspaceOverlappingDeployPolicyDocument, {
    variables: { id: workspaceId },
    errorPolicy: "all",
  });
  const [mutate] = useMutation(SetWorkspaceOverlappingDeployPolicyDocument);
  const [saving, setSaving] = useState(false);

  const save = useCallback(
    async (policy: OverlappingDeployPolicy) => {
      setSaving(true);
      try {
        await mutate({ variables: { id: workspaceId, policy } });
        toast.success(t("workspaces.overlappingDeployPolicySaved"));
        return true;
      } catch (err) {
        toast.error(
          mutationErrorMessage(
            err,
            t("workspaces.overlappingDeployPolicyError"),
          ),
        );
        return false;
      } finally {
        setSaving(false);
      }
    },
    [mutate, t, workspaceId],
  );

  const saved = data?.workspace?.overlappingDeployPolicy;
  return {
    policy: saved === "wait" || saved === "override" ? saved : null,
    loading: loading && data === undefined,
    saving,
    save,
  };
}
