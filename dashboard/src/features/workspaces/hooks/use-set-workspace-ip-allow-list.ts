import { useCallback, useState } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { SetWorkspaceIpAllowListDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  isForbiddenError,
  mutationErrorMessage,
  planFeatureRequired,
} from "@/common/lib/graphql-error";
import type { IPAllowListEntryDraft } from "@/common/lib/ip-allow-list";

/**
 * Why bex-api refused a workspace rule change, in the shapes the card words
 * differently: the plan does not include the feature (below Scale), the caller
 * is not an Admin, or anything else with the server's own reason.
 */
export type WorkspaceIPRulesRefusal =
  | { kind: "plan" }
  | { kind: "forbidden" }
  | { kind: "other"; message: string };

export interface UseSetWorkspaceIPAllowListResult {
  /** Full-replaces the workspace's rules; resolves true on success. */
  save: (id: string, entries: IPAllowListEntryDraft[]) => Promise<boolean>;
  saving: boolean;
  /** The last save's refusal, shown inline under the rules; null after a save starts. */
  refusal: WorkspaceIPRulesRefusal | null;
}

/**
 * Wires the Workspace Settings → Inbound IP Rules card to bex-api's
 * `setWorkspaceIPAllowList` (w9/m388). The result carries the saved list on
 * the normalized Workspace entity, so the card's `WorkspaceIPAllowList` read
 * shows it without a refetch.
 */
export function useSetWorkspaceIPAllowList(): UseSetWorkspaceIPAllowListResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(SetWorkspaceIpAllowListDocument);
  const [saving, setSaving] = useState(false);
  const [refusal, setRefusal] = useState<WorkspaceIPRulesRefusal | null>(null);

  const save = useCallback(
    async (id: string, entries: IPAllowListEntryDraft[]) => {
      setSaving(true);
      setRefusal(null);
      try {
        await mutate({
          variables: {
            id,
            ipAllowList: entries.map((entry) => ({
              cidrBlock: entry.cidrBlock.trim(),
              description: entry.description.trim(),
            })),
          },
        });
        toast.success(t("workspaces.ipRulesSaved"));
        return true;
      } catch (err) {
        setRefusal(
          planFeatureRequired(err, "inboundIPRules")
            ? { kind: "plan" }
            : isForbiddenError(err)
              ? { kind: "forbidden" }
              : {
                  kind: "other",
                  message: mutationErrorMessage(
                    err,
                    t("workspaces.ipRulesSaveError"),
                  ),
                },
        );
        return false;
      } finally {
        setSaving(false);
      }
    },
    [mutate, t],
  );

  return { save, saving, refusal };
}
