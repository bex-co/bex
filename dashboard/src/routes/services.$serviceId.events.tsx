import { serviceTabHead } from "@/features/services/lib/service-tab-head";
import { useMemo, useState } from "react";
import { createFileRoute, Link } from "@tanstack/react-router";
import { Activity, AlertCircle, ListFilter, RefreshCcw } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspaceSubjectLabel } from "@/common/hooks/use-workspace-subject-label";
import { EventIcon } from "@/features/events/components/event-icon";
import {
  eventIconClass,
  lifecycleStatusVariant,
} from "@/features/events/lib/event-presentation";
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
import { useInstanceTypes } from "@/features/services/hooks/use-instance-types";
import { useProjects } from "@/features/projects/hooks/use-projects";
import { useEnvironments } from "@/features/environments/hooks/use-environments";
import { useServiceBase } from "@/features/services/lib/service-base";
import { RelativeAge } from "@/common/components/relative-time";
import { useLocalDateTime } from "@/common/hooks/use-local-date";
import {
  deployStatusVariant as statusVariant,
  deployStatusKey as statusKey,
  deployEndedStatus,
  preDeployStatusKey as preDeployKey,
  isCancelableDeployStatus,
} from "@/features/deploys/lib/deploy-status";
import { DeployActions } from "@/features/deploys/components/deploy-actions";
import { formatDeployDuration } from "@/features/deploys/lib/deploy-presentation";
import { useGoneBuilds } from "@/features/deploys/hooks/use-gone-builds";
import {
  eventReasonKey,
  memoryLimitLabel,
} from "@/features/services/lib/event-reason";
import { DeployFailureReason } from "@/features/deploys/components/deploy-failure-reason";
import { toDeployReasonTemplate } from "@/features/deploys/lib/deploy-reason";
import {
  useServiceEvents,
  type ServiceEventView,
} from "@/features/events/hooks/use-service-events";
import { ServiceEventFilter } from "@/features/events/components/service-event-filter";
import {
  SERVICE_EVENT_TYPES,
  serviceEventLabelKey,
} from "@/features/events/service-event-catalog";
const CATALOG_TYPES = new Set(SERVICE_EVENT_TYPES);
import { ServiceEventsSkeleton } from "@/common/components/route-skeletons";

type EventDetails = ServiceEventView["details"];

/** The titles that name their group or domain, by event type. */
const NAMED_TITLES: Partial<
  Record<
    string,
    { key: string; name: (d: EventDetails) => string | null | undefined }
  >
> = {
  env_group_linked: {
    key: "services.eventsEnvGroupLinkedNamed",
    name: (d) => d?.envGroupName,
  },
  env_group_unlinked: {
    key: "services.eventsEnvGroupUnlinkedNamed",
    name: (d) => d?.envGroupName,
  },
  custom_domain_added: {
    key: "services.eventsCustomDomainAddedNamed",
    name: (d) => d?.domain,
  },
  custom_domain_removed: {
    key: "services.eventsCustomDomainRemovedNamed",
    name: (d) => d?.domain,
  },
  custom_domain_verified: {
    key: "services.eventsCustomDomainVerifiedNamed",
    name: (d) => d?.domain,
  },
};

export const Route = createFileRoute("/services/$serviceId/events")({
  head: serviceTabHead("services.navEvents", "/services/$serviceId"),
  component: RouteComponent,
  pendingComponent: ServiceEventsSkeleton,
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
  /** bex extension — Render's trigger vocabulary has no hook flag (w4/104). */
  deployHook?: boolean | null;
} | null;

function triggerKey(trigger: TriggerFlags): string | null {
  if (!trigger) return null;
  if (trigger.rollback) return "services.eventsTriggerRollback";
  if (trigger.firstBuild) return "services.eventsTriggerFirstBuild";
  // Before deployHook existed, a hook POST arrived as `manual` and read
  // "Manual deploy" for a rollout nobody clicked (w4/104). It is checked ahead
  // of manual so a server that still sets both cannot resurrect the old label.
  if (trigger.deployHook) return "services.eventsTriggerDeployHook";
  if (trigger.manual) return "services.eventsTriggerManual";
  if (trigger.envUpdated) return "services.eventsTriggerEnvUpdated";
  if (trigger.clearCache) return "services.eventsTriggerClearCache";
  if (trigger.deployedByRender) return "services.eventsTriggerDeployedByRender";
  return null;
}

export function ServiceEventsPage({ serviceId }: { serviceId: string }) {
  const { t } = useTranslations();
  const base = useServiceBase();
  // The service's createdAt bounds the history and autoDeploy words the
  // auto-deploy events; a cron job's runs have their own Runs page (w2/m360).
  const { service } = useServer(serviceId, { poll: false });
  // Hidden, not selected (w6/m122): seeding from the catalog made the feed render
  // `events ∩ catalog`, so any type the backend emits that the catalog omits was
  // invisible AND — the option list being catalog-derived too — unselectable. An
  // empty exclusion set shows whatever the API returned.
  const [hiddenTypes, setHiddenTypes] = useState<Set<string>>(() => new Set());
  const [historyWindow] = useState(() => {
    const end = Date.now();
    return {
      startTime: new Date(end - 720 * 60 * 60 * 1000).toISOString(),
      endTime: new Date(end).toISOString(),
    };
  });
  const goneBuilds = useGoneBuilds(serviceId);
  const { events, loading, loadingMore, hasMore, error, refetch, loadMore } =
    useServiceEvents(serviceId, {
      limit: 20,
      live: true,
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
  const visibleEvents = events.filter(
    (event) => !hiddenTypes.has(event.type ?? ""),
  );
  // Types the feed carries that this build's catalog has no entry for. They
  // already render (the filter is fail-open and serviceEventLabelKey falls back
  // to the generic label); listing them here also makes them de-selectable. A
  // row with a missing/empty type stays permanently visible rather than becoming
  // a nameless checkbox — an absent type is not a vocabulary member to filter on.
  const uncataloguedTypes = useMemo(
    () =>
      [
        ...new Set(
          events
            .map((event) => event.type ?? "")
            .filter((type) => type !== "" && !CATALOG_TYPES.has(type)),
        ),
      ].sort(),
    [events],
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
                  {/* Counts what the API returned for the window, not the
                      post-filter subset: the badge answers "how much happened",
                      while the list below answers "what am I looking at". */}
                  {!loading && !error && events.length > 0 ? (
                    <Badge
                      variant="secondary"
                      aria-label={t("services.eventsCount", {
                        count: events.length,
                      })}
                    >
                      {events.length}
                    </Badge>
                  ) : null}
                </div>
                <CardDescription>
                  {t("services.eventsDescription")}
                </CardDescription>
              </div>
            </div>
            <ServiceEventFilter
              hidden={hiddenTypes}
              onChange={setHiddenTypes}
              extraTypes={uncataloguedTypes}
            />
          </div>
        </CardHeader>
        <CardContent className="p-0">
          {error && events.length > 0 ? (
            <div
              role="alert"
              className="flex items-center justify-between gap-3 border-b px-5 py-3 text-sm"
            >
              <span>{t("services.eventsErrorDescription")}</span>
              <Button
                variant="outline"
                size="sm"
                onClick={() => void refetch()}
              >
                <RefreshCcw className="size-3.5" aria-hidden="true" />
                {t("services.eventsRetry")}
              </Button>
            </div>
          ) : null}
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
          ) : error && events.length === 0 ? (
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
                // status. deploy_ended badging lives in the shared deploy-status
                // helper so both surfaces read the same rule.
                const status =
                  event.type === "deploy_started"
                    ? finishedDeployIds.has(deployId)
                      ? ""
                      : "update_in_progress"
                    : deployEndedStatus(details);
                const trigger = triggerKey(details?.trigger ?? null);
                const preDeploy = preDeployKey(details?.preDeployStatus ?? "");
                const actor = eventActor(details);
                const summary = (
                  <EventSummary
                    type={event.type ?? ""}
                    status={status}
                    factStatus={details?.status ?? ""}
                    trigger={trigger}
                    deployId={deployId}
                    actor={actor.subject}
                    actorEmail={actor.email}
                    preDeployStatus={details?.preDeployStatus ?? ""}
                    preDeploy={preDeploy}
                    timestamp={event.timestamp}
                    details={details}
                  />
                );
                // Cancel an in-progress deploy, or roll back a PREVIOUS
                // (deactivated) one — never the current live deploy, which would
                // be a no-op restart (w4/051). Mirrors the deploys list's guard
                // so the events feed, list, and detail page all agree. A
                // previous deploy whose build the registry no longer holds
                // cannot be rolled back to, so it gets no action (w8/m156).
                const hasAction =
                  !!deployId &&
                  (isCancelableDeployStatus(status) ||
                    (status === "deactivated" && !goneBuilds.has(deployId)));

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
                          commitId={details?.commitId}
                          commitMessage={details?.commitMessage}
                          // The events feed carries the trigger as Render's
                          // flag object rather than the deploys list's string;
                          // firstBuild is the same cause as trigger "create"
                          // (w4/103).
                          trigger={
                            details?.trigger?.firstBuild ? "create" : undefined
                          }
                          autoDeployOn={
                            service ? service.autoDeploy === true : undefined
                          }
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
    </div>
  );
}

/**
 * Who acted on an event: the first user object the event names, with that
 * user's own email (w4/602), else a stored subject the roster resolves.
 */
function eventActor(details: EventDetails): { subject: string; email: string } {
  const user =
    [
      details?.suspendedByUser,
      details?.resumedByUser,
      details?.trigger?.user,
      details?.edgeCacheTrigger?.user,
    ].find((candidate) => candidate?.id) ?? null;
  if (user) return { subject: user.id ?? "", email: user.email ?? "" };
  const stored =
    (details?.actor && details.actor !== "user" ? details.actor : "") ||
    details?.triggeredByUser ||
    "";
  return { subject: stored, email: "" };
}

function EventSummary({
  type,
  status,
  factStatus,
  trigger,
  deployId,
  actor,
  actorEmail,
  preDeployStatus,
  preDeploy,
  timestamp,
  details,
}: {
  type: string;
  status: string;
  factStatus: string;
  trigger: string | null;
  deployId: string;
  actor: string;
  actorEmail: string;
  preDeployStatus: string;
  preDeploy: string | null;
  timestamp: string | null;
  // The event's own payload. Its fields were previously re-spelled as twelve
  // pass-through props, so a rename on ServiceEventView had to be mirrored in
  // two more places to keep compiling.
  details: ServiceEventView["details"];
}) {
  const { t } = useTranslations();
  const actorLabel = useWorkspaceSubjectLabel(actor, actorEmail);
  const image = details?.image || null;
  const commitId = details?.commitId || null;
  const commitMessage = details?.commitMessage || null;
  const startedAt = details?.startedAt || null;
  const finishedAt = details?.finishedAt || null;
  const reasonCode = details?.reasonCode || null;
  const memoryLimit = details?.memoryLimit || null;
  const exitCode = details?.exitCode ?? null;
  const instanceId = details?.instanceId || null;
  // An autoscaling start/end's replica pair, or an instance-count change's
  // (w3/m16): the audit-sourced type carries its values in its own fields.
  const fromCount = details?.fromCount ?? details?.instanceCountFrom ?? null;
  const toCount = details?.toCount ?? details?.instanceCountTo ?? null;
  const planTo = details?.planTo || null;
  const branchFrom = details?.branchFrom || null;
  const branchTo = details?.branchTo || null;
  const usagePercent = details?.usagePercent ?? null;
  const thresholdPercent = details?.thresholdPercent ?? null;
  const commitUrl = details?.commitUrl || null;
  // service_moved placement pair (w6/m134). A dimension renders only when it
  // actually changed; an absent side reads as the placement-none placeholder.
  const projectFrom = details?.projectFrom ?? null;
  const projectTo = details?.projectTo ?? null;
  const environmentFrom = details?.environmentFrom ?? null;
  const environmentTo = details?.environmentTo ?? null;
  // An env-group link names its group (w4/563) and a custom-domain row its
  // hostname (w4/m337); a row recorded before that keeps the bare type label.
  const named = NAMED_TITLES[type];
  const titleName = named?.name(details);
  const namedTitle =
    named && titleName ? t(named.key, { name: titleName }) : null;
  const projectChanged = type === "service_moved" && projectFrom !== projectTo;
  const environmentChanged =
    type === "service_moved" && environmentFrom !== environmentTo;
  const isDeploy = type === "deploy_started" || type === "deploy_ended";
  const exactTimestamp = useLocalDateTime(timestamp);
  // The Deploys list's own duration (w4/601), so one deploy reads the same on
  // both tabs.
  const deployDuration = formatDeployDuration(startedAt, finishedAt);

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
            {namedTitle ??
              t(serviceEventLabelKey(type) as Parameters<typeof t>[0])}
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
        <DeployFailureReason
          reason={details?.failureReason}
          template={toDeployReasonTemplate(details?.failureReasonTemplate)}
          className="mt-1"
        />
        <DeployFailureReason
          reason={details?.cancelReason}
          template={toDeployReasonTemplate(details?.cancelReasonTemplate)}
          tone="neutral"
          className="mt-1"
        />
        {/* w4/m112: the same stall_reason column the deploy detail page
            reads, so the feed and that page cannot disagree about why an
            open rollout is stuck. deploy_started rows only — the server
            leaves it empty everywhere else. */}
        <DeployFailureReason
          reason={details?.stallReason}
          template={toDeployReasonTemplate(details?.stallReasonTemplate)}
          tone="neutral"
          className="mt-1"
        />
        <div className="text-muted-foreground mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs">
          {trigger ? (
            <span>{t(trigger as Parameters<typeof t>[0])}</span>
          ) : null}
          {actor ? (
            <span>{t("services.eventsActor", { actor: actorLabel })}</span>
          ) : null}
          {deployId ? (
            <span className="font-mono">
              {t("services.eventsDeployReference", { id: deployId })}
            </span>
          ) : null}
          {timestamp ? (
            <RelativeAge
              value={timestamp}
              title={exactTimestamp ?? undefined}
            />
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
              {deployDuration !== null ? <span>{deployDuration}</span> : null}
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
            {t(eventReasonKey(type, reasonCode, memoryLimit), {
              exitCode: exitCode ?? "",
              memory: memoryLimit ? memoryLimitLabel(memoryLimit) : "",
            })}
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
        {type === "autoscaling_config_changed" && details ? (
          <AutoscalingChange details={details} />
        ) : null}
        {type === "plan_changed" && planTo ? (
          <PlanChange from={details?.planFrom || null} to={planTo} />
        ) : null}
        {usagePercent != null && thresholdPercent != null ? (
          <p className="text-muted-foreground mt-2 text-xs">
            {t("services.eventsDiskUsage", {
              disk: details?.diskName ?? "",
              mountPath: details?.mountPath ?? "",
              usage: usagePercent.toFixed(1),
              threshold: thresholdPercent.toFixed(0),
            })}
          </p>
        ) : null}
        {type === "display_name_changed" && details?.displayNameTo ? (
          <p className="text-muted-foreground mt-2 text-xs">
            {t("services.eventsDisplayNameChange", {
              from: details.displayNameFrom ?? "",
              to: details.displayNameTo,
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
        {projectChanged || environmentChanged ? (
          <PlacementChange
            projectFrom={projectFrom}
            projectTo={projectTo}
            environmentFrom={environmentFrom}
            environmentTo={environmentTo}
          />
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

/**
 * A service_moved event's placement pair by name, as the project page and the
 * breadcrumbs name it (w4/561). The event carries ids, the stable contract on
 * every API; a side that no longer exists, or one still loading, reads as its
 * id so the row never goes blank. Mounted only on a move event, so other rows
 * read no lists.
 */
function PlacementChange({
  projectFrom,
  projectTo,
  environmentFrom,
  environmentTo,
}: {
  projectFrom: string | null;
  projectTo: string | null;
  environmentFrom: string | null;
  environmentTo: string | null;
}) {
  const { t } = useTranslations();
  const projectChanged = projectFrom !== projectTo;
  const environmentChanged = environmentFrom !== environmentTo;
  const { projects } = useProjects({ poll: false });
  // Each side's environment is in its own project's list.
  const { environments: fromEnvironments } = useEnvironments(
    environmentChanged ? projectFrom : null,
    { poll: false },
  );
  const { environments: toEnvironments } = useEnvironments(
    environmentChanged && projectTo !== projectFrom ? projectTo : null,
    { poll: false },
  );
  const label = (
    id: string | null,
    named: readonly { id: string; name: string }[],
  ) =>
    id
      ? (named.find((item) => item.id === id)?.name ?? id)
      : t("services.eventsPlacementNone");
  const environments = [...fromEnvironments, ...toEnvironments];
  return (
    <div className="text-muted-foreground mt-2 space-y-1 text-xs">
      {projectChanged ? (
        <p>
          {t("services.eventsProjectChange", {
            from: label(projectFrom, projects),
            to: label(projectTo, projects),
          })}
        </p>
      ) : null}
      {environmentChanged ? (
        <p>
          {t("services.eventsEnvironmentChange", {
            from: label(environmentFrom, environments),
            to: label(environmentTo, environments),
          })}
        </p>
      ) : null}
    </div>
  );
}

/**
 * An autoscaling configuration change, each side Off or a range with its CPU
 * and memory targets (w4/568, w4/m335), so a change to a target alone reads as
 * one. Settings a disable kept read as Off until they run again (w4/597). A row
 * recorded before the values were keeps its bare title, and one recorded before
 * the targets were shows the range alone.
 */
function AutoscalingChange({
  details,
}: {
  details: NonNullable<ServiceEventView["details"]>;
}) {
  const { t } = useTranslations();
  const { autoscalingMinFrom, autoscalingMaxFrom } = details;
  const { autoscalingMinTo, autoscalingMaxTo } = details;
  const fromOff = details.autoscalingEnabledFrom === false;
  if (
    autoscalingMinFrom == null &&
    autoscalingMaxFrom == null &&
    autoscalingMinTo == null &&
    autoscalingMaxTo == null
  ) {
    return null;
  }
  const side = (
    min?: number | null,
    max?: number | null,
    cpu?: number | null,
    memory?: number | null,
  ) => {
    if (min == null || max == null) {
      return t("services.eventsAutoscalingOff");
    }
    if (cpu != null && memory != null) {
      return t("services.eventsAutoscalingRangeCPUMemory", {
        min,
        max,
        cpu,
        memory,
      });
    }
    if (cpu != null) {
      return t("services.eventsAutoscalingRangeCPU", { min, max, cpu });
    }
    if (memory != null) {
      return t("services.eventsAutoscalingRangeMemory", { min, max, memory });
    }
    return t("services.eventsAutoscalingRange", { min, max });
  };
  return (
    <p className="text-muted-foreground mt-2 text-xs">
      {t("services.eventsAutoscalingChange", {
        from: fromOff
          ? t("services.eventsAutoscalingOff")
          : side(
              autoscalingMinFrom,
              autoscalingMaxFrom,
              details.autoscalingCPUFrom,
              details.autoscalingMemoryFrom,
            ),
        to: side(
          autoscalingMinTo,
          autoscalingMaxTo,
          details.autoscalingCPUTo,
          details.autoscalingMemoryTo,
        ),
      })}
    </p>
  );
}

/** A plan change by its instance types' names, as the plan picker names them (w4/568). */
function PlanChange({ from, to }: { from: string | null; to: string }) {
  const { t } = useTranslations();
  const { byID } = useInstanceTypes();
  return (
    <p className="text-muted-foreground mt-2 text-xs">
      {t("services.eventsPlanChange", {
        from: from
          ? (byID(from)?.name ?? from)
          : t("services.eventsPlacementNone"),
        to: byID(to)?.name ?? to,
      })}
    </p>
  );
}
