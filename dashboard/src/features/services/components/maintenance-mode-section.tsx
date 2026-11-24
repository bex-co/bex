import { useState } from "react";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/common/components/ui/card";
import { Switch } from "@/common/components/ui/switch";
import { Label } from "@/common/components/ui/label";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import { useMaintenanceMode } from "@/features/services/hooks/use-maintenance-mode";
import { EditableFieldRow } from "@/features/services/components/editable-field-row";
import type { MaintenanceModeView } from "@/features/services/types";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useCapabilityGate } from "@/features/capabilities/hooks/use-capability-gate";
import { useServerActions } from "@/features/capabilities/hooks/use-resource-actions";
import { projectionReason } from "@/features/capabilities/lib/resource-actions";
import { useWorkspace } from "@/features/workspaces/context/hooks";

export interface MaintenanceModeSectionProps {
  serviceId: string;
  serviceName: string;
  /** Render exposes maintenance mode only on paid web-service plans. */
  plan?: string | null;
  /** Current spec.maintenanceMode; null while the detail query is still loading. */
  maintenanceMode: MaintenanceModeView | null;
}

/**
 * Settings section for w1/m37's Maintenance Mode: a toggle that takes the
 * service offline behind an interstitial page without suspending it, plus an
 * optional custom-page URL. Enabling confirms first (it takes the service
 * offline for every visitor, like the suspend row-action) — disabling runs
 * immediately, as resume does. web_service only; the Settings page gates
 * rendering by type. The switch, in both directions, follows the service's
 * toggle_maintenance_mode decision, so in a protected environment a Developer
 * reads Render's "This action is limited to admins." instead of a 403 after
 * confirming (w4/725). The page URL toggles nothing; like every service
 * setting it is can_create, so a Contributor reads the role's reason under it
 * (w9/m619).
 */
export function MaintenanceModeSection({
  serviceId,
  serviceName,
  plan,
  maintenanceMode,
}: MaintenanceModeSectionProps) {
  const { t } = useTranslations();
  const { setMaintenanceMode, busy } = useMaintenanceMode();
  const { currentWorkspaceId } = useWorkspace();
  const serverActions = useServerActions(serviceId);
  const { denied: createDenied, reason: createReason } =
    useCapabilityGate("canCreate");
  const enabled = maintenanceMode?.enabled ?? false;
  const eligible = plan !== "free";
  // The plan hint already explains a free service's disabled switch.
  const toggleReason = eligible
    ? projectionReason(
        serverActions,
        currentWorkspaceId,
        serviceId,
        "toggle_maintenance_mode",
        t,
      )
    : undefined;
  // A routine re-check disables the switch for a moment; only a settled
  // refusal replaces the hint, so the line does not flicker.
  const refusal =
    serverActions.status === "checking" ? undefined : toggleReason;
  const current = maintenanceMode?.uri ?? "";
  const [confirmEnable, setConfirmEnable] = useState(false);

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("services.maintenanceModeTitle")}</CardTitle>
        <CardDescription>
          {t("services.maintenanceModeDescription")}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex items-center justify-between gap-4">
          <div>
            <Label htmlFor="maintenance-mode-switch" className="text-sm">
              {enabled
                ? t("services.maintenanceModeEnabled")
                : t("services.maintenanceModeDisabled")}
            </Label>
            <p className="text-muted-foreground mt-1 text-sm">
              {!eligible
                ? t("services.maintenanceModePaidOnly")
                : (refusal ?? t("services.maintenanceModeSwitchHint"))}
            </p>
          </div>
          <PermissionTooltip reason={toggleReason}>
            <Switch
              id="maintenance-mode-switch"
              checked={enabled}
              disabled={busy || !eligible || !!toggleReason}
              onCheckedChange={(checked) => {
                if (checked) {
                  setConfirmEnable(true);
                } else {
                  void setMaintenanceMode(serviceId, false, current);
                }
              }}
              aria-label={t("services.maintenanceModeToggleLabel")}
            />
          </PermissionTooltip>
        </div>

        <EditableFieldRow
          label={t("services.maintenanceModeUriLabel")}
          hint={t("services.maintenanceModeUriHint")}
          value={current}
          editLabel={t("services.maintenanceModeUriEdit")}
          placeholder={t("services.maintenanceModeUriPlaceholder")}
          mono
          busy={busy}
          // The switch's own hint already explains the paid-plan requirement,
          // so the disabled URL field needs no duplicate reason; a role that
          // cannot change settings reads its own.
          disabled={!eligible || createDenied}
          disabledReason={eligible ? createReason : undefined}
          // Changing the URL re-applies with the current on/off state; the URL
          // edit never itself flips maintenance mode.
          onSave={(value) =>
            setMaintenanceMode(serviceId, enabled, value, "uri")
          }
        />
      </CardContent>

      <ConfirmDialog
        open={confirmEnable}
        onOpenChange={setConfirmEnable}
        title={t("services.confirmMaintenanceModeTitle")}
        description={t("services.confirmMaintenanceModeBody", {
          name: serviceName,
        })}
        cancelLabel={t("services.confirmCancel")}
        confirmLabel={t("services.maintenanceModeEnableAction")}
        onConfirm={() => {
          void setMaintenanceMode(serviceId, true, current);
          setConfirmEnable(false);
        }}
      />
    </Card>
  );
}
