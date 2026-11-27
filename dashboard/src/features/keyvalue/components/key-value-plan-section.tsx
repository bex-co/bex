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
import { ReviewChangesDialog } from "@/common/components/review-changes-dialog";
import { useComputePlanFigures } from "@/common/hooks/use-compute-plan-figures";
import { useTranslations } from "@/common/hooks/use-translations";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useCapabilityGate } from "@/features/capabilities/hooks/use-capability-gate";
import { useKeyValueInstanceTypes } from "@/features/keyvalue/hooks/use-key-value-instance-types";
import { useSetKeyValuePersistenceMode } from "@/features/keyvalue/hooks/use-set-key-value-persistence-mode";
import { useUpdateKeyValuePlan } from "@/features/keyvalue/hooks/use-update-key-value-plan";
import { ComputePlanList } from "@/common/components/compute-plan-list";
import { planPriceLabel } from "@/common/lib/plan-list";
import {
  KEY_VALUE_PLAN_SLOTS,
  PlanTierSlots,
} from "@/common/components/datastore-plan-preview";
import type { KeyValueView } from "@/features/keyvalue/types";
import {
  keyValueClearPhrase,
  persistenceKeepsData,
} from "@/features/keyvalue/lib/labels";
import { isPlanDowngrade } from "@/features/keyvalue/lib/plan-downgrade";

export interface KeyValuePlanSectionProps {
  keyValue: KeyValueView;
  onChanged: () => void;
}

export function KeyValuePlanSection({
  keyValue,
  onChanged,
}: KeyValuePlanSectionProps) {
  const { t } = useTranslations();
  const { denied: createDenied, reason: createReason } =
    useCapabilityGate("canCreate");
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
  const [reviewing, setReviewing] = useState(false);
  const [reviewError, setReviewError] = useState<string | null>(null);
  const figures = useComputePlanFigures();

  // Render: "It is not currently possible to downgrade a Key Value instance"
  // (w8/249), and bex-api refuses the change. A smaller plan is one with less
  // memory — bex-api's own comparison (keyvalue.ErrPlanDowngrade), read from
  // the catalog it sends, not from the catalog's list order. Each such card
  // stays in place, disabled, like a paused plan's.
  const currentType = instanceTypes.find((it) => it.id === currentPlan);
  const planCards = instanceTypes.map((it) =>
    isPlanDowngrade(it, currentType)
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
    if (createDenied || !selectedType) return;
    setReviewError(null);
    const result = await updatePlan(
      keyValue.id,
      selectedType.id,
      selectedType.name,
    );
    if (result.ok) {
      setReviewing(false);
      onChanged();
    } else {
      setReviewError(result.error ?? null);
    }
  }

  // Render's Key Value review lists the plan alone, as the plan list prices
  // it: Render bills no Key Value storage line.
  const price = (selectedType && planPriceLabel(selectedType, false, t)) ?? "—";

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("keyvalue.planTitle")}</CardTitle>
        <CardDescription>{t("keyvalue.planDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {createReason ? (
          <p className="text-muted-foreground text-sm" role="status">
            {createReason}
          </p>
        ) : null}
        {loading && instanceTypes.length === 0 ? (
          <PlanTierSlots count={KEY_VALUE_PLAN_SLOTS} connections />
        ) : instanceTypes.length === 0 ? (
          // A settled failed or empty catalog, not an empty choice (w4/m218).
          <PlanCatalogUnavailable
            onRetry={() => void retry()}
            retrying={retrying}
          />
        ) : (
          <ComputePlanList
            instanceTypes={planCards}
            value={selected}
            disabled={createDenied}
            ariaLabel={t("keyvalue.planTitle")}
            onChange={setSelected}
          />
        )}

        <div className="flex justify-end gap-2 border-t pt-4">
          <Button
            variant="outline"
            onClick={() => setSelected(currentPlan)}
            disabled={!canSave || busy || createDenied}
          >
            {t("keyvalue.planPickerCancel")}
          </Button>
          <PermissionTooltip reason={createReason}>
            <Button
              disabled={!canSave || busy || createDenied}
              onClick={() => {
                if (createDenied) return;
                setReviewError(null);
                setReviewing(true);
              }}
            >
              {t("keyvalue.planPickerSave")}
            </Button>
          </PermissionTooltip>
        </div>
      </CardContent>

      {selectedType ? (
        <ReviewChangesDialog
          open={reviewing && !createDenied}
          description={t("keyvalue.reviewComputeLead")}
          lineItems={[
            {
              id: "compute-plan",
              label: t("keyvalue.planTitle"),
              values: figures(selectedType),
              price,
              // Edit returns to the picker with the choice kept.
              onEdit: () => setReviewing(false),
              editLabel: t("common.reviewEditComputePlan"),
            },
          ]}
          total={price}
          confirmLabel={t("keyvalue.reviewUpgradeCompute")}
          // Render's warning, and bex's data-loss sentence whenever the
          // restart empties the store: Render's Free instances have no
          // persistence, bex's lose data only with Persistence Off (ADR021).
          warning={[
            t("keyvalue.reviewUnavailable"),
            losesData
              ? t(
                  persistenceMode === "off"
                    ? "keyvalue.restartConfirmDataLoss"
                    : "keyvalue.restartConfirmDataLossUnknown",
                  { name: keyValue.name },
                )
              : "",
          ]
            .filter(Boolean)
            .join(" ")}
          phrase={losesData ? keyValueClearPhrase(keyValue.name) : undefined}
          pending={busy}
          error={reviewError}
          onConfirm={() => void handleConfirm()}
          onClose={() => setReviewing(false)}
        />
      ) : null}
    </Card>
  );
}
