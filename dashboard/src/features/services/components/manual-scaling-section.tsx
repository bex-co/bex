import { useState } from "react";
import { Loader2 } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@/common/components/ui/card";
import { useTranslations } from "@/common/hooks/use-translations";
import { useScaleService } from "@/features/services/hooks/use-scale-service";
import {
  INSTANCE_MAX,
  INSTANCE_MIN,
  SliderInput,
} from "@/features/services/components/autoscaling-section";

export interface ManualScalingSectionProps {
  serviceId: string;
  /** Current spec.replicas — the parent mounts this card only once loaded. */
  replicas: number;
}

/**
 * Render's "Manual Scaling" card on the Scaling page (w7/m43): a fixed
 * instance count via slider + numeric input, saved through the same
 * `scaleService` verb REST/MCP use. The Scaling route mounts it only while
 * autoscaling is off (Render's mutual exclusion) and only for service types
 * with a replica concept — supersedes the w5/m16 Settings stepper, whose
 * placement followed a docs-fallback guess the live capture corrected
 * (docs/render-artifacts/manual-scaling.md).
 */
export function ManualScalingSection({
  serviceId,
  replicas,
}: ManualScalingSectionProps) {
  const { t } = useTranslations();
  const { scaleService, busy } = useScaleService();
  // null = no local edit; fall through to the live count (the same null-draft
  // idiom as the autoscaling card), so an external replica change — another
  // tab, REST, the autoscaler — shows through while the form is pristine.
  const [draft, setDraft] = useState<number | null>(null);

  const value = draft ?? replicas;
  const dirty = draft != null && draft !== replicas;

  async function handleSave() {
    if (draft == null) return;
    const accepted = await scaleService(serviceId, draft);
    // Mutation success means the desired count was accepted; Apollo updates the
    // cached spec.replicas selection, while operator/Deployment convergence is
    // still asynchronous. Preserve a rejected draft so the user can retry.
    if (accepted) setDraft(null);
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("services.scalingManualTitle")}</CardTitle>
        <CardDescription className="mt-1">
          {t("services.scalingManualDescription")}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className="space-y-3">
          <p className="text-sm font-medium">
            {t("services.scalingManualInstances")}
          </p>
          <SliderInput
            id="manual-instances"
            value={value}
            min={INSTANCE_MIN}
            max={INSTANCE_MAX}
            disabled={busy}
            onChange={setDraft}
          />
          <Button
            size="sm"
            disabled={!dirty || busy}
            onClick={() => void handleSave()}
          >
            {busy ? (
              <Loader2 className="animate-spin" />
            ) : (
              t("services.scalingSaveChanges")
            )}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
