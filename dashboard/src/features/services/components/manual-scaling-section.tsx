import { useState } from "react";
import { Loader2 } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { Skeleton } from "@/common/components/ui/skeleton";
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
  /** Render plan spelling — "free" caps the service at a single instance. */
  plan?: string | null;
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
  plan,
}: ManualScalingSectionProps) {
  const { t } = useTranslations();
  const { scaleService, busy } = useScaleService();
  // Render offers no free horizontal scaling: the free plan caps at a single
  // running instance, so the copy and the slider ceiling both narrow (w6/m118).
  const isFree = plan === "free";
  const maxInstances = isFree ? 1 : INSTANCE_MAX;
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
      <ManualScalingHeader isFree={isFree} />
      <CardContent>
        <div className="space-y-3">
          <p className="text-sm font-medium">
            {t("services.scalingManualInstances")}
          </p>
          <SliderInput
            id="manual-instances"
            label={t("services.scalingManualInstances")}
            value={value}
            min={INSTANCE_MIN}
            max={maxInstances}
            // Free caps at one instance, so there is nothing to choose: leave
            // the count visible but inert rather than accepting an edit the
            // plan will refuse.
            disabled={busy || isFree}
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

function ManualScalingHeader({ isFree }: { isFree: boolean }) {
  const { t } = useTranslations();
  return (
    <CardHeader>
      <CardTitle>{t("services.scalingManualTitle")}</CardTitle>
      <CardDescription className="mt-1">
        {isFree
          ? t("services.scalingManualDescriptionFree")
          : t("services.scalingManualDescription")}
      </CardDescription>
    </CardHeader>
  );
}

/**
 * The Manual Scaling card's frame while the autoscaling read resolves
 * (w4/688). The off state — this card showing — is the common outcome and the
 * only one on a free plan, so the slot is reserved at this card's geometry:
 * the same header and Instances label, the field row (the capped numeric input
 * alone on free, the slider row otherwise; both 32 px) and the Save button.
 */
export function ManualScalingSkeleton({ plan }: { plan?: string | null }) {
  const { t } = useTranslations();
  const isFree = plan === "free";
  return (
    <Card data-testid="manual-scaling-loading">
      <ManualScalingHeader isFree={isFree} />
      <CardContent>
        <div className="space-y-3">
          <p className="text-sm font-medium">
            {t("services.scalingManualInstances")}
          </p>
          <Skeleton className={isFree ? "h-8 w-16" : "h-8 w-full"} />
          <Skeleton className="h-8 w-28" />
        </div>
      </CardContent>
    </Card>
  );
}
