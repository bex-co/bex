import { useState } from "react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import { Skeleton } from "@/common/components/ui/skeleton";
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
import { PlanCardGrid } from "@/common/components/plan-card-grid";
import { useTranslations } from "@/common/hooks/use-translations";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useDatabaseInstanceTypes } from "@/features/databases/hooks/use-database-instance-types";
import { useUpdateDatabasePlan } from "@/features/databases/hooks/use-update-database-plan";
import type { DatabaseDetailView } from "@/features/databases/types";

export interface DatabasePlanSectionProps {
  database: DatabaseDetailView;
  onChanged: () => void;
}

export function DatabasePlanSection({
  database,
  onChanged,
}: DatabasePlanSectionProps) {
  const { t } = useTranslations();
  const { canOperate, loaded: capabilitiesLoaded } = useCapabilities();
  const operateDenied = capabilitiesLoaded && !canOperate;
  const operateReason = operateDenied
    ? t("capabilities.reasonCanOperate")
    : undefined;
  const { instanceTypes, loading } = useDatabaseInstanceTypes();
  const { updatePlan, busy } = useUpdateDatabasePlan();
  const [selected, setSelected] = useState<string | null>(database.plan);
  const [confirming, setConfirming] = useState(false);

  const selectedType = instanceTypes.find((it) => it.id === selected);
  const canSave = selected != null && selected !== database.plan;

  async function handleConfirm() {
    if (operateDenied) return;
    setConfirming(false);
    if (!selectedType) return;
    const ok = await updatePlan(
      database.id,
      selectedType.id,
      selectedType.name,
    );
    if (ok) onChanged();
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("databases.planTitle")}</CardTitle>
        <CardDescription>{t("databases.planDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {operateReason ? (
          <p className="text-muted-foreground text-sm" role="status">
            {operateReason}
          </p>
        ) : null}
        {loading && instanceTypes.length === 0 ? (
          <div className="grid grid-cols-1 gap-2 sm:grid-cols-3">
            {Array.from({ length: 3 }).map((_, i) => (
              <Skeleton key={i} className="h-20 w-full" />
            ))}
          </div>
        ) : (
          <PlanCardGrid
            instanceTypes={instanceTypes}
            value={selected ?? ""}
            disabled={operateDenied}
            ariaLabel={t("databases.planTitle")}
            onChange={setSelected}
          />
        )}

        <div className="flex justify-end gap-2 border-t pt-4">
          <Button
            variant="outline"
            onClick={() => setSelected(database.plan)}
            disabled={!canSave || busy || operateDenied}
          >
            {t("databases.planPickerCancel")}
          </Button>
          <PermissionTooltip reason={operateReason}>
            <Button
              disabled={!canSave || busy || operateDenied}
              onClick={() => {
                if (!operateDenied) setConfirming(true);
              }}
            >
              {t("databases.planPickerSave")}
            </Button>
          </PermissionTooltip>
        </div>
      </CardContent>

      <AlertDialog
        open={confirming && !operateDenied}
        onOpenChange={(open) => {
          if (!operateDenied) setConfirming(open);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {selectedType &&
                t("databases.planPickerConfirmTitle", {
                  name: selectedType.name,
                })}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("databases.planPickerConfirmBody")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>
              {t("databases.planPickerCancel")}
            </AlertDialogCancel>
            <AlertDialogAction onClick={() => void handleConfirm()}>
              {t("databases.planPickerSave")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  );
}
