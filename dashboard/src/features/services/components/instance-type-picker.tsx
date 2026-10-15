import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  useCardRadioGroup,
  type CardRadioProps,
} from "@/common/hooks/use-card-radio-group";
import {
  useInstanceTypes,
  type InstanceTypeView,
} from "@/features/services/hooks/use-instance-types";
import { useUpdatePlan } from "@/features/services/hooks/use-update-plan";
import {
  formatInstanceCPU,
  formatInstanceMemory,
  offeredInstanceTypes,
} from "@/features/services/lib/instance-type";
import {
  INSTANCE_TYPE_CARD_FRAME,
  InstanceTypeCardBody,
  InstanceTypeGroup,
  InstanceTypeGroupsSkeleton,
} from "@/features/services/components/instance-type-groups";
import { cn } from "@/common/lib/utils/utils.ts";

export interface InstanceTypePickerProps {
  serviceId: string;
  /** The App's current plan (Render spelling), or null if untiered. */
  currentPlan: string | null;
  /** The service's type — a paid-only type (background_worker, private_service, cron_job) is never offered Free. */
  serviceType: string | null;
}

/**
 * Render's "Pick an Instance Type" page (captured live): a card per tier —
 * Free visually separated from the paid ladder (and absent entirely for a
 * paid-only service type) — the current plan pre-selected, Save disabled
 * until the selection changes. bex cards omit price (billing is separate from
 * this resource catalog) and the custom-instance contact link (no bex
 * equivalent).
 */
export function InstanceTypePicker({
  serviceId,
  currentPlan,
  serviceType,
}: InstanceTypePickerProps) {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { instanceTypes, loading, error } = useInstanceTypes();
  const { updatePlan, busy } = useUpdatePlan();
  const [selected, setSelected] = useState<string | null>(currentPlan);
  const [confirming, setConfirming] = useState(false);
  const offered = offeredInstanceTypes(serviceType, instanceTypes);
  const free = offered.filter((it) => it.id === "free");
  const paid = offered.filter((it) => it.id !== "free");
  // One radio group spanning both sections, in their visual order.
  const radio = useCardRadioGroup(
    [...free, ...paid].map((it) => it.id),
    selected ?? undefined,
    setSelected,
  );

  if (error) {
    return (
      <Card>
        <CardContent className="py-6 text-center">
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

  const selectedType = offered.find((it) => it.id === selected);
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
          <InstanceTypeGroupsSkeleton serviceType={serviceType} />
        ) : (
          <div
            role="radiogroup"
            aria-label={t("services.planPickerTitle")}
            className="space-y-4"
          >
            {free.length > 0 && (
              <InstanceTypeGroup label={t("services.planPickerFreeGroup")}>
                {free.map((it) => (
                  <InstanceTypeCard
                    key={it.id}
                    instanceType={it}
                    selected={selected === it.id}
                    onSelect={() => setSelected(it.id)}
                    radio={radio(it.id)}
                  />
                ))}
              </InstanceTypeGroup>
            )}
            {paid.length > 0 && (
              <InstanceTypeGroup label={t("services.planPickerPaidGroup")}>
                {paid.map((it) => (
                  <InstanceTypeCard
                    key={it.id}
                    instanceType={it}
                    selected={selected === it.id}
                    onSelect={() => setSelected(it.id)}
                    radio={radio(it.id)}
                  />
                ))}
              </InstanceTypeGroup>
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

      <ConfirmDialog
        open={confirming}
        onOpenChange={setConfirming}
        title={
          selectedType
            ? t("services.planPickerConfirmTitle", { name: selectedType.name })
            : ""
        }
        description={t("services.planPickerConfirmBody")}
        cancelLabel={t("services.planPickerCancel")}
        confirmLabel={t("services.planPickerSave")}
        // Changing plan is the primary action here, not a destructive one.
        destructive={false}
        onConfirm={() => void handleConfirm()}
      />
    </Card>
  );
}

function InstanceTypeCard({
  instanceType,
  selected,
  onSelect,
  radio,
}: {
  instanceType: InstanceTypeView;
  selected: boolean;
  onSelect: () => void;
  radio: CardRadioProps;
}) {
  return (
    <button
      {...radio}
      type="button"
      role="radio"
      aria-checked={selected}
      onClick={onSelect}
      className={cn(
        INSTANCE_TYPE_CARD_FRAME,
        "transition-colors hover:border-foreground/50",
        selected ? "border-primary ring-1 ring-primary" : "border-border",
      )}
    >
      <InstanceTypeCardBody
        name={instanceType.name}
        memory={`${formatInstanceMemory(instanceType.memory)} (RAM)`}
        cpu={formatInstanceCPU(instanceType.cpu)}
      />
    </button>
  );
}
