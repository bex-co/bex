import { useState, useMemo } from "react";
import { Switch } from "@/common/components/ui/switch";
import { Label } from "@/common/components/ui/label";
import { Input } from "@/common/components/ui/input";
import { Button } from "@/common/components/ui/button";
import { Skeleton } from "@/common/components/ui/skeleton";
import { Slider } from "@/common/components/ui/slider";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/common/components/ui/card";
import { useTranslations } from "@/common/hooks/use-translations";
import { useAutoscaling } from "@/features/services/hooks/use-autoscaling";

// Render uses 1–25 for instance count and 1–90 for utilisation targets.
const INSTANCE_MIN = 1;
const INSTANCE_MAX = 25;
const UTIL_MIN = 1;
const UTIL_MAX = 90;

interface FormState {
  minInstances: number;
  maxInstances: number;
  cpuEnabled: boolean;
  targetCPUPercent: number;
  memEnabled: boolean;
  targetMemoryPercent: number;
}

interface Draft {
  enabled: boolean;
  form: FormState;
}

function clamp(v: number, lo: number, hi: number) {
  return Math.max(lo, Math.min(hi, v));
}

function validate(form: FormState): string | null {
  if (form.minInstances > form.maxInstances)
    return "Min instances must be ≤ max instances.";
  if (!form.cpuEnabled && !form.memEnabled)
    return "At least one utilisation target (CPU or memory) must be enabled.";
  return null;
}

// A slider + numeric input pair sharing the same value.
function SliderInput({
  id,
  value,
  min,
  max,
  disabled,
  onChange,
}: {
  id: string;
  value: number;
  min: number;
  max: number;
  disabled: boolean;
  onChange: (v: number) => void;
}) {
  return (
    <div className="flex items-center gap-3">
      <span className="w-6 shrink-0 text-right text-xs text-muted-foreground">
        {min}
      </span>
      <Slider
        min={min}
        max={max}
        step={1}
        value={[value]}
        disabled={disabled}
        onValueChange={([v]) => onChange(clamp(v, min, max))}
        className="flex-1"
      />
      <span className="w-6 shrink-0 text-xs text-muted-foreground">{max}</span>
      <Input
        id={id}
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
        aria-label="Minimum instances"
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
        aria-label="Maximum instances"
      />
    </div>
  );
}

export function AutoscalingSection({ serviceId }: { serviceId: string }) {
  const { t } = useTranslations();
  const as = useAutoscaling(serviceId);

  // null = no local edits; fall through to server state.
  const [draft, setDraft] = useState<Draft | null>(null);
  const [validationError, setValidationError] = useState<string | null>(null);

  const serverState = useMemo<Draft>(
    () => ({
      enabled: as.enabled,
      form: {
        minInstances: clamp(as.minInstances, INSTANCE_MIN, INSTANCE_MAX),
        maxInstances: clamp(as.maxInstances, INSTANCE_MIN, INSTANCE_MAX),
        cpuEnabled: as.targetCPUPercent != null,
        targetCPUPercent:
          as.targetCPUPercent != null
            ? clamp(as.targetCPUPercent, UTIL_MIN, UTIL_MAX)
            : 75,
        memEnabled: as.targetMemoryPercent != null,
        targetMemoryPercent:
          as.targetMemoryPercent != null
            ? clamp(as.targetMemoryPercent, UTIL_MIN, UTIL_MAX)
            : 90,
      },
    }),
    [
      as.enabled,
      as.minInstances,
      as.maxInstances,
      as.targetCPUPercent,
      as.targetMemoryPercent,
    ],
  );

  const { enabled, form } = draft ?? serverState;

  function patch(partial: Partial<FormState>) {
    setDraft({ enabled, form: { ...form, ...partial } });
    setValidationError(null);
  }

  async function handleMainToggle(checked: boolean) {
    setDraft({ enabled: checked, form });
    if (!checked) {
      const ok = await as.disable();
      if (ok) setDraft(null);
    }
  }

  async function handleSave() {
    const err = validate(form);
    if (err) {
      setValidationError(err);
      return;
    }
    const ok = await as.save({
      minInstances: form.minInstances,
      maxInstances: form.maxInstances,
      targetCPUPercent: form.cpuEnabled ? form.targetCPUPercent : null,
      targetMemoryPercent: form.memEnabled ? form.targetMemoryPercent : null,
    });
    if (ok) setDraft(null);
  }

  function handleCancel() {
    setDraft(null);
    setValidationError(null);
  }

  if (as.loading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("services.scalingTitle")}</CardTitle>
          <CardDescription>{t("services.scalingDescription")}</CardDescription>
        </CardHeader>
        <CardContent>
          <Skeleton className="h-20 w-full" />
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div>
            <CardTitle>{t("services.scalingTitle")}</CardTitle>
            <CardDescription className="mt-1">
              {t("services.scalingDescription")}
            </CardDescription>
          </div>
          <div className="flex items-center gap-2 shrink-0">
            <Switch
              id="autoscaling-enabled"
              checked={enabled}
              disabled={as.saving}
              onCheckedChange={handleMainToggle}
            />
            <Label htmlFor="autoscaling-enabled" className="text-sm">
              {enabled ? t("services.scalingOn") : t("services.scalingOff")}
            </Label>
          </div>
        </div>
      </CardHeader>

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
                disabled={as.saving}
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
                  disabled={as.saving}
                  onCheckedChange={(v) => patch({ cpuEnabled: v })}
                />
              </div>
              {form.cpuEnabled && (
                <div className="flex items-center gap-2">
                  <SliderInput
                    id="target-cpu"
                    value={form.targetCPUPercent}
                    min={UTIL_MIN}
                    max={UTIL_MAX}
                    disabled={as.saving}
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
                  disabled={as.saving}
                  onCheckedChange={(v) => patch({ memEnabled: v })}
                />
              </div>
              {form.memEnabled && (
                <div className="flex items-center gap-2">
                  <SliderInput
                    id="target-mem"
                    value={form.targetMemoryPercent}
                    min={UTIL_MIN}
                    max={UTIL_MAX}
                    disabled={as.saving}
                    onChange={(v) => patch({ targetMemoryPercent: v })}
                  />
                  <span className="text-sm text-muted-foreground">%</span>
                </div>
              )}
            </div>

            {validationError && (
              <p className="text-sm text-destructive">{validationError}</p>
            )}

            <div className="flex gap-2">
              <Button
                variant="outline"
                size="sm"
                disabled={as.saving}
                onClick={handleCancel}
              >
                {t("services.scalingCancel")}
              </Button>
              <Button size="sm" disabled={as.saving} onClick={handleSave}>
                {t("services.scalingSaveChanges")}
              </Button>
            </div>
          </div>
        </CardContent>
      )}
    </Card>
  );
}
