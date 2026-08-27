import { AlertCircle } from "lucide-react";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import type { DeployView } from "@/features/deploys/hooks/use-deploy";
import { useDeployTimeline } from "@/features/deploys/hooks/use-deploy-timeline";
import {
  buildDeployTimeline,
  type DeployTimelineStepKind,
} from "@/features/deploys/lib/deploy-timeline";
import { deployStatusKey } from "@/features/deploys/lib/deploy-status";
import { LocalDateTime } from "@/common/components/local-time";

const STEP_KEYS: Record<DeployTimelineStepKind, string> = {
  created: "deploys.timelineCreated",
  started: "deploys.timelineStarted",
  in_progress: "deploys.timelineInProgress",
  live: "deploys.timelineLive",
  failed: "deploys.timelineFailed",
  canceled: "deploys.timelineCanceled",
  deactivated: "deploys.timelineDeactivated",
};

export interface DeployTimelineProps {
  serviceId: string;
  /** Nullish while the header `deploy` query is still in flight (w9/m62 t002). */
  deploy?: DeployView | null;
}

/** A deploy-row + matching-service-events timeline with no inferred phases. */
export function DeployTimeline({ serviceId, deploy }: DeployTimelineProps) {
  const { t } = useTranslations();
  // Mounted in parallel with the header query; its own query stays skipped
  // until the deploy's window arrives (w9/m62 t002), then fires — no serialized
  // extra mount behind the header round trip.
  const { events, error } = useDeployTimeline(
    serviceId,
    deploy?.id ?? "",
    deploy?.createdAt ?? undefined,
    deploy?.finishedAt ?? undefined,
    !deploy,
  );

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("deploys.timelineTitle")}</CardTitle>
      </CardHeader>
      <CardContent>
        {!deploy ? (
          <div className="space-y-3" role="status" aria-label={t("common.loading")}>
            <Skeleton className="h-4 w-40" />
            <Skeleton className="h-4 w-28" />
            <Skeleton className="h-4 w-36" />
          </div>
        ) : (
          <ol className="ml-2 border-l">
          {buildDeployTimeline(deploy, events).map((step) => (
            <li key={step.id} className="relative pb-5 pl-6 last:pb-0">
              <span
                className={`absolute -left-1.5 top-1 h-3 w-3 rounded-full border-2 border-background ${dotClass(step.kind)}`}
              />
              <p className="text-sm font-medium">
                {t(
                  (step.status
                    ? deployStatusKey(step.status)
                    : STEP_KEYS[step.kind]) as Parameters<typeof t>[0],
                )}
              </p>
              {step.timestamp ? (
                <LocalDateTime
                  value={step.timestamp}
                  className="text-xs text-muted-foreground"
                />
              ) : null}
            </li>
          ))}
        </ol>
        )}
        {error ? (
          <p className="mt-4 flex items-center gap-2 text-xs text-muted-foreground">
            <AlertCircle className="h-3.5 w-3.5" />
            {t("deploys.timelineEventsUnavailable")}
          </p>
        ) : null}
      </CardContent>
    </Card>
  );
}

function dotClass(kind: DeployTimelineStepKind): string {
  switch (kind) {
    case "live":
      return "bg-emerald-500";
    case "failed":
      return "bg-destructive";
    case "canceled":
    case "deactivated":
      return "bg-muted-foreground";
    case "in_progress":
      return "bg-blue-500";
    default:
      return "bg-primary";
  }
}
