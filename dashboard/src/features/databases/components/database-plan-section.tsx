import { useState } from "react";
import { Loader2 } from "lucide-react";
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
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { PlanCardGrid } from "@/common/components/plan-card-grid";
import {
  PlanTierSlots,
  POSTGRES_PLAN_SLOTS,
} from "@/common/components/datastore-plan-preview";
import { useTranslations } from "@/common/hooks/use-translations";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useDatabaseInstanceTypes } from "@/features/databases/hooks/use-database-instance-types";
import { useUpdateDatabasePlan } from "@/features/databases/hooks/use-update-database-plan";
import { useUpdateDatabaseDiskSize } from "@/features/databases/hooks/use-update-database-disk-size";
import {
  STORAGE_STEP_GB,
  databaseStorageGB,
  planStorageFixed,
  storageGrowProblem,
} from "@/features/databases/lib/storage";
import { highAvailabilityRequested } from "@/features/databases/lib/status";
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
  const capabilities = useCapabilities();
  const { canOperate } = capabilities;
  const operateDenied = !canOperate;
  const operateReasonKey = capabilities.reasonKey("can_operate");
  const operateReason = operateReasonKey ? t(operateReasonKey) : undefined;
  const { instanceTypes, loading, retry, retrying } =
    useDatabaseInstanceTypes();
  const { updatePlan, busy } = useUpdateDatabasePlan();
  const [selected, setSelected] = useState<string | null>(database.plan);
  const [confirming, setConfirming] = useState(false);

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
          <PlanTierSlots count={POSTGRES_PLAN_SLOTS} />
        ) : instanceTypes.length === 0 ? (
          // A settled failed or empty catalog, not an empty choice (w4/m218).
          <PlanCatalogUnavailable
            onRetry={() => void retry()}
            retrying={retrying}
          />
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

        <StorageRow
          database={database}
          plan={instanceTypes.find((it) => it.id === database.plan)}
          operateReason={operateReason}
          onChanged={onChanged}
        />
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

/**
 * Render's Plan page Storage section (w2/m307): the allocated size and an Edit
 * that grows it — any multiple of 5 GB above the current size, up to the
 * plan's maximum. Storage never shrinks, and a free plan's is fixed, as
 * bex-api's admission rules (and Render) have it.
 */
function StorageRow({
  database,
  plan,
  operateReason,
  onChanged,
}: {
  database: DatabaseDetailView;
  /** The database's current plan, or undefined while the catalog loads. */
  plan: DatabaseInstanceTypeView | undefined;
  operateReason: string | undefined;
  onChanged: () => void;
}) {
  const { t } = useTranslations();
  const { updateDiskSize, busy } = useUpdateDatabaseDiskSize();
  const [editing, setEditing] = useState(false);
  const [value, setValue] = useState("");
  const [serverError, setServerError] = useState<string | null>(null);

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
  const error = fieldError ?? serverError;
  // The smallest size it accepts, which also anchors the field's step.
  const next = (Math.floor(current / STORAGE_STEP_GB) + 1) * STORAGE_STEP_GB;

  function startEditing() {
    setValue(max == null || next <= max ? String(next) : "");
    setServerError(null);
    setEditing(true);
  }

  async function save() {
    if (!canSave) return;
    const message = await updateDiskSize(database.id, Number(value));
    if (message) {
      setServerError(message);
      return;
    }
    setEditing(false);
    onChanged();
  }

  return (
    <div className="flex items-center justify-between gap-4 border-t pt-4">
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
      <PermissionTooltip reason={operateReason}>
        <Button
          variant="outline"
          size="sm"
          aria-label={t("databases.storageEditAria")}
          // Until the catalog names the plan, a free database's fixed disk
          // is unknown, so the edit waits for it.
          disabled={operateReason != null || plan == null || fixed || busy}
          onClick={startEditing}
        >
          {t("databases.storageEdit")}
        </Button>
      </PermissionTooltip>

      <Dialog
        open={editing}
        onOpenChange={(open) => {
          if (!busy) setEditing(open);
        }}
      >
        <DialogContent>
          {/* noValidate: the field's own messages explain a refusal, in the
              user's language, instead of the browser's constraint bubble. */}
          <form
            noValidate
            onSubmit={(e) => {
              e.preventDefault();
              void save();
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
                onChange={(e) => {
                  setValue(e.target.value);
                  setServerError(null);
                }}
                aria-invalid={error != null}
                aria-describedby={
                  error ? "database-storage-size-error" : undefined
                }
              />
              {error ? (
                <p
                  id="database-storage-size-error"
                  className="text-destructive text-sm"
                  role="alert"
                >
                  {error}
                </p>
              ) : null}
            </div>
            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                disabled={busy}
                onClick={() => setEditing(false)}
              >
                {t("common.cancel")}
              </Button>
              <Button
                type="submit"
                disabled={!canSave || busy}
              >
                {busy ? <Loader2 className="animate-spin" /> : null}
                {t("databases.storageEditSave")}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  );
}
