import { Loader2, RefreshCw } from "lucide-react";
import { useNavigate } from "@tanstack/react-router";
import { useTranslations } from "@/common/hooks/use-translations";
import { Button } from "@/common/components/ui/button";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useCapabilityGate } from "@/features/capabilities/hooks/use-capability-gate";
import { usePlanBlueprintSync } from "@/features/blueprints/hooks/use-plan-blueprint-sync";
import { NO_CHANGES_PLAN_STATE } from "@/features/blueprints/lib/manual-sync";

/**
 * The Blueprint header's one action, Render's **Manual sync** (w3/m677), with
 * a refresh icon after its label. As Render's does, it plans a sync at the
 * branch head and opens that sync's page, where the plan is reviewed and
 * approved (w3/m678); a plan with no changes opens a page saying so. A plan
 * the server refuses goes to `onError`, Render's dismissible Sync Error
 * banner.
 *
 * As on Render it is disabled with the reason for a role that cannot change
 * the Blueprint (bex-api plans a sync under `can_create`).
 */
export function BlueprintManualSync({
  blueprintId,
  linkId,
  onError,
}: {
  /** The Blueprint's exs- id, which the API takes. */
  blueprintId: string;
  /** The Blueprint id as the URL spells it, which the sync's page keeps. */
  linkId: string;
  onError: (reason: string | null) => void;
}) {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const gate = useCapabilityGate("canCreate");
  const { plan, busy } = usePlanBlueprintSync();

  async function manualSync() {
    onError(null);
    const result = await plan(blueprintId);
    if (result.status === "planned") {
      await navigate({
        to: "/blueprints/$blueprintId/sync/$syncId",
        params: { blueprintId: linkId, syncId: result.syncId },
      });
    } else if (result.status === "no_changes") {
      await navigate({
        to: "/blueprints/$blueprintId/sync",
        params: { blueprintId: linkId },
        state: NO_CHANGES_PLAN_STATE,
      });
    } else {
      onError(result.reason);
    }
  }

  return (
    <PermissionTooltip reason={gate.denied ? gate.reason : undefined}>
      <Button
        size="sm"
        onClick={() => void manualSync()}
        disabled={gate.denied || busy}
        aria-busy={busy || undefined}
      >
        {t("blueprints.syncButton")}
        {busy ? (
          <Loader2 aria-hidden="true" className="animate-spin" />
        ) : (
          <RefreshCw aria-hidden="true" />
        )}
      </Button>
    </PermissionTooltip>
  );
}
