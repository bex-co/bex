import { useEffect, useState } from "react";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/common/components/ui/card";
import { Switch } from "@/common/components/ui/switch";
import { Label } from "@/common/components/ui/label";
import { Input } from "@/common/components/ui/input";
import { Button } from "@/common/components/ui/button";
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
  const [draft, setDraft] = useState(current);
  const [confirmEnable, setConfirmEnable] = useState(false);

  // Sync the draft once the detail query resolves (it's null on first render)
  // and whenever the live value changes elsewhere; a user-initiated save makes
  // this a no-op since draft already equals the newly-saved value.
  useEffect(() => {
    setDraft(current);
  }, [current]);

  const uriChanged = draft !== current;

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

        <div className="space-y-1.5">
          <Label htmlFor="maintenance-mode-uri" className="text-sm">
            {t("services.maintenanceModeUriLabel")}
          </Label>
          <div className="flex items-center gap-2">
            <Input
              id="maintenance-mode-uri"
              value={draft}
              disabled={!eligible || busy}
              onChange={(e) => setDraft(e.target.value)}
              placeholder={t("services.maintenanceModeUriPlaceholder")}
              className="font-mono text-sm"
            />
            <Button
              variant="outline"
              size="sm"
              disabled={busy || !eligible || !uriChanged}
              onClick={() => void setMaintenanceMode(serviceId, enabled, draft)}
            >
              {t("services.maintenanceModeSaveUri")}
            </Button>
          </div>
          <p className="text-muted-foreground text-sm">
            {t("services.maintenanceModeUriHint")}
          </p>
        </div>
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
                void setMaintenanceMode(serviceId, true, draft);
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
