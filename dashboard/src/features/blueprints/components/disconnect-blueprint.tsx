import { useState } from "react";
import { Loader2 } from "lucide-react";
import { useRouter } from "@tanstack/react-router";
import { useTranslations } from "@/common/hooks/use-translations";
import { Alert, AlertDescription } from "@/common/components/ui/alert";
import { Button } from "@/common/components/ui/button";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import {
  aroundPlaceholder,
  PLACEHOLDER_SENTINEL,
} from "@/common/lib/i18n-placeholder";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useCapabilityGate } from "@/features/capabilities/hooks/use-capability-gate";
import { useDisconnectBlueprint } from "@/features/blueprints/hooks/use-disconnect-blueprint";

/**
 * Render's Disconnect Blueprint (w3/m679): a critical button that asks with
 * Render's modal, its three sentences with the name in bold, Cancel and a
 * critical Disconnect Blueprint. A refusal stays in the modal; a disconnect
 * opens `/blueprints`. bex-api disconnects under can_create, as Render gates
 * the button on updateBlueprint, so a role without it gets the button
 * disabled behind the reason.
 */
export function DisconnectBlueprint({
  blueprintId,
  name,
}: {
  blueprintId: string;
  name: string;
}) {
  const { t } = useTranslations();
  const router = useRouter();
  const { disconnect, busy } = useDisconnectBlueprint();
  const gate = useCapabilityGate("canCreate");
  const [open, setOpen] = useState(false);
  const [refusal, setRefusal] = useState<string | null>(null);
  const [before, after] = aroundPlaceholder(
    t("blueprints.disconnectConfirm", { name: PLACEHOLDER_SENTINEL }),
  );

  function handleOpenChange(next: boolean) {
    setOpen(next);
    setRefusal(null);
  }

  async function handleDisconnect() {
    const reason = await disconnect(blueprintId);
    if (reason) {
      setRefusal(reason);
      return;
    }
    setOpen(false);
    void router.navigate({ to: "/blueprints", replace: true });
  }

  return (
    <>
      <PermissionTooltip reason={gate.denied ? gate.reason : undefined}>
        <Button
          variant="destructive"
          disabled={gate.denied}
          onClick={() => handleOpenChange(true)}
        >
          {t("blueprints.disconnectButton")}
        </Button>
      </PermissionTooltip>
      <ConfirmDialog
        open={open}
        onOpenChange={handleOpenChange}
        title={t("blueprints.disconnectTitle")}
        description={
          <>
            <span className="block">{t("blueprints.disconnectNoSync")}</span>
            <span className="mt-4 block">
              {t("blueprints.disconnectKeeps")}
            </span>
            <span className="mt-4 block">
              {before}
              <strong className="text-foreground font-semibold">{name}</strong>
              {after}
            </span>
          </>
        }
        cancelLabel={t("blueprints.disconnectCancel")}
        confirmLabel={
          <>
            {busy ? <Loader2 className="animate-spin" /> : null}
            {t("blueprints.disconnectAction")}
          </>
        }
        pending={busy}
        // A refusal is shown here, so the modal outlives a failed confirm.
        closeOnConfirm={false}
        onConfirm={() => void handleDisconnect()}
      >
        {refusal ? (
          <Alert variant="destructive">
            <AlertDescription>{refusal}</AlertDescription>
          </Alert>
        ) : null}
      </ConfirmDialog>
    </>
  );
}
