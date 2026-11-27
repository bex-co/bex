import { useState, useCallback } from "react";
import { useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import { UpdateBlueprintDocument } from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";

/** One Settings row's change: its name, Auto Sync, Branch or Blueprint Path. */
export interface BlueprintUpdate {
  name?: string;
  autoSync?: boolean;
  branch?: string;
  path?: string;
}

/** A save's outcome: the refusal is the caller's to word at its row. */
export type BlueprintUpdateResult =
  { ok: true } | { ok: false; error: unknown };

export interface UseUpdateBlueprintResult {
  update: (
    id: string,
    fields: BlueprintUpdate,
  ) => Promise<BlueprintUpdateResult>;
  busy: boolean;
}

/**
 * Saves a Blueprint's settings through GraphQL `updateBlueprint`. The answer
 * refreshes the cached Blueprint, so the header and every page read the new
 * values. A refusal is returned, not toasted: Render shows it at the row that
 * caused it (w3/m679).
 */
export function useUpdateBlueprint(): UseUpdateBlueprintResult {
  const { t } = useTranslations();
  const [mutate] = useMutation(UpdateBlueprintDocument);
  const [busy, setBusy] = useState(false);

  const update = useCallback(
    async (
      id: string,
      fields: BlueprintUpdate,
    ): Promise<BlueprintUpdateResult> => {
      setBusy(true);
      try {
        await mutate({ variables: { id, ...fields } });
        toast.success(t("blueprints.updateSuccess"));
        return { ok: true };
      } catch (error) {
        return { ok: false, error };
      } finally {
        setBusy(false);
      }
    },
    [mutate, t],
  );

  return { update, busy };
}
