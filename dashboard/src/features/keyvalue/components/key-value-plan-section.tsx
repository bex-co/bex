import { useState } from "react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useKeyValueInstanceTypes } from "@/features/keyvalue/hooks/use-key-value-instance-types";
import { useUpdateKeyValuePlan } from "@/features/keyvalue/hooks/use-update-key-value-plan";
import { PlanCardGrid } from "@/common/components/plan-card-grid";
import type { KeyValueView } from "@/features/keyvalue/types";

export interface KeyValuePlanSectionProps {
  keyValue: KeyValueView;
  onChanged: () => void;
}

export function KeyValuePlanSection({
  keyValue,
  onChanged,
}: KeyValuePlanSectionProps) {
  const { t } = useTranslations();
  const { canOperate, loaded: capabilitiesLoaded } = useCapabilities();
  const operateDenied = capabilitiesLoaded && !canOperate;
  const operateReason = operateDenied
    ? t("capabilities.reasonCanOperate")
    : undefined;
  const { instanceTypes } = useKeyValueInstanceTypes();
  const { updatePlan, busy } = useUpdateKeyValuePlan();
  const [selected, setSelected] = useState<string>(keyValue.plan ?? "free");
  const [confirming, setConfirming] = useState(false);

  const selectedType = instanceTypes.find((it) => it.id === selected);
  const canSave = selected !== keyValue.plan;

  async function handleConfirm() {
    if (operateDenied) return;
    setConfirming(false);
    if (!selectedType) return;
    const ok = await updatePlan(
      keyValue.id,
      selectedType.id,
      selectedType.name,
    );
    if (ok) onChanged();
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("keyvalue.planTitle")}</CardTitle>
        <CardDescription>{t("keyvalue.planDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {operateReason ? (
          <p className="text-muted-foreground text-sm" role="status">
            {operateReason}
          </p>
        ) : null}
        <PlanCardGrid
          instanceTypes={instanceTypes}
          value={selected}
          disabled={operateDenied}
          onChange={setSelected}
        />

        <div className="flex justify-end gap-2 border-t pt-4">
          <Button
            variant="outline"
            onClick={() => setSelected(keyValue.plan ?? "free")}
            disabled={!canSave || busy || operateDenied}
          >
            {t("keyvalue.planPickerCancel")}
          </Button>
          <PermissionTooltip reason={operateReason}>
            <Button
              disabled={!canSave || busy || operateDenied}
              onClick={() => {
                if (!operateDenied) setConfirming(true);
              }}
            >
              {t("keyvalue.planPickerSave")}
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
            ? t("keyvalue.planPickerConfirmTitle", { name: selectedType.name })
            : ""
        }
        description={t("keyvalue.planPickerConfirmBody")}
        cancelLabel={t("keyvalue.planPickerCancel")}
        confirmLabel={t("keyvalue.planPickerSave")}
        // Changing plan is the primary action, not a destructive one.
        destructive={false}
        onConfirm={() => void handleConfirm()}
      />
    </Card>
  );
}
