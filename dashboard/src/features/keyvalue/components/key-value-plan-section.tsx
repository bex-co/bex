import { useState } from "react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
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
import { useKeyValueInstanceTypes } from "@/features/keyvalue/hooks/use-key-value-instance-types";
import { useUpdateKeyValuePlan } from "@/features/keyvalue/hooks/use-update-key-value-plan";
import { KeyValuePlanPicker } from "@/features/keyvalue/components/key-value-plan-picker";
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
  const { instanceTypes } = useKeyValueInstanceTypes();
  const { updatePlan, busy } = useUpdateKeyValuePlan();
  const [selected, setSelected] = useState<string>(keyValue.plan ?? "free");
  const [confirming, setConfirming] = useState(false);

  const selectedType = instanceTypes.find((it) => it.id === selected);
  const canSave = selected !== keyValue.plan;

  async function handleConfirm() {
    setConfirming(false);
    if (!selectedType) return;
    const ok = await updatePlan(keyValue.id, selectedType.id, selectedType.name);
    if (ok) onChanged();
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("keyvalue.planTitle")}</CardTitle>
        <CardDescription>{t("keyvalue.planDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <KeyValuePlanPicker
          instanceTypes={instanceTypes}
          value={selected}
          onChange={setSelected}
        />

        <div className="flex justify-end gap-2 border-t pt-4">
          <Button
            variant="outline"
            onClick={() => setSelected(keyValue.plan ?? "free")}
            disabled={!canSave || busy}
          >
            {t("keyvalue.planPickerCancel")}
          </Button>
          <Button
            disabled={!canSave || busy}
            onClick={() => setConfirming(true)}
          >
            {t("keyvalue.planPickerSave")}
          </Button>
        </div>
      </CardContent>

      <AlertDialog open={confirming} onOpenChange={setConfirming}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {selectedType &&
                t("keyvalue.planPickerConfirmTitle", {
                  name: selectedType.name,
                })}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("keyvalue.planPickerConfirmBody")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>
              {t("keyvalue.planPickerCancel")}
            </AlertDialogCancel>
            <AlertDialogAction onClick={() => void handleConfirm()}>
              {t("keyvalue.planPickerSave")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  );
}
