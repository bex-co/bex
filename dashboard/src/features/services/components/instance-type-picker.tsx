import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import { PlanPickerGridSkeleton } from "@/common/components/detail-skeletons";
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
import {
  useInstanceTypes,
  type InstanceTypeView,
} from "@/features/services/hooks/use-instance-types";
import { useUpdatePlan } from "@/features/services/hooks/use-update-plan";
import {
  formatInstanceCPU,
  formatInstanceMemory,
} from "@/features/services/lib/instance-type";
import { cn } from "@/common/lib/utils/utils.ts";

export interface InstanceTypePickerProps {
  serviceId: string;
  /** The App's current plan (Render spelling), or null if untiered. */
  currentPlan: string | null;
}

/**
 * Render's "Pick an Instance Type" page (captured live): a card per tier —
 * Free visually separated from the paid ladder — the current plan
 * pre-selected, Save disabled until the selection changes. bex cards omit
 * price (billing is separate from this resource catalog) and the custom-instance
 * contact link (no bex equivalent).
 */
export function InstanceTypePicker({
  serviceId,
  currentPlan,
}: InstanceTypePickerProps) {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { instanceTypes, loading, error } = useInstanceTypes();
  const { updatePlan, busy } = useUpdatePlan();
  const [selected, setSelected] = useState<string | null>(currentPlan);
  const [confirming, setConfirming] = useState(false);

  if (error) {
    return (
      <Card>
        <CardContent className="py-8 text-center">
          <p className="mb-1 font-medium">
            {t("services.planPickerErrorTitle")}
          </p>
          <p className="text-sm text-muted-foreground">
            {t("services.planPickerErrorBody")}
          </p>
        </CardContent>
      </Card>
    );
  }

  const free = instanceTypes.filter((it) => it.id === "free");
  const paid = instanceTypes.filter((it) => it.id !== "free");
  const selectedType = instanceTypes.find((it) => it.id === selected);
  const canSave = selected != null && selected !== currentPlan;

  async function handleConfirm() {
    setConfirming(false);
    if (!selectedType) return;
    const ok = await updatePlan(serviceId, selectedType.id, selectedType.name);
    if (ok) {
      void navigate({
        to: "/services/$serviceId/settings",
        params: { serviceId },
      });
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("services.planPickerTitle")}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-6">
        {loading && instanceTypes.length === 0 ? (
          <PlanPickerGridSkeleton />
        ) : (
          <div
            role="radiogroup"
            aria-label={t("services.planPickerTitle")}
            className="space-y-4"
          >
            {free.length > 0 && (
              <CardGroup label={t("services.planPickerFreeGroup")}>
                {free.map((it) => (
                  <InstanceTypeCard
                    key={it.id}
                    instanceType={it}
                    selected={selected === it.id}
                    onSelect={() => setSelected(it.id)}
                  />
                ))}
              </CardGroup>
            )}
            {paid.length > 0 && (
              <CardGroup label={t("services.planPickerPaidGroup")}>
                {paid.map((it) => (
                  <InstanceTypeCard
                    key={it.id}
                    instanceType={it}
                    selected={selected === it.id}
                    onSelect={() => setSelected(it.id)}
                  />
                ))}
              </CardGroup>
            )}
          </div>
        )}

        <div className="flex justify-end gap-2 border-t pt-4">
          <Button
            variant="outline"
            onClick={() =>
              void navigate({
                to: "/services/$serviceId/settings",
                params: { serviceId },
              })
            }
          >
            {t("services.planPickerCancel")}
          </Button>
          <Button
            disabled={!canSave || busy}
            onClick={() => setConfirming(true)}
          >
            {t("services.planPickerSave")}
          </Button>
        </div>
      </CardContent>

      <AlertDialog open={confirming} onOpenChange={setConfirming}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {selectedType &&
                t("services.planPickerConfirmTitle", {
                  name: selectedType.name,
                })}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("services.planPickerConfirmBody")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>
              {t("services.planPickerCancel")}
            </AlertDialogCancel>
            <AlertDialogAction onClick={() => void handleConfirm()}>
              {t("services.planPickerSave")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  );
}

function CardGroup({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="space-y-2">
      <div className="text-xs font-medium tracking-wide text-muted-foreground uppercase">
        {label}
      </div>
      <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
        {children}
      </div>
    </div>
  );
}

function InstanceTypeCard({
  instanceType,
  selected,
  onSelect,
}: {
  instanceType: InstanceTypeView;
  selected: boolean;
  onSelect: () => void;
}) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={selected}
      onClick={onSelect}
      className={cn(
        "rounded-lg border p-4 text-left transition-colors",
        "hover:border-foreground/50",
        selected ? "border-primary ring-1 ring-primary" : "border-border",
      )}
    >
      <div className="mb-2 font-semibold">{instanceType.name}</div>
      <div className="space-y-0.5 text-sm text-muted-foreground">
        <div>{formatInstanceMemory(instanceType.memory)} (RAM)</div>
        <div>{formatInstanceCPU(instanceType.cpu)}</div>
      </div>
    </button>
  );
}
