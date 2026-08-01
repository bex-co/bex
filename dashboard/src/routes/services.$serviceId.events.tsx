import { useState } from "react";
import { createFileRoute, Link } from "@tanstack/react-router";
import {
  Activity,
  AlertCircle,
  Ban,
  CheckCircle2,
  CircleDot,
  GitBranch,
  Hammer,
  ListFilter,
  PauseCircle,
  PlayCircle,
  RefreshCcw,
  Rocket,
  Scale,
  Terminal,
  XCircle,
} from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
  CardContent,
} from "@/common/components/ui/card";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useServer } from "@/features/services/hooks/use-server";
import { CronRunsSection } from "@/features/services/components/cron-runs-section";
import { isCron } from "@/features/services/lib/service-type";
import { useServiceBase } from "@/features/services/lib/service-base";
import { formatRelativeAge } from "@/features/services/lib/format";
import { formatDateTime } from "@/common/lib/format";
import {
  deployStatusVariant as statusVariant,
  deployStatusKey as statusKey,
  preDeployStatusKey as preDeployKey,
  isCancelableDeployStatus,
} from "@/features/deploys/lib/deploy-status";
import { DeployActions } from "@/features/deploys/components/deploy-actions";
import { useServiceEvents } from "@/features/events/hooks/use-service-events";
import { ServiceEventFilter } from "@/features/events/components/service-event-filter";
import {
  SERVICE_EVENT_TYPES,
  serviceEventLabelKey,
} from "@/features/events/service-event-catalog";

export const Route = createFileRoute("/services/$serviceId/events")({
  component: RouteComponent,
});

function RouteComponent() {
  const { serviceId } = Route.useParams();
  return <ServiceEventsPage serviceId={serviceId} />;
}

// ServiceEventDetails.trigger is the Trigger object (boolean flags), a
// different shape from Deploy.trigger's plain string — derive a short label
// from whichever flag is set, first-match-wins.
type TriggerFlags = {
  firstBuild?: boolean | null;
  envUpdated?: boolean | null;
  manual?: boolean | null;
  deployedByRender?: boolean | null;
  clearCache?: boolean | null;
  rollback?: boolean | null;
} | null;

function triggerKey(trigger: TriggerFlags): string | null {
  if (!trigger) return null;
  if (trigger.rollback) return "services.eventsTriggerRollback";
  if (trigger.firstBuild) return "services.eventsTriggerFirstBuild";
  if (trigger.manual) return "services.eventsTriggerManual";
  if (trigger.envUpdated) return "services.eventsTriggerEnvUpdated";
  if (trigger.clearCache) return "services.eventsTriggerClearCache";
  if (trigger.deployedByRender) return "services.eventsTriggerDeployedByRender";
  return null;
}

function EventIcon({
  type,
  status,
  factStatus,
}: {
  type: string;
  status: string;
  factStatus: string;
}) {
  const iconProps = { className: "size-4", "aria-hidden": true } as const;

  if (type === "deploy_started") return <Rocket {...iconProps} />;
  if (type === "build_started") return <Hammer {...iconProps} />;
  if (type === "pre_deploy_started") return <Terminal {...iconProps} />;
  // Lifecycle-step endings (w7/m66) render by their outcome: check / cross / ban.
  if (
    type === "build_ended" ||
    type === "pre_deploy_ended" ||
    type === "job_run_ended"
  ) {
    if (factStatus === "failed") return <XCircle {...iconProps} />;
    if (factStatus === "canceled") return <Ban {...iconProps} />;
    return <CheckCircle2 {...iconProps} />;
  }
  if (type === "branch_deleted") return <GitBranch {...iconProps} />;
  if (type === "image_pull_failed" || type === "server_failed") {
    return <XCircle {...iconProps} />;
  }
  if (type === "deploy_ended") {
    return status === "update_failed" ? (
      <XCircle {...iconProps} />
    ) : (
      <CheckCircle2 {...iconProps} />
    );
  }
  if (type === "suspender_added" || type === "service_suspended") {
    return <PauseCircle {...iconProps} />;
  }
  if (
    type === "suspender_removed" ||
    type === "service_resumed" ||
    type === "server_available"
  ) {
    return <PlayCircle {...iconProps} />;
  }
  if (type === "server_restarted") return <RefreshCcw {...iconProps} />;
  if (
    type === "instance_count_changed" ||
    type === "autoscaling_config_changed" ||
    type === "autoscaling_started" ||
    type === "autoscaling_ended"
  ) {
    return <Scale {...iconProps} />;
  }
  return <CircleDot {...iconProps} />;
}

function eventIconClass(
  type: string,
  status: string,
  factStatus: string,
): string {
  if (
    status === "update_failed" ||
    factStatus === "failed" ||
    type === "image_pull_failed" ||
    type === "server_failed"
  ) {
    return "bg-destructive/10 text-destructive";
  }
  if (
    factStatus === "succeeded" ||
    (type === "deploy_ended" && status === "live")
  ) {
    return "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400";
  }
  if (
    type === "deploy_started" ||
    type === "build_started" ||
    type === "pre_deploy_started"
  ) {
    return "bg-primary/10 text-primary";
  }
  return "bg-muted text-muted-foreground";
}

// A lifecycle-step fact's status (w7/m66) → a Badge variant. succeeded reads as
// the default (accent), failed as destructive, canceled as a muted outline.
function lifecycleStatusVariant(
  status: string,
): "default" | "destructive" | "outline" {
  if (status === "failed") return "destructive";
  if (status === "canceled") return "outline";
  return "default";
}

export function ServiceEventsPage({ serviceId }: { serviceId: string }) {
  const { t } = useTranslations();
  const base = useServiceBase();
  // A cron_job's first-class run history hangs off the same landing tab.
  const { service } = useServer(serviceId);
  const [selectedTypes, setSelectedTypes] = useState<Set<string>>(
    () => new Set(SERVICE_EVENT_TYPES),
  );
  const [historyWindow] = useState(() => {
    const end = Date.now();
    return {
      startTime: new Date(end - 720 * 60 * 60 * 1000).toISOString(),
      endTime: new Date(end).toISOString(),
    };
  });
  const { events, loading, loadingMore, hasMore, error, refetch, loadMore } =
    useServiceEvents(serviceId, {
      limit: 20,
      ...historyWindow,
      historyStartTime: service?.createdAt ?? undefined,
      windowHours: 720,
    });

  const finishedDeployIds = new Set(
    events
      .filter((event) => event.type === "deploy_ended")
      .map((event) => event.details?.deployId)
      .filter((id): id is string => !!id),
  );
  const visibleEvents = events.filter((event) =>
    selectedTypes.has(event.type ?? ""),
  );

  return (
    <div className="space-y-6">
      <Card className="gap-0 overflow-hidden py-0">
        <CardHeader className="border-b py-5">
          <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-start">
            <div className="flex items-start gap-3">
              <div className="bg-primary/10 text-primary flex size-9 shrink-0 items-center justify-center rounded-lg">
                <Activity className="size-4" aria-hidden="true" />
              </div>
              <div className="min-w-0 space-y-1">
                <div className="flex items-center gap-2">
                  <CardTitle>{t("services.eventsTitle")}</CardTitle>
                  {!loading && !error && visibleEvents.length > 0 ? (
                    <Badge
                      variant="secondary"
                      aria-label={t("services.eventsCount", {
                        count: visibleEvents.length,
                      })}
                    >
                      {visibleEvents.length}
                    </Badge>
                  ) : null}
                </div>
                <CardDescription>
                  {t("services.eventsDescription")}
                </CardDescription>
              </div>
            </div>
            <ServiceEventFilter
              value={selectedTypes}
              onChange={setSelectedTypes}
            />
          </div>
        </CardHeader>
        <CardContent className="p-0">
          {loading && events.length === 0 ? (
            <div className="space-y-5 p-5 sm:p-6">
              {[0, 1, 2].map((i) => (
                <div key={i} className="flex items-start gap-3">
                  <Skeleton className="size-9 shrink-0 rounded-full" />
                  <div className="flex-1 space-y-2 pt-0.5">
                    <Skeleton className="h-4 w-40" />
                    <Skeleton className="h-3 w-24" />
                  </div>
                  <Skeleton className="h-3 w-12" />
                </div>
              ))}
            </div>
          ) : error ? (
            <div className="flex flex-col items-center px-6 py-14 text-center">
              <div className="bg-destructive/10 text-destructive mb-4 flex size-10 items-center justify-center rounded-full">
                <AlertCircle className="size-5" aria-hidden="true" />
              </div>
              <p className="font-medium">{t("services.eventsErrorTitle")}</p>
              <p className="text-muted-foreground mt-1 max-w-sm text-sm">
                {t("services.eventsErrorDescription")}
              </p>
              <Button
                className="mt-4"
                variant="outline"
                size="sm"
                onClick={() => void refetch()}
              >
                <RefreshCcw className="size-3.5" aria-hidden="true" />
                {t("services.eventsRetry")}
              </Button>
            </div>
          ) : events.length === 0 ? (
            <div className="flex flex-col items-center px-6 py-14 text-center">
              <div className="bg-muted text-muted-foreground mb-4 flex size-10 items-center justify-center rounded-full">
                <Activity className="size-5" aria-hidden="true" />
              </div>
              <p className="font-medium">{t("services.eventsEmptyTitle")}</p>
              <p className="text-muted-foreground mt-1 max-w-sm text-sm">
                {t("services.eventsEmpty")}
              </p>
            </div>
          ) : visibleEvents.length === 0 ? (
            <div className="flex flex-col items-center px-6 py-14 text-center">
              <div className="bg-muted text-muted-foreground mb-4 flex size-10 items-center justify-center rounded-full">
                <ListFilter className="size-5" aria-hidden="true" />
              </div>
              <p className="font-medium">
                {t("services.eventsFilterEmptyTitle")}
              </p>
              <p className="text-muted-foreground mt-1 max-w-sm text-sm">
                {t("services.eventsFilterEmpty")}
              </p>
            </div>
          ) : (
            <div className="divide-y">
              {visibleEvents.map((event) => {
                const details = event.details;
                const deployId = details?.deployId ?? "";
                // Render's deploy_started event intentionally has no terminal
                // status, while deploy_ended uses succeeded/failed instead of
                // the deploy object's live/update_failed vocabulary. Normalize
                // that API boundary for the shared badge and action helpers.
                const status =
                  event.type === "deploy_started"
                    ? finishedDeployIds.has(deployId)
                      ? ""
                      : "update_in_progress"
                    : details?.deployStatus === "succeeded"
                      ? "live"
                      : details?.deployStatus === "failed"
                        ? "update_failed"
                        : (details?.deployStatus ?? "");
                const trigger = triggerKey(details?.trigger ?? null);
                const preDeploy = preDeployKey(details?.preDeployStatus ?? "");
                const summary = (
                  <EventSummary
                    type={event.type ?? ""}
                    status={status}
                    factStatus={details?.status ?? ""}
                    trigger={trigger}
                    deployId={deployId}
                    actor={details?.actor || details?.triggeredByUser || ""}
                    preDeployStatus={details?.preDeployStatus ?? ""}
                    preDeploy={preDeploy}
                    timestamp={event.timestamp}
                    image={details?.image || null}
                    commitId={details?.commitId || null}
                    commitMessage={details?.commitMessage || null}
                    startedAt={details?.startedAt || null}
                    finishedAt={details?.finishedAt || null}
                    reasonCode={details?.reasonCode || null}
                    instanceId={details?.instanceId || null}
                    fromCount={details?.fromCount ?? null}
                    toCount={details?.toCount ?? null}
                    branchFrom={details?.branchFrom || null}
                    branchTo={details?.branchTo || null}
                    commitUrl={details?.commitUrl || null}
                  />
                );
                const hasAction =
                  !!deployId &&
                  (isCancelableDeployStatus(status) || status === "live");

                return (
                  <div
                    key={event.id}
                    className="hover:bg-muted/30 flex flex-col gap-3 px-5 py-4 transition-colors sm:flex-row sm:items-start sm:px-6"
                  >
                    {deployId ? (
                      <Link
                        to={`${base}/$serviceId/deploys/$deployId`}
                        params={{ serviceId, deployId }}
                        className="group min-w-0 flex-1 rounded-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                      >
                        {summary}
                      </Link>
                    ) : (
                      <div className="min-w-0 flex-1">{summary}</div>
                    )}
                    {hasAction ? (
                      <div className="shrink-0 self-end sm:self-center">
                        <DeployActions
                          serviceId={serviceId}
                          deployId={deployId}
                          status={status}
                          onChanged={() => void refetch()}
                        />
                      </div>
                    ) : null}
                  </div>
                );
              })}
              {hasMore ? (
                <div className="flex justify-center px-5 py-4 sm:px-6">
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={loadingMore}
                    onClick={() => void loadMore()}
                  >
                    <RefreshCcw
                      className={`size-3.5 ${loadingMore ? "animate-spin" : ""}`}
                      aria-hidden="true"
                    />
                    {loadingMore
                      ? t("services.eventsLoadingOlder")
                      : t("services.eventsLoadOlder")}
                  </Button>
                </div>
              ) : (
                <p className="text-muted-foreground px-5 py-4 text-center text-xs sm:px-6">
                  {t("services.eventsHistoryEnd")}
                </p>
              )}
            </div>
          )}
        </CardContent>
      </Card>

      {service && isCron(service) ? (
        <CronRunsSection serviceId={serviceId} />
      ) : null}
    </div>
  );
}

function EventSummary({
  type,
  status,
  factStatus,
  trigger,
  deployId,
  actor,
  preDeployStatus,
  preDeploy,
  timestamp,
  image,
  commitId,
  commitMessage,
  startedAt,
  finishedAt,
  reasonCode,
  instanceId,
  fromCount,
  toCount,
  branchFrom,
  branchTo,
  commitUrl,
}: {
  type: string;
  status: string;
  factStatus: string;
  trigger: string | null;
  deployId: string;
  actor: string;
  preDeployStatus: string;
  preDeploy: string | null;
  timestamp: string | null;
  image?: string | null;
  commitId?: string | null;
  commitMessage?: string | null;
  startedAt?: string | null;
  finishedAt?: string | null;
  reasonCode?: string | null;
  instanceId?: string | null;
  fromCount?: number | null;
  toCount?: number | null;
  branchFrom?: string | null;
  branchTo?: string | null;
  commitUrl?: string | null;
}) {
  const { t } = useTranslations();
  const isDeploy = type === "deploy_started" || type === "deploy_ended";
  const exactTimestamp = formatDateTime(timestamp);
  // Compute deploy duration (w1/m47): startedAt → finishedAt
  const deployDuration =
    startedAt && finishedAt
      ? Math.round(
          (new Date(finishedAt).getTime() - new Date(startedAt).getTime()) /
            1000,
        )
      : null;

  return (
    <div className="flex min-w-0 items-start gap-3">
      <div
        className={`flex size-9 shrink-0 items-center justify-center rounded-full ${eventIconClass(type, status, factStatus)}`}
      >
        <EventIcon type={type} status={status} factStatus={factStatus} />
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <p className="text-sm font-medium group-hover:text-primary">
            {t(serviceEventLabelKey(type) as Parameters<typeof t>[0])}
          </p>
          {isDeploy && status ? (
            <Badge variant={statusVariant(status)}>
              {t(statusKey(status) as Parameters<typeof t>[0])}
            </Badge>
          ) : null}
          {/* Lifecycle-step outcome (w7/m66): build/pre-deploy/job endings. */}
          {factStatus ? (
            <Badge variant={lifecycleStatusVariant(factStatus)}>
              {t(
                `services.eventsStatus.${factStatus}` as Parameters<
                  typeof t
                >[0],
              )}
            </Badge>
          ) : null}
        </div>
        <div className="text-muted-foreground mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs">
          {trigger ? (
            <span>{t(trigger as Parameters<typeof t>[0])}</span>
          ) : null}
          {actor ? <span>{t("services.eventsActor", { actor })}</span> : null}
          {deployId ? (
            <span className="font-mono">
              {t("services.eventsDeployReference", { id: deployId })}
            </span>
          ) : null}
          {timestamp && exactTimestamp ? (
            <time dateTime={timestamp} title={exactTimestamp}>
              {formatRelativeAge(timestamp)}
            </time>
          ) : null}
        </div>
        {/* Deploy enrichment (w1/m47): show commit, image, duration for deploy events */}
        {isDeploy && (
          <div className="text-muted-foreground mt-2 space-y-1 text-xs">
            {commitMessage ? (
              <p className="line-clamp-1">{commitMessage}</p>
            ) : null}
            <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
              {commitId ? (
                <span className="font-mono">{commitId.slice(0, 8)}</span>
              ) : null}
              {image ? <span className="truncate">{image}</span> : null}
              {deployDuration !== null ? <span>{deployDuration}s</span> : null}
            </div>
          </div>
        )}
        {preDeploy ? (
          <p
            className={`mt-2 text-xs ${
              preDeployStatus === "failed"
                ? "text-destructive"
                : "text-muted-foreground"
            }`}
          >
            {t(preDeploy as Parameters<typeof t>[0])}
          </p>
        ) : null}
        {reasonCode ? (
          <p className="text-muted-foreground mt-2 text-xs">
            {t(`services.eventsReason.${reasonCode}`)}
            {instanceId
              ? ` · ${t("services.eventsInstanceReference", { id: instanceId })}`
              : ""}
          </p>
        ) : null}
        {fromCount != null && toCount != null ? (
          <p className="text-muted-foreground mt-2 text-xs">
            {t("services.eventsReplicaChange", {
              from: fromCount,
              to: toCount,
            })}
          </p>
        ) : null}
        {branchFrom && branchTo ? (
          <p className="text-muted-foreground mt-2 text-xs">
            {t("services.eventsBranchChange", {
              from: branchFrom,
              to: branchTo,
            })}
          </p>
        ) : null}
        {type === "branch_deleted" && branchFrom ? (
          <p className="text-muted-foreground mt-2 text-xs">
            {t("services.eventsBranchDeleted", { branch: branchFrom })}
          </p>
        ) : null}
        {type === "commit_ignored" && commitId ? (
          <p className="text-muted-foreground mt-2 text-xs">
            {commitUrl ? (
              <a
                href={commitUrl}
                target="_blank"
                rel="noreferrer"
                className="font-mono underline underline-offset-2"
              >
                {commitId.slice(0, 8)}
              </a>
            ) : (
              <span className="font-mono">{commitId.slice(0, 8)}</span>
            )}
          </p>
        ) : null}
      </div>
    </div>
  );
}
