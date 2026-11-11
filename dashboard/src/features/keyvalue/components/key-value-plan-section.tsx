import { useState } from "react";
import { PlanCatalogUnavailable } from "@/common/components/plan-catalog-unavailable";
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
import { useSetKeyValuePersistenceMode } from "@/features/keyvalue/hooks/use-set-key-value-persistence-mode";
import { useUpdateKeyValuePlan } from "@/features/keyvalue/hooks/use-update-key-value-plan";
import { PlanCardGrid } from "@/common/components/plan-card-grid";
import {
  KEY_VALUE_PLAN_SLOTS,
  PlanTierSlots,
} from "@/common/components/datastore-plan-preview";
import type { KeyValueView } from "@/features/keyvalue/types";
import {
  keyValueClearPhrase,
  persistenceKeepsData,
} from "@/features/keyvalue/lib/labels";

export interface KeyValuePlanSectionProps {
  keyValue: KeyValueView;
  onChanged: () => void;
}

export function KeyValuePlanSection({
  keyValue,
  onChanged,
}: KeyValuePlanSectionProps) {
  const { t } = useTranslations();
  const capabilities = useCapabilities();
  const { canOperate } = capabilities;
  const operateDenied = !canOperate;
  const operateReasonKey = capabilities.reasonKey("can_operate");
  const operateReason = operateReasonKey ? t(operateReasonKey) : undefined;
  const { instanceTypes, loading, retry, retrying } =
    useKeyValueInstanceTypes();
  const { updatePlan, busy } = useUpdateKeyValuePlan();
  const { mode: persistenceMode } = useSetKeyValuePersistenceMode(keyValue.id);
  // A plan change restarts the only pod; with Persistence Off nothing is on
  // disk to reload, so every key is lost (w4/m263).
  const losesData =
    !keyValue.suspended && !persistenceKeepsData(persistenceMode);
  const currentPlan = keyValue.plan ?? "free";
  const [selected, setSelected] = useState<string>(currentPlan);
  const [confirming, setConfirming] = useState(false);

  // Render: "It is not currently possible to downgrade a Key Value instance"
  // (w8/249), and bex-api refuses the change. The catalog lists plans from
  // the smallest to the largest, so every plan before the current one is
  // smaller. Each such card stays in place, disabled, like a paused plan's.
  const currentIndex = instanceTypes.findIndex((it) => it.id === currentPlan);
  const planCards = instanceTypes.map((it, index) =>
    index < currentIndex
      ? {
          ...it,
          available: false,
          unavailableReason: t("keyvalue.planDowngradeUnavailable"),
        }
      : it,
  );
  const selectedType = planCards.find((it) => it.id === selected);
  // The current plan can rise under an unsaved pick (another tab, a Blueprint
  // sync), which leaves the pick on a card that is now disabled.
  const canSave =
    selected !== keyValue.plan && selectedType?.available !== false;

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
        {loading && instanceTypes.length === 0 ? (
          <PlanTierSlots count={KEY_VALUE_PLAN_SLOTS} />
        ) : instanceTypes.length === 0 ? (
          // A settled failed or empty catalog, not an empty choice (w4/m218).
          <PlanCatalogUnavailable
            onRetry={() => void retry()}
            retrying={retrying}
          />
        ) : (
          <PlanCardGrid
            instanceTypes={planCards}
            value={selected}
            disabled={operateDenied}
            ariaLabel={t("keyvalue.planTitle")}
            onChange={setSelected}
          />
        )}

        <div className="flex justify-end gap-2 border-t pt-4">
          <Button
            variant="outline"
            onClick={() => setSelected(currentPlan)}
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
        description={[
          t("keyvalue.planPickerConfirmBody"),
          losesData
            ? t(
                persistenceMode === "off"
                  ? "keyvalue.restartConfirmDataLoss"
                  : "keyvalue.restartConfirmDataLossUnknown",
                { name: keyValue.name },
              )
            : "",
          selectedType?.monthlyUsd
            ? t("keyvalue.planPickerConfirmPrice", {
                price: selectedType.monthlyUsd,
              })
            : "",
        ]
          .filter(Boolean)
          .join(" ")}
        phrase={losesData ? keyValueClearPhrase(keyValue.name) : undefined}
        cancelLabel={t("keyvalue.planPickerCancel")}
        confirmLabel={t("keyvalue.planPickerSave")}
        // Destructive only when the restart empties a Persistence Off store.
        destructive={losesData}
        onConfirm={() => void handleConfirm()}
      />
    </Card>
  );
}
