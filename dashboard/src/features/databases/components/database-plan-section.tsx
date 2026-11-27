import { useRef, useState } from "react";
import { PlanCatalogUnavailable } from "@/common/components/plan-catalog-unavailable";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Label } from "@/common/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import {
  ReviewChangesDialog,
  type ReviewLineItem,
} from "@/common/components/review-changes-dialog";
import { useComputePlanFigures } from "@/common/hooks/use-compute-plan-figures";
import { ComputePlanList } from "@/common/components/compute-plan-list";
import {
  PlanTierSlots,
  POSTGRES_PLAN_SLOTS,
} from "@/common/components/datastore-plan-preview";
import { useTranslations } from "@/common/hooks/use-translations";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useCapabilityGate } from "@/features/capabilities/hooks/use-capability-gate";
import { useDatabaseInstanceTypes } from "@/features/databases/hooks/use-database-instance-types";
import { useUpdateDatabasePlan } from "@/features/databases/hooks/use-update-database-plan";
import { useUpdateDatabaseDiskSize } from "@/features/databases/hooks/use-update-database-disk-size";
import { DatabaseDiskAutoscalingControl } from "@/features/databases/components/database-disk-autoscaling-control";
import {
  STORAGE_STEP_GB,
  databaseStorageGB,
  planStorageFixed,
  storageGrowProblem,
} from "@/features/databases/lib/storage";
import { highAvailabilityRequested } from "@/features/databases/lib/status";
import {
  centsToUsd,
  databaseReviewPricing,
  usdToCents,
} from "@/features/databases/lib/plan-review";
import type {
  DatabaseDetailView,
  DatabaseInstanceTypeView,
} from "@/features/databases/types";

export interface DatabasePlanSectionProps {
  database: DatabaseDetailView;
  onChanged: () => void;
}

export function DatabasePlanSection({
  database,
  onChanged,
}: DatabasePlanSectionProps) {
  const { t } = useTranslations();
  const { denied: createDenied, reason: createReason } =
    useCapabilityGate("canCreate");
  const { instanceTypes, loading, retry, retrying } =
    useDatabaseInstanceTypes();
  const { updatePlan, busy } = useUpdateDatabasePlan();
  const [selected, setSelected] = useState<string | null>(database.plan);
  const [reviewing, setReviewing] = useState(false);
  const [reviewError, setReviewError] = useState<string | null>(null);
  const reviewLines = useDatabaseReviewLines(database);
  const haEdit = useEditHighAvailability();

  const selectedType = instanceTypes.find((it) => it.id === selected);
  const dirty = selected != null && selected !== database.plan;
  let planError: string | null = null;
  if (dirty && selectedType) {
    const params = { name: selectedType.name };
    // The spec intent, as bex-api's rule checks it: a standby still starting
    // already blocks a move below 1 CPU (w2/m171).
    if (
      highAvailabilityRequested(database) &&
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
    if (createDenied || !canSave || !selectedType) return;
    setReviewError(null);
    const result = await updatePlan(
      database.id,
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

  const currentType = instanceTypes.find((it) => it.id === database.plan);
  // Render's review of a compute change (w3/m674): the new plan, the standby
  // and replicas it also prices, and the storage the change leaves, which a
  // plan's larger floor raises (EffectiveStorageGB). A plan whose storage is
  // included (Free) has no Storage line.
  const review =
    selectedType &&
    reviewLines({
      plan: selectedType,
      storageGB:
        usdToCents(selectedType.storageMonthlyUsdPerGB) === 0
          ? null
          : Math.max(
              database.diskSizeGB ?? currentType?.storageGB ?? 0,
              selectedType.storageGB,
            ),
      // Edit returns to the picker with the choice kept.
      onEditPlan: () => setReviewing(false),
      onEditHighAvailability: () =>
        haEdit.edit(() => {
          setSelected(database.plan);
          setReviewing(false);
        }),
    });
  const replicas = database.readReplicas.length > 0;
  const seconds = highAvailabilityRequested(database);

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("databases.planTitle")}</CardTitle>
        <CardDescription>{t("databases.planDescription")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {createReason ? (
          <p className="text-muted-foreground text-sm" role="status">
            {createReason}
          </p>
        ) : null}
        {loading && instanceTypes.length === 0 ? (
          <PlanTierSlots count={POSTGRES_PLAN_SLOTS} />
        ) : instanceTypes.length === 0 ? (
          // A settled failed or empty catalog, not an empty choice (w4/m218).
          <PlanCatalogUnavailable
            onRetry={() => void retry()}
            retrying={retrying}
          />
        ) : (
          <ComputePlanList
            instanceTypes={instanceTypes}
            value={selected ?? ""}
            disabled={createDenied}
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
            disabled={!dirty || busy || createDenied}
          >
            {t("databases.planPickerCancel")}
          </Button>
          <PermissionTooltip reason={createReason}>
            <Button
              disabled={!canSave || busy || createDenied}
              onClick={() => {
                if (createDenied) return;
                setReviewError(null);
                haEdit.clear();
                setReviewing(true);
              }}
            >
              {t("databases.planPickerSave")}
            </Button>
          </PermissionTooltip>
        </div>

        <StorageRow
          database={database}
          plan={instanceTypes.find((it) => it.id === database.plan)}
          onChanged={onChanged}
        />
        <DatabaseDiskAutoscalingControl
          database={database}
          onChanged={onChanged}
        />
      </CardContent>

      {review ? (
        <ReviewChangesDialog
          open={reviewing && !createDenied}
          description={t("databases.reviewComputeLead")}
          lineItems={review.lines}
          total={review.total}
          confirmLabel={t("databases.reviewUpdateCompute")}
          // Render's warning: seconds when a standby takes over, minutes when
          // the only instance restarts; the replicas restart too.
          warning={t(
            replicas
              ? seconds
                ? "databases.reviewUnavailableReplicasSeconds"
                : "databases.reviewUnavailableReplicasMinutes"
              : seconds
                ? "databases.reviewUnavailableSeconds"
                : "databases.reviewUnavailableMinutes",
          )}
          pending={busy}
          error={reviewError}
          onConfirm={() => void handleConfirm()}
          onClose={() => setReviewing(false)}
          returnFocusRef={haEdit.returnFocusRef}
        />
      ) : null}
    </Card>
  );
}

/**
 * Render's Plan page Storage section (w2/m307): the allocated size and an Edit
 * that grows it — any multiple of 5 GB above the current size, up to the
 * plan's maximum. Storage never shrinks, and a free plan's is fixed, as
 * bex-api's admission rules (and Render) have it.
 */
function StorageRow({
  database,
  plan,
  onChanged,
}: {
  database: DatabaseDetailView;
  /** The database's current plan, or undefined while the catalog loads. */
  plan: DatabaseInstanceTypeView | undefined;
  onChanged: () => void;
}) {
  const { t } = useTranslations();
  const { denied: createDenied, reason: createReason } =
    useCapabilityGate("canCreate");
  const { updateDiskSize, busy } = useUpdateDatabaseDiskSize();
  const [editing, setEditing] = useState(false);
  const [reviewing, setReviewing] = useState(false);
  const [value, setValue] = useState("");
  const [reviewError, setReviewError] = useState<string | null>(null);
  const reviewLines = useDatabaseReviewLines(database);
  const haEdit = useEditHighAvailability();

  const current = plan
    ? databaseStorageGB(database, plan)
    : (database.diskSizeGB ?? 0);
  const max = plan?.maxStorageGB ?? null;
  const fixed = plan != null && planStorageFixed(plan);
  const problem =
    value.trim() === "" ? null : storageGrowProblem(value, current, max);
  const canSave = value.trim() !== "" && problem == null;
  const fieldError =
    problem === "step"
      ? t("databases.storageEditStep", { step: STORAGE_STEP_GB })
      : problem === "notGreater"
        ? t("databases.storageEditNotGreater", { current })
        : problem === "tooLarge"
          ? t("databases.storageEditTooLarge", {
              name: plan?.name ?? "",
              max: max ?? "",
            })
          : undefined;
  // The smallest size it accepts, which also anchors the field's step.
  const next = (Math.floor(current / STORAGE_STEP_GB) + 1) * STORAGE_STEP_GB;

  function startEditing() {
    setValue(max == null || next <= max ? String(next) : "");
    setEditing(true);
  }

  // Save opens Render's storage review; Edit storage and Cancel return to
  // the field with the size kept.
  function review() {
    if (!canSave) return;
    setReviewError(null);
    haEdit.clear();
    setEditing(false);
    setReviewing(true);
  }

  function resumeEditing() {
    setReviewing(false);
    setEditing(true);
  }

  async function save() {
    if (!canSave) return;
    setReviewError(null);
    const message = await updateDiskSize(database.id, Number(value));
    if (message) {
      setReviewError(message);
      return;
    }
    setReviewing(false);
    onChanged();
  }

  // Render's storage review (w3/m674): the plan as it stands, the standby and
  // replicas, and the new size on every instance the database bills.
  const storageReview =
    plan && canSave
      ? reviewLines({
          plan,
          storageGB: Number(value),
          onEditStorage: resumeEditing,
          onEditHighAvailability: () => haEdit.edit(() => setReviewing(false)),
        })
      : undefined;

  return (
    // `#storage` is Render's anchor for this control: its almost-full
    // warning links `/plan#storage` (w3/m656).
    <div
      id="storage"
      className="flex scroll-mt-6 items-center justify-between gap-4 border-t pt-4"
    >
      <div className="min-w-0 space-y-1">
        <p className="text-sm font-medium">{t("databases.storageTitle")}</p>
        <p className="text-muted-foreground text-sm">
          {current > 0 ? t("databases.storageSize", { size: current }) : "—"}
        </p>
        {fixed ? (
          <p className="text-muted-foreground text-sm">
            {t("databases.storageFixed")}
          </p>
        ) : null}
      </div>
      <PermissionTooltip reason={createReason}>
        <Button
          variant="outline"
          size="sm"
          aria-label={t("databases.storageEditAria")}
          // Until the catalog names the plan, a free database's fixed disk
          // is unknown, so the edit waits for it.
          disabled={createDenied || plan == null || fixed || busy}
          onClick={startEditing}
        >
          {t("databases.storageEdit")}
        </Button>
      </PermissionTooltip>

      <Dialog open={editing} onOpenChange={setEditing}>
        <DialogContent>
          {/* noValidate: the field's own messages explain a refusal, in the
              user's language, instead of the browser's constraint bubble. */}
          <form
            noValidate
            onSubmit={(e) => {
              e.preventDefault();
              review();
            }}
            className="space-y-4"
          >
            <DialogHeader>
              <DialogTitle>
                {t("databases.storageEditTitle", { name: database.name })}
              </DialogTitle>
              <DialogDescription>
                {t("databases.storageEditBody", {
                  step: STORAGE_STEP_GB,
                  current,
                })}
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-2">
              <Label htmlFor="database-storage-size">
                {t("databases.storageEditLabel")}
              </Label>
              <Input
                id="database-storage-size"
                type="number"
                inputMode="numeric"
                min={next}
                max={max ?? undefined}
                step={STORAGE_STEP_GB}
                value={value}
                onChange={(e) => setValue(e.target.value)}
                aria-invalid={fieldError != null}
                aria-describedby={
                  fieldError ? "database-storage-size-error" : undefined
                }
              />
              {fieldError ? (
                <p
                  id="database-storage-size-error"
                  className="text-destructive text-sm"
                  role="alert"
                >
                  {fieldError}
                </p>
              ) : null}
            </div>
            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={() => setEditing(false)}
              >
                {t("common.cancel")}
              </Button>
              <Button type="submit" disabled={!canSave}>
                {t("databases.storageEditSave")}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {storageReview ? (
        <ReviewChangesDialog
          open={reviewing}
          description={t("databases.reviewStorageLead")}
          lineItems={storageReview.lines}
          total={storageReview.total}
          confirmLabel={t("databases.reviewUpdateStorage")}
          pending={busy}
          error={reviewError}
          onConfirm={() => void save()}
          onClose={resumeEditing}
          returnFocusRef={haEdit.returnFocusRef}
        />
      ) : null}
    </div>
  );
}

/**
 * The lines of Render's Postgres review (w3/m674): Compute plan, High
 * availability when it is on, Read replicas when there are any, and Storage
 * when the plan bills it, each priced by databaseReviewPricing, then the
 * Total. An `onEdit…` puts the Edit beside its line.
 */
function useDatabaseReviewLines(database: DatabaseDetailView) {
  const { t } = useTranslations();
  const figures = useComputePlanFigures();
  // "—" for a plan the sheet does not price.
  const price = (cents: number | null) =>
    cents == null
      ? "—"
      : t("common.pricePerMonth", { price: centsToUsd(cents) });
  return ({
    plan,
    storageGB,
    onEditPlan,
    onEditHighAvailability,
    onEditStorage,
  }: {
    plan: DatabaseInstanceTypeView;
    storageGB: number | null;
    onEditPlan?: () => void;
    onEditHighAvailability: () => void;
    onEditStorage?: () => void;
  }) => {
    const highAvailability = highAvailabilityRequested(database);
    const readReplicas = database.readReplicas.length;
    const pricing = databaseReviewPricing({
      planMonthlyUsd: plan.monthlyUsd,
      storageMonthlyUsdPerGB: plan.storageMonthlyUsdPerGB,
      highAvailability,
      readReplicas,
      storageGB,
    });
    const lines: ReviewLineItem[] = [
      {
        id: "compute-plan",
        label: t("databases.planTitle"),
        values: figures(plan),
        price: price(pricing.planCents),
        onEdit: onEditPlan,
        editLabel: t("common.reviewEditComputePlan"),
      },
    ];
    if (highAvailability) {
      lines.push({
        id: "high-availability",
        label: t("databases.haTitle"),
        values: [t("databases.haEnabled")],
        price: price(pricing.highAvailabilityCents),
        onEdit: onEditHighAvailability,
        editLabel: t("databases.reviewEditHighAvailability"),
      });
    }
    if (readReplicas > 0) {
      lines.push({
        id: "read-replicas",
        label: t("databases.haReadReplicas"),
        values: [t("databases.reviewReplicaCount", { count: readReplicas })],
        price: price(pricing.readReplicasCents),
      });
    }
    if (storageGB != null) {
      lines.push({
        id: "storage",
        label: t("databases.storageTitle"),
        values: [
          pricing.billedInstances > 1
            ? t("databases.reviewStorageInstances", {
                size: storageGB,
                count: pricing.billedInstances,
              })
            : t("databases.storageSize", { size: storageGB }),
        ],
        price: price(pricing.storageCents),
        onEdit: onEditStorage,
        editLabel: t("databases.storageEditAria"),
      });
    }
    return { lines, total: price(pricing.totalCents) };
  };
}

/**
 * The High availability line's Edit, as on Render: it drops the change under
 * review and goes to the High Availability section below, whose switch takes
 * the focus the closing dialog hands back. Opening a review calls `clear`,
 * so any other close returns focus to its opener.
 */
function useEditHighAvailability() {
  const returnFocusRef = useRef<HTMLElement | null>(null);
  return {
    returnFocusRef,
    clear() {
      returnFocusRef.current = null;
    },
    edit(close: () => void) {
      const section = document.getElementById("high-availability");
      returnFocusRef.current =
        section?.querySelector<HTMLElement>('[role="switch"]') ?? null;
      close();
      section?.scrollIntoView({ behavior: "smooth" });
    },
  };
}
