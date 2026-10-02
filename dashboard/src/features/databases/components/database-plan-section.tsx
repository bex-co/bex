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
import { ConfirmDialog } from "@/common/components/confirm-dialog";
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
  const capabilities = useCapabilities();
  const { canOperate } = capabilities;
  const operateDenied = !canOperate;
  const operateReasonKey = capabilities.reasonKey("can_operate");
  const operateReason = operateReasonKey ? t(operateReasonKey) : undefined;
  const { instanceTypes, loading } = useDatabaseInstanceTypes();
  const { updatePlan, busy } = useUpdateDatabasePlan();
  const [selected, setSelected] = useState<string | null>(database.plan);
  const [confirming, setConfirming] = useState(false);

  const selectedType = instanceTypes.find((it) => it.id === selected);
  const dirty = selected != null && selected !== database.plan;
  let planError: string | null = null;
  if (dirty && selectedType) {
    const params = { name: selectedType.name };
    if (
      database.highAvailabilityEnabled &&
      !selectedType.supportsHighAvailability
    ) {
      planError = t("databases.planPickerHAUnsupported", params);
    } else if (
      database.diskSizeGB != null &&
      selectedType.maxStorageGB != null &&
      database.diskSizeGB > selectedType.maxStorageGB
    ) {
      planError = t("databases.planPickerStorageUnsupported", {
        ...params,
        current: database.diskSizeGB,
        max: selectedType.maxStorageGB,
      });
    } else if (database.readReplicas.length > selectedType.maxReadReplicas) {
      planError = t("databases.planPickerReplicasUnsupported", {
        ...params,
        max: selectedType.maxReadReplicas,
        current: database.readReplicas.length,
      });
    } else if (
      database.diskAutoscalingEnabled &&
      !selectedType.supportsDiskAutoscaling
    ) {
      planError = t("databases.planPickerAutoscalingUnsupported", params);
    } else if (
      database.poolerEnabled &&
      !selectedType.supportsConnectionPooling
    ) {
      planError = t("databases.planPickerPoolerUnsupported", params);
    }
  }
  const canSave = dirty && selectedType != null && planError == null;

  async function handleConfirm() {
    if (operateDenied || !canSave) return;
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

        {planError ? (
          <p className="text-destructive text-sm" role="alert">
            {planError}
          </p>
        ) : null}

        <div className="flex justify-end gap-2 border-t pt-4">
          <Button
            variant="outline"
            onClick={() => setSelected(database.plan)}
            disabled={!dirty || busy || operateDenied}
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

      <ConfirmDialog
        open={confirming && !operateDenied}
        onOpenChange={(open) => {
          if (!operateDenied) setConfirming(open);
        }}
        title={
          selectedType
            ? t("databases.planPickerConfirmTitle", { name: selectedType.name })
            : ""
        }
        description={
          selectedType?.monthlyUsd
            ? `${t("databases.planPickerConfirmBody")} ${t("databases.planPickerConfirmPrice", { price: selectedType.monthlyUsd })}`
            : t("databases.planPickerConfirmBody")
        }
        cancelLabel={t("databases.planPickerCancel")}
        confirmLabel={t("databases.planPickerSave")}
        // Changing plan is the primary action, not a destructive one.
        destructive={false}
        onConfirm={() => void handleConfirm()}
      />
    </Card>
  );
}
