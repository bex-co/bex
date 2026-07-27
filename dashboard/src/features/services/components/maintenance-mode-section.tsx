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
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/common/components/ui/alert-dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import { useMaintenanceMode } from "@/features/services/hooks/use-maintenance-mode";
import { EditableFieldRow } from "@/features/services/components/editable-field-row";
import type { MaintenanceModeView } from "@/features/services/types";

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
 * immediately, mirroring resume's asymmetric guard. web_service only; the
 * Settings page gates rendering by type.
 */
export function MaintenanceModeSection({
  serviceId,
  serviceName,
  plan,
  maintenanceMode,
}: MaintenanceModeSectionProps) {
  const { t } = useTranslations();
  const { setMaintenanceMode, busy } = useMaintenanceMode();
  const enabled = maintenanceMode?.enabled ?? false;
  const eligible = plan !== "free";
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
              {eligible
                ? t("services.maintenanceModeSwitchHint")
                : t("services.maintenanceModePaidOnly")}
            </p>
          </div>
          <Switch
            id="maintenance-mode-switch"
            checked={enabled}
            disabled={busy || !eligible}
            onCheckedChange={(checked) => {
              if (checked) {
                setConfirmEnable(true);
              } else {
                void setMaintenanceMode(serviceId, false, current);
              }
            }}
            aria-label={t("services.maintenanceModeToggleLabel")}
          />
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
          // so the disabled URL field needs no duplicate reason.
          disabled={!eligible}
          // Changing the URL re-applies with the current on/off state; the URL
          // edit never itself flips maintenance mode.
          onSave={(value) => setMaintenanceMode(serviceId, enabled, value)}
        />
      </CardContent>

      <AlertDialog open={confirmEnable} onOpenChange={setConfirmEnable}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t("services.confirmMaintenanceModeTitle")}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("services.confirmMaintenanceModeBody", {
                name: serviceName,
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>
              {t("services.confirmCancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                void setMaintenanceMode(serviceId, true, current);
                setConfirmEnable(false);
              }}
            >
              {t("services.maintenanceModeEnableAction")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  );
}
