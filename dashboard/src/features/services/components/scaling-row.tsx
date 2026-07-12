import { useState } from "react";
import { Minus, Plus, Loader2 } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { useTranslations } from "@/common/hooks/use-translations";
import { useScaleService } from "@/features/services/hooks/use-scale-service";

const MIN_INSTANCES = 1;
const MAX_INSTANCES = 100;

const clamp = (n: number) =>
  Math.min(MAX_INSTANCES, Math.max(MIN_INSTANCES, n));

export interface ScalingRowProps {
  serviceId: string;
  /** Current spec.replicas; null while loading (renders skeleton-safe zero). */
  replicas: number | null;
}

/**
 * Settings row for Render-compatible manual instance-count scaling (w5/m16).
 * Renders a +/- stepper; Save fires scaleService when the draft differs from
 * the current replica count. Hidden for cron jobs by the settings page.
 */
export function ScalingRow({ serviceId, replicas }: ScalingRowProps) {
  const { t } = useTranslations();
  const { scaleService, busy } = useScaleService();
  const current = replicas ?? 1;
  const [draft, setDraft] = useState<number>(current);

  const dirty = draft !== current;

  async function handleSave() {
    const ok = await scaleService(serviceId, draft);
    if (!ok) setDraft(current);
  }

  return (
    <div className="flex items-center justify-between gap-4">
      <div>
        <div className="text-sm text-muted-foreground">
          {t("services.scalingInstanceCount")}
        </div>
        <div className="mt-1 text-sm text-muted-foreground">
          {t("services.scalingInstanceCountHint")}
        </div>
      </div>
      <div className="flex items-center gap-2">
        <Button
          size="icon"
          variant="outline"
          aria-label={t("services.scalingDecrement")}
          disabled={busy || draft <= MIN_INSTANCES}
          onClick={() => setDraft((n) => clamp(n - 1))}
        >
          <Minus />
        </Button>
        <Input
          type="number"
          min={MIN_INSTANCES}
          max={MAX_INSTANCES}
          value={draft}
          onChange={(e) => {
            const n = parseInt(e.target.value, 10);
            if (!isNaN(n)) setDraft(clamp(n));
          }}
          disabled={busy}
          className="w-16 text-center"
        />
        <Button
          size="icon"
          variant="outline"
          aria-label={t("services.scalingIncrement")}
          disabled={busy || draft >= MAX_INSTANCES}
          onClick={() => setDraft((n) => clamp(n + 1))}
        >
          <Plus />
        </Button>
        <Button
          size="sm"
          disabled={!dirty || busy}
          onClick={() => void handleSave()}
        >
          {busy ? (
            <Loader2 className="animate-spin" />
          ) : (
            t("services.scalingSaveCount")
          )}
        </Button>
      </div>
    </div>
  );
}
