import { useId } from "react";
import { AlertCircle } from "lucide-react";
import { cn } from "@/common/lib/utils/utils";
import { useTranslations } from "@/common/hooks/use-translations";
import type { DeployView } from "@/features/deploys/hooks/use-deploy";
import { useDeployTimeline } from "@/features/deploys/hooks/use-deploy-timeline";
import {
  buildDeployTimeline,
  type DeployTimelineStepKind,
} from "@/features/deploys/lib/deploy-timeline";
import { deployStatusKey } from "@/features/deploys/lib/deploy-status";
import { LocalDateTime } from "@/common/components/local-time";
import { DeployTimelineStepsSkeleton } from "./deploy-page-skeletons";
import {
  TIMELINE_DOT_CLASS,
  TIMELINE_LIST_CLASS,
  TIMELINE_SECTION_CLASS,
  TIMELINE_STEP_CLASS,
} from "./deploy-page-layout";

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
  const titleId = useId();
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
    <section aria-labelledby={titleId} className={TIMELINE_SECTION_CLASS}>
      <h3 id={titleId} className="text-sm leading-5 font-semibold">
        {t("deploys.timelineTitle")}
      </h3>
      {!deploy ? (
        <div role="status" aria-label={t("common.loading")}>
          <DeployTimelineStepsSkeleton />
        </div>
      ) : (
        <ol className={TIMELINE_LIST_CLASS}>
          {buildDeployTimeline(deploy, events).map((step) => (
            <li key={step.id} className={TIMELINE_STEP_CLASS}>
              <span
                aria-hidden
                className={cn(TIMELINE_DOT_CLASS, dotClass(step.kind))}
              />
              <p className="text-sm leading-5 font-medium">
                {t(
                  (step.status
                    ? deployStatusKey(step.status)
                    : STEP_KEYS[step.kind]) as Parameters<typeof t>[0],
                )}
              </p>
              {step.timestamp ? (
                <LocalDateTime
                  value={step.timestamp}
                  className="block text-xs leading-4 text-muted-foreground"
                />
              ) : null}
            </li>
          ))}
        </ol>
      )}
      {error ? (
        <p className="flex items-center gap-2 text-xs text-muted-foreground">
          <AlertCircle className="h-3.5 w-3.5" />
          {t("deploys.timelineEventsUnavailable")}
        </p>
      ) : null}
    </section>
  );
}

function dotClass(kind: DeployTimelineStepKind): string {
  switch (kind) {
    // One green for every step that happened: the Live step is the last of
    // them, not a second shade.
    case "live":
      return "bg-primary";
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
