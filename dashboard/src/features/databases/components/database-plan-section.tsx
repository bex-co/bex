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
import { cn } from "@/common/lib/utils/utils.ts";
import { useTranslations } from "@/common/hooks/use-translations";
import { useDatabaseInstanceTypes } from "@/features/databases/hooks/use-database-instance-types";
import { useUpdateDatabasePlan } from "@/features/databases/hooks/use-update-database-plan";
import {
  formatInstanceCPU,
  formatInstanceMemory,
} from "@/features/services/lib/instance-type";
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
  const { instanceTypes, loading } = useDatabaseInstanceTypes();
  const { updatePlan, busy } = useUpdateDatabasePlan();
  const [selected, setSelected] = useState<string | null>(database.plan);
  const [confirming, setConfirming] = useState(false);

  const selectedType = instanceTypes.find((it) => it.id === selected);
  const canSave = selected != null && selected !== database.plan;

  async function handleConfirm() {
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
        {loading && instanceTypes.length === 0 ? (
          <div className="grid grid-cols-1 gap-2 sm:grid-cols-3">
            {Array.from({ length: 3 }).map((_, i) => (
              <Skeleton key={i} className="h-20 w-full" />
            ))}
          </div>
        ) : (
          <div
            role="radiogroup"
            aria-label={t("databases.planTitle")}
            className="grid grid-cols-1 gap-2 sm:grid-cols-3"
          >
            {instanceTypes.map((it) => {
              const sel = it.id === selected;
              return (
                <button
                  key={it.id}
                  type="button"
                  role="radio"
                  aria-checked={sel}
                  onClick={() => setSelected(it.id)}
                  className={cn(
                    "rounded-lg border p-3 text-left transition-colors",
                    sel
                      ? "border-primary ring-1 ring-primary"
                      : "border-border hover:border-muted-foreground/50",
                  )}
                >
                  <div className="font-medium">{it.name}</div>
                  <div className="text-sm text-muted-foreground">
                    {formatInstanceMemory(it.memory)} RAM ·{" "}
                    {formatInstanceCPU(it.cpu)}
                  </div>
                </button>
              );
            })}
          </div>
        )}

        <div className="flex justify-end gap-2 border-t pt-4">
          <Button
            variant="outline"
            onClick={() => setSelected(database.plan)}
            disabled={!canSave || busy}
          >
            {t("databases.planPickerCancel")}
          </Button>
          <Button
            disabled={!canSave || busy}
            onClick={() => setConfirming(true)}
          >
            {t("databases.planPickerSave")}
          </Button>
        </div>
      </CardContent>

      <AlertDialog open={confirming} onOpenChange={setConfirming}>
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
