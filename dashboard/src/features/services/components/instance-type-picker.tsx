import { RadioDot } from "@/common/components/radio-dot";
import { useState } from "react";
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
import { isCronType } from "@/features/services/lib/service-type";
import {
  computePlanLabel,
  formatInstanceCPU,
  formatInstanceMemory,
  offeredInstanceTypes,
} from "@/features/services/lib/instance-type";
import {
  CronBillingHint,
  INSTANCE_TYPE_CARD_FRAME,
  InstanceTypeCardBody,
  InstanceTypeGroup,
  InstanceTypeGroupsSkeleton,
} from "@/features/services/components/instance-type-groups";
import { ComputePlanName } from "@/common/components/compute-plan-name";
import { cn } from "@/common/lib/utils/utils.ts";
import { Badge } from "@/common/components/ui/badge";
import { CRON_MINIMUM_USD } from "@/common/lib/cron-minimum";

export interface InstanceTypePickerProps {
  serviceId: string;
  /** The App's current plan (Render spelling), or null if untiered. */
  currentPlan: string | null;
  /** The service's type — a paid-only type (background_worker, private_service, cron_job) is never offered Free. */
  serviceType: string | null;
  /** A release has gone live; false means nothing keeps serving (w4/548). */
  live?: boolean;
  /** Leaves edit mode: Cancel, or a change the server accepted. */
  onDone: () => void;
  /** The id of the legacy-Free notice above the picker, when one is shown. */
  describedBy?: string;
}

/**
 * The Compute section's editor, opened by its Edit button: "Select a new
 * compute plan and click Save" (render.com/docs/compute-plans, w6/m483). A
 * card per tier — Free visually separated from the paid ladder (and absent
 * entirely for a paid-only service type) — the current plan pre-selected,
 * Save disabled until the selection changes. Cards show the plan ID beside
 * the legacy name and the catalog monthly price, and the confirmation repeats
 * the price. The custom-instance contact link is omitted (no bex equivalent).
 */
export function InstanceTypePicker({
  serviceId,
  currentPlan,
  serviceType,
  live = true,
  onDone,
  describedBy,
}: InstanceTypePickerProps) {
  const { t } = useTranslations();
  const { instanceTypes, loading, error } = useInstanceTypes();
  const { updatePlan, busy } = useUpdatePlan();
  const [selected, setSelected] = useState<string | null>(currentPlan);
  const [confirming, setConfirming] = useState(false);
  const offered = offeredInstanceTypes(serviceType, instanceTypes);
  // A cron job is billed per second only while a run executes (w4/546).
  const runTimePricing = serviceType !== null && isCronType(serviceType);
  const free = offered.filter((it) => it.id === "free");
  const paid = offered.filter((it) => it.id !== "free");
  // One radio group spanning both sections, in their visual order; a paused
  // tier (w2/m250) is shown disabled and is never a keyboard stop.
  const radio = useCardRadioGroup(
    [...free, ...paid].filter((it) => it.available).map((it) => it.id),
    selected ?? undefined,
    setSelected,
  );

  if (error) {
    return (
      <div className="space-y-4">
        <div className="py-6 text-center">
          <p className="mb-1 font-medium">
            {t("services.planPickerErrorTitle")}
          </p>
          <p className="text-sm text-muted-foreground">
            {t("services.planPickerErrorBody")}
          </p>
        </div>
        <div className="flex justify-end border-t pt-4">
          <Button variant="outline" onClick={onDone}>
            {t("services.planPickerCancel")}
          </Button>
        </div>
      </div>
    );
  }

  const selectedType = offered.find((it) => it.id === selected);
  const canSave =
    selected != null &&
    selected !== currentPlan &&
    selectedType?.available === true;

  async function handleConfirm() {
    setConfirming(false);
    if (!selectedType?.available) return;
    const ok = await updatePlan(
      serviceId,
      selectedType.id,
      computePlanLabel(selectedType),
    );
    if (ok) onDone();
  }

  return (
    <div className="space-y-6">
      <CronBillingHint serviceType={serviceType} />
      {loading && instanceTypes.length === 0 ? (
        <InstanceTypeGroupsSkeleton serviceType={serviceType} />
      ) : (
        <div
          role="radiogroup"
          aria-label={t("services.planPickerTitle")}
          aria-describedby={describedBy}
          className="space-y-4"
        >
          {free.length > 0 && (
            <InstanceTypeGroup
              label={t("services.planPickerFreeGroup")}
              note={t("services.planPickerFreeSmtp")}
            >
              {free.map((it) => (
                <InstanceTypeCard
                  key={it.id}
                  instanceType={it}
                  selected={selected === it.id}
                  current={currentPlan === it.id}
                  onSelect={() => setSelected(it.id)}
                  radio={radio(it.id)}
                  runTimePricing={runTimePricing}
                />
              ))}
            </InstanceTypeGroup>
          )}
          {paid.length > 0 && (
            <InstanceTypeGroup
              label={t("services.planPickerPaidGroup")}
              note={t("services.planPickerPaidSmtp")}
            >
              {paid.map((it) => (
                <InstanceTypeCard
                  key={it.id}
                  instanceType={it}
                  selected={selected === it.id}
                  current={currentPlan === it.id}
                  onSelect={() => setSelected(it.id)}
                  radio={radio(it.id)}
                  runTimePricing={runTimePricing}
                />
              ))}
            </InstanceTypeGroup>
          )}
        </div>
      )}

      <div className="flex justify-end gap-2 border-t pt-4">
        <Button variant="outline" onClick={onDone}>
          {t("services.planPickerCancel")}
        </Button>
        <Button disabled={!canSave || busy} onClick={() => setConfirming(true)}>
          {t("services.planPickerSave")}
        </Button>
      </div>

      <ConfirmDialog
        open={confirming}
        onOpenChange={setConfirming}
        title={
          selectedType
            ? t("services.planPickerConfirmTitle", {
                name: computePlanLabel(selectedType),
              })
            : ""
        }
        description={[
          // A cron job serves no requests; its next run takes the new size.
          t(
            runTimePricing
              ? "services.planPickerConfirmBodyCron"
              : live
                ? "services.planPickerConfirmBody"
                : "services.planPickerConfirmBodyNeverLive",
          ),
          runTimePricing
            ? selectedType?.usdPerMinute &&
              t("services.planPickerConfirmRunPrice", {
                price: selectedType.usdPerMinute,
                minimum: CRON_MINIMUM_USD,
              })
            : selectedType?.monthlyUsd &&
              t("services.planPickerConfirmPrice", {
                price: selectedType.monthlyUsd,
              }),
        ]
          .filter(Boolean)
          .join(" ")}
        cancelLabel={t("services.planPickerCancel")}
        confirmLabel={t("services.planPickerSave")}
        // Changing plan is the primary action here, not a destructive one.
        destructive={false}
        onConfirm={() => void handleConfirm()}
      />
    </div>
  );
}

function InstanceTypeCard({
  instanceType,
  selected,
  current,
  onSelect,
  radio,
  runTimePricing,
}: {
  instanceType: InstanceTypeView;
  selected: boolean;
  /** The service's plan now: badged, so it stays findable once another
   *  card is picked (the selection ring alone moved away with the pick). */
  current: boolean;
  onSelect: () => void;
  radio: CardRadioProps;
  /** A cron job's price is per minute of run time (w4/546). */
  runTimePricing: boolean;
}) {
  const { t } = useTranslations();
  // A paused tier (w2/m250) keeps its card — a service already on it still
  // sees its plan — but disabled, its price row naming why.
  const unavailable = !instanceType.available;
  return (
    <button
      {...radio}
      type="button"
      role="radio"
      aria-checked={selected}
      disabled={unavailable}
      title={unavailable ? instanceType.unavailableReason : undefined}
      onClick={onSelect}
      className={cn(
        INSTANCE_TYPE_CARD_FRAME,
        "transition-colors",
        unavailable
          ? "cursor-not-allowed opacity-60"
          : "hover:border-foreground/50",
        selected ? "border-primary ring-1 ring-primary" : "border-border",
      )}
    >
      <InstanceTypeCardBody
        name={
          // A radio's dot, as the Edge Caching cards draw it, so the cards
          // read as one choice; the current plan carries its badge.
          <span className="flex items-center justify-between gap-2">
            <span className="flex min-w-0 items-center gap-2">
              <ComputePlanName plan={instanceType} className="min-w-0" />
              {current ? (
                <>
                  {/* A space for assistive tech: "Pro Current", not
                      "ProCurrent". */}{" "}
                  <Badge variant="secondary" className="font-normal">
                    {t("services.planCurrent")}
                  </Badge>
                </>
              ) : null}
            </span>
            <RadioDot checked={selected} />
          </span>
        }
        memory={`${formatInstanceMemory(instanceType.memory)} (RAM)`}
        cpu={formatInstanceCPU(instanceType.cpu)}
        price={
          unavailable ? (
            <>
              {t("common.planUnavailable")}
              {instanceType.unavailableReason ? (
                <span className="sr-only">
                  {instanceType.unavailableReason}
                </span>
              ) : null}
            </>
          ) : runTimePricing ? (
            instanceType.usdPerMinute &&
            t("common.pricePerRunMinute", { price: instanceType.usdPerMinute })
          ) : (
            instanceType.monthlyUsd &&
            t("common.pricePerMonth", { price: instanceType.monthlyUsd })
          )
        }
      />
    </button>
  );
}
