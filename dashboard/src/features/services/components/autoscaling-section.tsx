import { useState, useMemo } from "react";
import { TriangleAlert } from "lucide-react";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert";
import { Switch } from "@/common/components/ui/switch";
import { Label } from "@/common/components/ui/label";
import { Input } from "@/common/components/ui/input";
import { Button } from "@/common/components/ui/button";
import { Slider } from "@/common/components/ui/slider";
import { ConfirmDialog } from "@/common/components/confirm-dialog";
import { PlanBillingLink } from "@/common/components/plan-billing-link";
import {
  AutoscalingHeader,
  AutoscalingLoadingCard,
  AutoscalingLockedNotice,
} from "@/features/services/components/scaling-frames";
import { autoscalingLock } from "@/features/services/lib/autoscaling-lock";
import { Card, CardContent } from "@/common/components/ui/card";
import { useTranslations } from "@/common/hooks/use-translations";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useCapabilityGate } from "@/features/capabilities/hooks/use-capability-gate";
import type {
  AutoscalingFormValues,
  UseAutoscalingResult,
} from "@/features/services/hooks/use-autoscaling";

// Render uses 1–100 for instance count and 1–90 for utilization targets, with
// 60% as the first-enable default target (live capture 2026-07-16, w7/m43).
// The backend validates the same 1–100 (store.MaxReplicas). The bounds are
// exported for the Manual Scaling card, which shares them.
export const INSTANCE_MIN = 1;
export const INSTANCE_MAX = 100;
const UTIL_MIN = 1;
const UTIL_MAX = 90;
const DEFAULT_TARGET_PERCENT = 60;

interface FormState {
  minInstances: number;
  maxInstances: number;
  cpuEnabled: boolean;
  targetCPUPercent: number;
  memEnabled: boolean;
  targetMemoryPercent: number;
}

// Render's disable dialog names why autoscaling goes off: the switch, or the
// last utilization target turned off (its dashboard bundle, w4/m696).
type DisableBodyKey =
  | "services.scalingDisableConfirmBody"
  | "services.scalingDisableCPUConfirmBody"
  | "services.scalingDisableMemoryConfirmBody";

function clamp(v: number, lo: number, hi: number) {
  return Math.max(lo, Math.min(hi, v));
}

function formValues(form: FormState): AutoscalingFormValues {
  return {
    minInstances: form.minInstances,
    maxInstances: form.maxInstances,
    targetCPUPercent: form.cpuEnabled ? form.targetCPUPercent : null,
    targetMemoryPercent: form.memEnabled ? form.targetMemoryPercent : null,
  };
}

// A slider + numeric input pair sharing the same value. Exported for the
// Manual Scaling card, which uses the same control for its instance count
// (w7/m43).
export function SliderInput({
  id,
  label,
  value,
  min,
  max,
  disabled,
  onChange,
}: {
  id: string;
  label: string;
  value: number;
  min: number;
  max: number;
  disabled: boolean;
  onChange: (v: number) => void;
}) {
  // A single-point range (min === max — the free plan's 1-instance cap) has no
  // span to drag, and the track fill computes (value - min) / (max - min):
  // 0/0 -> NaN -> `calc(NaN% + 0px)`. Browsers drop that declaration silently;
  // jsdom's CSS parser throws outright. Either way a one-position slider is
  // not a control, so render the capped numeric input alone.
  const fixed = min >= max;
  return (
    <div className="flex items-center gap-3">
      {!fixed && (
        <>
          <span className="w-6 shrink-0 text-right text-xs text-muted-foreground">
            {min}
          </span>
          <Slider
            thumbLabels={[label]}
            min={min}
            max={max}
            step={1}
            value={[value]}
            disabled={disabled}
            onValueChange={([v]) => onChange(clamp(v, min, max))}
            className="flex-1"
          />
          <span className="w-6 shrink-0 text-xs text-muted-foreground">
            {max}
          </span>
        </>
      )}
      <Input
        id={id}
        aria-label={label}
        type="number"
        min={min}
        max={max}
        value={value}
        disabled={disabled}
        onChange={(e) => {
          const n = parseInt(e.target.value, 10);
          if (!isNaN(n)) onChange(clamp(n, min, max));
        }}
        className="h-8 w-16 text-center"
      />
    </div>
  );
}

// A range slider (two thumbs) + two numeric inputs — matches Render's
// "Number of Instances" control (min on left, max on right).
function RangeSliderInput({
  min,
  max,
  valueMin,
  valueMax,
  absMin,
  absMax,
  disabled,
  onChangeMin,
  onChangeMax,
}: {
  min: number;
  max: number;
  valueMin: number;
  valueMax: number;
  absMin: number;
  absMax: number;
  disabled: boolean;
  onChangeMin: (v: number) => void;
  onChangeMax: (v: number) => void;
}) {
  const { t } = useTranslations();
  return (
    <div className="flex items-center gap-3">
      <Input
        type="number"
        min={absMin}
        max={valueMax}
        value={valueMin}
        disabled={disabled}
        onChange={(e) => {
          const n = parseInt(e.target.value, 10);
          if (!isNaN(n)) onChangeMin(clamp(n, absMin, valueMax));
        }}
        className="h-8 w-16 text-center"
        aria-label={t("services.autoscaleMinInstances")}
      />
      <Slider
        min={min}
        max={max}
        step={1}
        value={[valueMin, valueMax]}
        disabled={disabled}
        onValueChange={(vals) => {
          onChangeMin(vals[0]);
          onChangeMax(vals[1]);
        }}
        className="flex-1"
      />
      <Input
        type="number"
        min={valueMin}
        max={absMax}
        value={valueMax}
        disabled={disabled}
        onChange={(e) => {
          const n = parseInt(e.target.value, 10);
          if (!isNaN(n)) onChangeMax(clamp(n, valueMin, absMax));
        }}
        className="h-8 w-16 text-center"
        aria-label={t("services.autoscaleMaxInstances")}
      />
    </div>
  );
}

// Takes the hook result rather than calling useAutoscaling itself: the route
// already holds one instance for the manual-card exclusion gate, and a second
// instance would carry its own (divergent) `saving` state (w7/m43 simplify).
export function AutoscalingSection({
  autoscaling: as,
  instances = 1,
  plan,
  workspacePlan,
}: {
  autoscaling: UseAutoscalingResult;
  /** The service's instance count: a first enable scales from it. */
  instances?: number;
  /** Render offers autoscaling on paid plans only; free caps at one instance. */
  plan?: string | null;
  /**
   * The workspace plan: Render offers autoscaling on Pro workspaces and
   * higher (w7/m429). Unknown (still loading) reads as included; the API
   * enforces the rule either way.
   */
  workspacePlan?: string | null;
}) {
  const { t } = useTranslations();
  const { denied: createDenied, reason: createReason } =
    useCapabilityGate("canCreate");
  // Every control of the card is held while a save runs or the role denies.
  const inert = as.saving || createDenied;

  // null = no local edits; fall through to server state. The form shows only
  // while autoscaling is on, so a draft only ever edits a running policy.
  const [draft, setDraft] = useState<FormState | null>(null);
  const [minMaxError, setMinMaxError] = useState(false);
  // As on Render, the switch saves nothing by itself: turning autoscaling on
  // confirms the billing first, and turning it off — from the switch or by
  // turning off the last utilization target — confirms the revert to the
  // fixed Manual Scaling count (its dashboard bundle, w4/m696; w7/m43).
  const [confirmEnable, setConfirmEnable] = useState(false);
  const [confirmDisable, setConfirmDisable] = useState(false);
  // Kept apart from the open flag so the body survives the close animation.
  const [disableBody, setDisableBody] = useState<DisableBodyKey>(
    "services.scalingDisableConfirmBody",
  );

  const serverForm = useMemo<FormState>(
    () => ({
      minInstances: clamp(as.minInstances, INSTANCE_MIN, INSTANCE_MAX),
      maxInstances: clamp(as.maxInstances, INSTANCE_MIN, INSTANCE_MAX),
      cpuEnabled: as.targetCPUPercent != null,
      targetCPUPercent:
        as.targetCPUPercent != null
          ? clamp(as.targetCPUPercent, UTIL_MIN, UTIL_MAX)
          : DEFAULT_TARGET_PERCENT,
      memEnabled: as.targetMemoryPercent != null,
      targetMemoryPercent:
        as.targetMemoryPercent != null
          ? clamp(as.targetMemoryPercent, UTIL_MIN, UTIL_MAX)
          : DEFAULT_TARGET_PERCENT,
    }),
    [
      as.minInstances,
      as.maxInstances,
      as.targetCPUPercent,
      as.targetMemoryPercent,
    ],
  );

  // Autoscaling is paid-only: setAutoscaling refuses a free service
  // server-side at every range (w4/365), so offering the editor only to fail
  // the save is a promise the plan cannot keep (w7/048). Gated the way
  // maintenance-mode-section.tsx gates its toggle. A free service can still
  // carry a stored config (written before the gate, or left by a downgrade):
  // it reads as saved-but-inactive below with a working turn-off, never as
  // a plain Off that hides settings nobody can remove.
  //
  // A Hobby workspace cannot turn autoscaling on (w7/m429; Render: Pro and
  // higher), so the switch is held off with the plan notice and a Billing
  // link. A policy it already runs keeps running: it reads as on, with its
  // settings, and the switch can only turn it off.
  const { eligible, planGated, locked } = autoscalingLock(plan, workspacePlan);
  const storedLocked = locked && as.enabled;
  const enabled = !locked && as.enabled;
  const form = draft ?? serverForm;
  const dirty =
    draft != null &&
    (Object.keys(draft) as (keyof FormState)[]).some(
      (k) => draft[k] !== serverForm[k],
    );

  function patch(partial: Partial<FormState>) {
    setDraft({ ...form, ...partial });
    setMinMaxError(false);
  }

  function askDisable(body: DisableBodyKey) {
    setDisableBody(body);
    setConfirmDisable(true);
  }

  // The switch reads as.enabled, and is disabled while locked and off: on
  // free, or below Pro, it can only turn a stored config off.
  function handleMainToggle(checked: boolean) {
    if (createDenied) return;
    if (!checked) askDisable("services.scalingDisableConfirmBody");
    else if (!locked) setConfirmEnable(true);
  }

  // Turning off the last utilization target turns autoscaling off, as
  // Render's docs say ("If you enable neither toggle, autoscaling is
  // disabled"), so it asks as the switch does instead of leaving a form no
  // save can take.
  function handleTargetToggle(target: "cpu" | "memory", on: boolean) {
    const otherOn = target === "cpu" ? form.memEnabled : form.cpuEnabled;
    if (!on && !otherOn) {
      askDisable(
        target === "cpu"
          ? "services.scalingDisableCPUConfirmBody"
          : "services.scalingDisableMemoryConfirmBody",
      );
      return;
    }
    patch(target === "cpu" ? { cpuEnabled: on } : { memEnabled: on });
  }

  // An enable starts from the settings a disable kept (w4/597) or, never
  // configured (no stored target), from Render's first-enable defaults: the
  // current instance count up to twice it, CPU and memory both at 60%.
  function enableValues(): AutoscalingFormValues {
    if (as.targetCPUPercent != null || as.targetMemoryPercent != null)
      return formValues(serverForm);
    const min = clamp(instances, INSTANCE_MIN, INSTANCE_MAX);
    return {
      minInstances: min,
      maxInstances: Math.min(min * 2, INSTANCE_MAX),
      targetCPUPercent: DEFAULT_TARGET_PERCENT,
      targetMemoryPercent: DEFAULT_TARGET_PERCENT,
    };
  }

  async function handleEnableConfirmed() {
    setConfirmEnable(false);
    await as.save(enableValues());
  }

  async function handleDisableConfirmed() {
    setConfirmDisable(false);
    const ok = await as.disable();
    if (ok) handleCancel();
  }

  async function handleSave() {
    if (createDenied) return;
    if (form.minInstances > form.maxInstances) {
      setMinMaxError(true);
      return;
    }
    const ok = await as.save(formValues(form));
    if (ok) setDraft(null);
  }

  function handleCancel() {
    setDraft(null);
    setMinMaxError(false);
  }

  // A stored policy the switch can only turn off: its settings, read-only.
  const storedSummary = (
    <p className="text-sm">
      {t("services.scalingInstancesTitle")}: {as.minInstances}–{as.maxInstances}
      {as.targetCPUPercent != null &&
        ` · ${t("services.scalingCPUTitle")}: ${as.targetCPUPercent}%`}
      {as.targetMemoryPercent != null &&
        ` · ${t("services.scalingMemoryTitle")}: ${as.targetMemoryPercent}%`}
    </p>
  );

  // The header and the plan notice are the loading frame's own
  // (scaling-frames.tsx), so the two cannot drift apart (w4/688): only the
  // switch slot differs.
  if (as.loading) {
    return <AutoscalingLoadingCard plan={plan} workspacePlan={workspacePlan} />;
  }

  return (
    <Card>
      <AutoscalingHeader
        control={
          <>
            <PermissionTooltip reason={createReason}>
              <Switch
                id="autoscaling-enabled"
                checked={as.enabled}
                disabled={inert || (locked && !as.enabled)}
                onCheckedChange={handleMainToggle}
              />
            </PermissionTooltip>
            <Label htmlFor="autoscaling-enabled" className="text-sm">
              {as.enabled ? t("services.scalingOn") : t("services.scalingOff")}
            </Label>
          </>
        }
      />

      {locked && (
        <CardContent>
          {storedLocked ? (
            <div className="space-y-2">
              <p className="text-sm text-muted-foreground">
                {planGated ? (
                  <>
                    {t("services.scalingProPlanOnlyRunning")}{" "}
                    <PlanBillingLink />
                  </>
                ) : (
                  t("services.scalingPaidOnlyInactive")
                )}
              </p>
              {storedSummary}
            </div>
          ) : (
            <AutoscalingLockedNotice planGated={planGated} />
          )}
        </CardContent>
      )}

      {/* The operator holds the count while it cannot read utilization
          (w4/m364); without this the card read as active autoscaling. */}
      {eligible && as.enabled && as.metricsUnavailable && (
        <CardContent>
          <Alert>
            <TriangleAlert />
            <AlertTitle>
              {t("services.scalingMetricsUnavailableTitle")}
            </AlertTitle>
            <AlertDescription>
              {t("services.scalingMetricsUnavailableBody")}
            </AlertDescription>
          </Alert>
        </CardContent>
      )}

      {enabled && (
        <CardContent>
          <div className="space-y-8">
            {/* Number of Instances */}
            <div className="space-y-3">
              <div>
                <p className="text-sm font-medium">
                  {t("services.scalingInstancesTitle")}
                </p>
                <p className="text-xs text-muted-foreground">
                  {t("services.scalingInstancesHint")}
                </p>
              </div>
              <RangeSliderInput
                min={INSTANCE_MIN}
                max={INSTANCE_MAX}
                absMin={INSTANCE_MIN}
                absMax={INSTANCE_MAX}
                valueMin={form.minInstances}
                valueMax={form.maxInstances}
                disabled={inert}
                onChangeMin={(v) =>
                  patch({
                    minInstances: v,
                    maxInstances: Math.max(v, form.maxInstances),
                  })
                }
                onChangeMax={(v) =>
                  patch({
                    maxInstances: v,
                    minInstances: Math.min(v, form.minInstances),
                  })
                }
              />
            </div>

            {/* Target CPU Utilization */}
            <div className="space-y-3">
              <div className="flex items-start justify-between gap-4">
                <div>
                  <p className="text-sm font-medium">
                    {t("services.scalingCPUTitle")}
                  </p>
                  <p className="text-xs text-muted-foreground">
                    {t("services.scalingCPUHint")}
                  </p>
                </div>
                <Switch
                  id="cpu-enabled"
                  checked={form.cpuEnabled}
                  disabled={inert}
                  onCheckedChange={(v) => handleTargetToggle("cpu", v)}
                  aria-label={t("services.scalingCPUTitle")}
                />
              </div>
              {form.cpuEnabled && (
                <div className="flex items-center gap-2">
                  <SliderInput
                    id="target-cpu"
                    label={t("services.scalingCPUTitle")}
                    value={form.targetCPUPercent}
                    min={UTIL_MIN}
                    max={UTIL_MAX}
                    disabled={inert}
                    onChange={(v) => patch({ targetCPUPercent: v })}
                  />
                  <span className="text-sm text-muted-foreground">%</span>
                </div>
              )}
            </div>

            {/* Target Memory Utilization */}
            <div className="space-y-3">
              <div className="flex items-start justify-between gap-4">
                <div>
                  <p className="text-sm font-medium">
                    {t("services.scalingMemoryTitle")}
                  </p>
                  <p className="text-xs text-muted-foreground">
                    {t("services.scalingMemoryHint")}
                  </p>
                </div>
                <Switch
                  id="mem-enabled"
                  checked={form.memEnabled}
                  disabled={inert}
                  onCheckedChange={(v) => handleTargetToggle("memory", v)}
                  aria-label={t("services.scalingMemoryTitle")}
                />
              </div>
              {form.memEnabled && (
                <div className="flex items-center gap-2">
                  <SliderInput
                    id="target-mem"
                    label={t("services.scalingMemoryTitle")}
                    value={form.targetMemoryPercent}
                    min={UTIL_MIN}
                    max={UTIL_MAX}
                    disabled={inert}
                    onChange={(v) => patch({ targetMemoryPercent: v })}
                  />
                  <span className="text-sm text-muted-foreground">%</span>
                </div>
              )}
            </div>

            {minMaxError ? (
              <p className="text-sm text-destructive">
                {t("services.scalingValidationMinMax")}
              </p>
            ) : null}

            <div className="flex gap-2">
              {/* Render's Cancel shows once a value changes, and Save
                  Changes waits for one. */}
              {dirty && (
                <Button
                  variant="outline"
                  size="sm"
                  disabled={as.saving}
                  onClick={handleCancel}
                >
                  {t("services.scalingCancel")}
                </Button>
              )}
              <PermissionTooltip reason={createReason}>
                <Button
                  size="sm"
                  disabled={inert || !dirty}
                  onClick={handleSave}
                >
                  {t("services.scalingSaveChanges")}
                </Button>
              </PermissionTooltip>
            </div>
          </div>
        </CardContent>
      )}

      {/* Render styles both confirmations primary, not destructive. */}
      <ConfirmDialog
        open={confirmEnable}
        onOpenChange={setConfirmEnable}
        title={t("services.scalingEnableConfirmTitle")}
        description={t("services.scalingEnableConfirmBody")}
        cancelLabel={t("services.scalingCancel")}
        confirmLabel={t("services.scalingEnableConfirmAction")}
        destructive={false}
        onConfirm={() => void handleEnableConfirmed()}
      />
      <ConfirmDialog
        open={confirmDisable}
        onOpenChange={setConfirmDisable}
        title={t("services.scalingDisableConfirmTitle")}
        description={t(disableBody)}
        cancelLabel={t("services.scalingCancel")}
        confirmLabel={t("services.scalingDisableConfirmAction")}
        destructive={false}
        onConfirm={() => void handleDisableConfirmed()}
      />
    </Card>
  );
}
