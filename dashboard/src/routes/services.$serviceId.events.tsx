import { SeparatedMeta } from "@/common/components/separated-meta";
import { STRETCHED_LINK_CLASS } from "@/common/lib/stretched-link";
import {
  aroundPlaceholder,
  PLACEHOLDER_SENTINEL,
} from "@/common/lib/i18n-placeholder";
import { CommitSha } from "@/features/deploys/components/commit-sha";
import { serviceTabHead } from "@/features/services/lib/service-tab-head";
import { useId, useMemo, useState, type ReactNode } from "react";
import { createFileRoute, Link } from "@tanstack/react-router";
import {
  Activity,
  AlertCircle,
  ChevronRight,
  History,
  ListFilter,
  RefreshCcw,
  type LucideIcon,
} from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import { ActivityGate } from "@/features/capabilities/components/activity-gate";
import { useWorkspaceSubjectLabel } from "@/common/hooks/use-workspace-subject-label";
import { UserAvatar } from "@/features/avatars/components/avatar";
import { EventIcon } from "@/features/events/components/event-icon";
import {
  eventIconClass,
  lifecycleStatusVariant,
} from "@/features/events/lib/event-presentation";
import { Card } from "@/common/components/ui/card";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useServer } from "@/features/services/hooks/use-server";
import { useBuildSources } from "@/features/services/hooks/use-build-sources";
import { useInstanceTypes } from "@/features/services/hooks/use-instance-types";
import { useProjects } from "@/features/projects/hooks/use-projects";
import { useEnvironments } from "@/features/environments/hooks/use-environments";
import { useServiceBase } from "@/features/services/lib/service-base";
import { useLocalDateTime } from "@/common/hooks/use-local-date";
import { useIsHydrated } from "@/common/hooks/use-is-hydrated";
import {
  formatMediumDate,
  formatMediumDateTime,
  formatMonthDay,
  formatMonthDayTime,
  formatTimeAgo,
  formatTimeToMinute,
} from "@/common/lib/format";
import { useNow } from "@/common/hooks/use-now";
import { cn } from "@/common/lib/utils/utils";
import {
  EVENT_ACTIONS_CELL,
  EVENT_BODY_CELL,
  EVENT_CHEVRON_CELL,
  EVENT_FOOTER_CLASS,
  EVENT_ICON_CELL,
  EVENT_ROW_CLASS,
  EVENT_TIME_CELL,
  EVENT_TITLE_CELL,
  EVENT_TOOLBAR_CLASS,
} from "@/features/events/lib/event-row-layout";
import {
  deployStatusVariant as statusVariant,
  deployStatusKey as statusKey,
  deployEndedStatus,
  preDeployStatusKey as preDeployKey,
  isCancelableDeployStatus,
} from "@/features/deploys/lib/deploy-status";
import { DeployActions } from "@/features/deploys/components/deploy-actions";
import {
  deploySubject,
  formatDeployDuration,
} from "@/features/deploys/lib/deploy-presentation";
import { useGoneBuilds } from "@/features/deploys/hooks/use-gone-builds";
import {
  autoDeployDisabledReasonKey,
  eventReasonKey,
  healthCheckResult,
  memoryLimitLabel,
  redeployTriggerKey,
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
import {
  ServiceEventRowsSkeleton,
  ServiceEventsSkeleton,
} from "@/common/components/route-skeletons";

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

/** The Events tab, behind the role's service-activity gate (w9/m621). */
export function ServiceEventsPage({ serviceId }: { serviceId: string }) {
  return (
    <ActivityGate>
      <ServiceEventsFeed serviceId={serviceId} />
    </ActivityGate>
  );
}

function ServiceEventsFeed({ serviceId }: { serviceId: string }) {
  const { t } = useTranslations();
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

  const filtered = hiddenTypes.size > 0;
  // Every known type is hidden, which the filtered-to-nothing state names
  // rather than suggesting more types it has none of. Older history can still
  // bring a type nobody hid, so Load older stays.
  const noTypeSelected =
    filtered &&
    [...SERVICE_EVENT_TYPES, ...uncataloguedTypes].every((type) =>
      hiddenTypes.has(type),
    );
  // How much the feed holds: the loaded count, a floor ("20+") while older
  // events remain (w4/700), and how many of them a filter leaves showing.
  const loadedTotal = hasMore ? `${events.length}+` : String(events.length);
  const countText = filtered
    ? t("services.eventsShownOf", {
        shown: visibleEvents.length,
        total: loadedTotal,
      })
    : t(hasMore ? "services.eventsCountAtLeast" : "services.eventsCount", {
        count: events.length,
      });

  const firstLoad = loading && events.length === 0;
  const retryButton = (
    <Button variant="outline" size="sm" onClick={() => void refetch()}>
      <RefreshCcw className="size-3.5" aria-hidden="true" />
      {t("services.eventsRetry")}
    </Button>
  );

  return (
    <Card className="gap-0 overflow-hidden py-0">
      {/* The tab's name is the heading above; the feed opens on what it
          holds and the filter, on one line at every width. An empty or
          unread feed has neither, so its state stands alone. */}
      {!firstLoad && events.length === 0 ? null : (
        <div className={EVENT_TOOLBAR_CLASS}>
          {loading && events.length === 0 ? (
            // The count's slot while the first page loads.
            <Skeleton className="h-4 w-36" />
          ) : (
            <p className="text-muted-foreground text-sm" aria-live="polite">
              {countText}
            </p>
          )}
          <ServiceEventFilter
            hidden={hiddenTypes}
            onChange={setHiddenTypes}
            extraTypes={uncataloguedTypes}
            // Nothing to filter until the first page lands.
            disabled={events.length === 0}
          />
        </div>
      )}
      {error && events.length > 0 ? (
        <div
          role="alert"
          className="flex items-center justify-between gap-3 border-b px-5 py-3 text-sm sm:px-6"
        >
          <span>{t("services.eventsErrorDescription")}</span>
          {retryButton}
        </div>
      ) : null}
      {firstLoad ? (
        <ServiceEventRowsSkeleton />
      ) : error && events.length === 0 ? (
        <FeedState
          role="alert"
          icon={AlertCircle}
          tone="destructive"
          title={t("services.eventsErrorTitle")}
          body={t("services.eventsErrorDescription")}
          action={retryButton}
        />
      ) : events.length === 0 ? (
        <FeedState
          icon={Activity}
          title={t("services.eventsEmptyTitle")}
          body={t("services.eventsEmpty")}
        />
      ) : (
        <>
          {visibleEvents.length === 0 ? (
            // Both ways forward the copy names, a click away: every type
            // back, or (below) older history.
            <FeedState
              icon={ListFilter}
              title={t(
                noTypeSelected
                  ? "services.eventsFilterNoneTitle"
                  : "services.eventsFilterEmptyTitle",
              )}
              body={t(
                noTypeSelected
                  ? "services.eventsFilterNone"
                  : "services.eventsFilterEmpty",
              )}
              action={
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setHiddenTypes(new Set())}
                >
                  {t("services.eventsFilterShowAll")}
                </Button>
              }
            />
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
                  <EventRow
                    key={event.id}
                    event={event}
                    status={status}
                    serviceId={serviceId}
                    actions={
                      hasAction ? (
                        <DeployActions
                          serviceId={serviceId}
                          deployId={deployId}
                          status={status}
                          commitId={details?.commitId}
                          // The events feed carries the trigger as Render's
                          // flag object rather than the deploys list's string;
                          // firstBuild is the same cause as trigger "create"
                          // (w4/103).
                          trigger={
                            details?.trigger?.firstBuild ? "create" : undefined
                          }
                          onChanged={() => void refetch()}
                        />
                      ) : null
                    }
                  />
                );
              })}
            </div>
          )}
          <div className={EVENT_FOOTER_CLASS}>
            {hasMore ? (
              <Button
                variant="outline"
                size="sm"
                disabled={loadingMore}
                onClick={() => void loadMore()}
              >
                <History
                  className={`size-3.5 ${loadingMore ? "animate-pulse" : ""}`}
                  aria-hidden="true"
                />
                {loadingMore
                  ? t("services.eventsLoadingOlder")
                  : t("services.eventsLoadOlder")}
              </Button>
            ) : (
              <p className="text-muted-foreground text-center text-xs">
                {t("services.eventsHistoryEnd")}
              </p>
            )}
          </div>
        </>
      )}
    </Card>
  );
}

/** The feed's empty, error and filtered-to-nothing states: one shape. */
function FeedState({
  role,
  icon: Icon,
  tone = "muted",
  title,
  body,
  action,
}: {
  /** `alert` for the load failure, so assistive technology announces it. */
  role?: "alert";
  icon: LucideIcon;
  tone?: "muted" | "destructive";
  title: string;
  body: string;
  action?: ReactNode;
}) {
  return (
    <div
      role={role}
      className="flex flex-col items-center px-6 py-14 text-center"
    >
      <div
        className={cn(
          "mb-4 flex size-10 items-center justify-center rounded-full",
          tone === "destructive"
            ? "bg-destructive/10 text-destructive"
            : "bg-muted text-muted-foreground",
        )}
      >
        <Icon className="size-5" aria-hidden="true" />
      </div>
      <p className="font-medium">{title}</p>
      <p className="text-muted-foreground mt-1 max-w-sm text-sm text-balance">
        {body}
      </p>
      {action ? <div className="mt-4">{action}</div> : null}
    </div>
  );
}

/**
 * Who acted on an event: the first user object the event names, with that
 * user's own email (w4/602), else a stored subject the roster resolves.
 */
function eventActor(details: EventDetails): {
  subject: string;
  email: string;
  avatarURL: string | null;
} {
  const user =
    [
      details?.suspendedByUser,
      details?.resumedByUser,
      details?.trigger?.user,
      details?.edgeCacheTrigger?.user,
    ].find((candidate) => candidate?.id) ?? null;
  if (user) {
    return {
      subject: user.id ?? "",
      email: user.email ?? "",
      avatarURL: user.avatarDownload?.url ?? null,
    };
  }
  const stored =
    (details?.actor && details.actor !== "user" ? details.actor : "") ||
    details?.triggeredByUser ||
    "";
  return { subject: stored, email: "", avatarURL: null };
}

function EventRow({
  event,
  status,
  serviceId,
  actions,
}: {
  event: ServiceEventView;
  /** The deploy's status this row shows, worked out across the feed. */
  status: string;
  serviceId: string;
  /** A deploy row's Cancel or Rollback. */
  actions: ReactNode;
}) {
  const { t } = useTranslations();
  const base = useServiceBase();
  const bodyId = useId();
  const type = event.type ?? "";
  const timestamp = event.timestamp;
  const details = event.details;
  const factStatus = details?.status ?? "";
  const trigger = triggerKey(details?.trigger ?? null);
  const deployId = details?.deployId ?? "";
  const preDeployStatus = details?.preDeployStatus ?? "";
  const preDeploy = preDeployKey(preDeployStatus);
  const actor = eventActor(details);
  const actorLabel = useWorkspaceSubjectLabel(actor.subject, actor.email);
  const image = details?.image || null;
  const commitId = details?.commitId || null;
  const commitMessage = details?.commitMessage || null;
  const startedAt = details?.startedAt || null;
  const finishedAt = details?.finishedAt || null;
  const reasonCode = details?.reasonCode || null;
  const memoryLimit = details?.memoryLimit || null;
  // w6/m634: what a failed health check got back, in Render's words.
  const checkResult = healthCheckResult(type, reasonCode, details?.unhealthy);
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
  const autoDeployReason =
    type === "auto_deploy_disabled"
      ? autoDeployDisabledReasonKey(details?.autoDeployDisabledReason)
      : null;
  const redeployTrigger =
    type === "zero_downtime_redeploy_started"
      ? redeployTriggerKey(details?.redeployTrigger)
      : null;
  const fromBuildSourceId = details?.fromBuildSourceId || null;
  const toBuildSourceId = details?.toBuildSourceId || null;
  const artifactBuildId =
    type === "artifact_fetch_failed" ? details?.buildId || null : null;
  const artifactMessage =
    type === "artifact_fetch_failed" ? details?.message || null : null;
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
  const title =
    namedTitle ?? t(serviceEventLabelKey(type) as Parameters<typeof t>[0]);
  // The Deploys list's own duration (w4/601), so one deploy reads the same on
  // both tabs.
  const deployDuration = formatDeployDuration(startedAt, finishedAt);

  return (
    <div
      className={cn(
        EVENT_ROW_CLASS,
        deployId && "transition-colors hover:bg-muted/40",
      )}
    >
      <div
        className={cn(
          EVENT_ICON_CELL,
          "flex items-center justify-center",
          eventIconClass(type, status, factStatus),
        )}
      >
        <EventIcon type={type} status={status} factStatus={factStatus} />
      </div>
      <div className={EVENT_TITLE_CELL}>
        {deployId ? (
          // The whole row opens the deploy; the row's details describe the
          // link.
          <Link
            to={`${base}/$serviceId/deploys/$deployId`}
            params={{ serviceId, deployId }}
            aria-describedby={bodyId}
            className={cn(
              "text-sm leading-5 font-medium",
              STRETCHED_LINK_CLASS,
            )}
          >
            {title}
          </Link>
        ) : (
          <p className="text-sm leading-5 font-medium">{title}</p>
        )}
        {isDeploy && status ? (
          <Badge variant={statusVariant(status)}>
            {t(statusKey(status) as Parameters<typeof t>[0])}
          </Badge>
        ) : null}
        {/* Lifecycle-step outcome (w7/m66): build/pre-deploy/job endings. */}
        {factStatus ? (
          <Badge variant={lifecycleStatusVariant(factStatus)}>
            {t(
              `services.eventsStatus.${factStatus}` as Parameters<typeof t>[0],
            )}
          </Badge>
        ) : null}
      </div>
      {timestamp ? <EventTime timestamp={timestamp} /> : null}
      <div id={bodyId} className={EVENT_BODY_CELL}>
        <DeployFailureReason
          reason={details?.failureReason}
          template={toDeployReasonTemplate(details?.failureReasonTemplate)}
        />
        <DeployFailureReason
          reason={details?.cancelReason}
          template={toDeployReasonTemplate(details?.cancelReasonTemplate)}
          tone="neutral"
        />
        {/* w4/m112: the same stall_reason column the deploy detail page
            reads, so the feed and that page cannot disagree about why an
            open rollout is stuck. deploy_started rows only — the server
            leaves it empty everywhere else. */}
        <DeployFailureReason
          reason={details?.stallReason}
          template={toDeployReasonTemplate(details?.stallReasonTemplate)}
          tone="neutral"
        />
        <SeparatedMeta>
          {trigger ? (
            <span>{t(trigger as Parameters<typeof t>[0])}</span>
          ) : null}
          {actor.subject ? (
            <span className="inline-flex items-center gap-1">
              {/* Who acted, with their avatar, as Render's events feed
                  (w1/m648): a person's, never a subject's. */}
              {actorLabel.includes("@") ? (
                <UserAvatar
                  email={actorLabel}
                  imageURL={actor.avatarURL}
                  size="mini"
                />
              ) : null}
              <span>{t("services.eventsActor", { actor: actorLabel })}</span>
            </span>
          ) : null}
          {deployId ? <DeployReference id={deployId} /> : null}
          {artifactBuildId ? <BuildReference id={artifactBuildId} /> : null}
        </SeparatedMeta>
        {/* Deploy enrichment (w1/m47): commit, message, image and
            duration on one line, as a deploy row reads on the Deploys tab. */}
        {isDeploy && (commitId || commitMessage || image || deployDuration) ? (
          <SeparatedMeta>
            {commitId ? <CommitSha commitId={commitId} url={null} /> : null}
            {commitMessage ? (
              <span className="min-w-0 truncate">
                {deploySubject(commitMessage)}
              </span>
            ) : null}
            {image ? <span className="min-w-0 truncate">{image}</span> : null}
            {deployDuration !== null ? <span>{deployDuration}</span> : null}
          </SeparatedMeta>
        ) : null}
        {preDeploy ? (
          <p
            className={
              preDeployStatus === "failed" ? "text-destructive" : undefined
            }
          >
            {t(preDeploy as Parameters<typeof t>[0])}
          </p>
        ) : null}
        {artifactMessage ? (
          <p className="text-destructive">{artifactMessage}</p>
        ) : null}
        {reasonCode && !artifactMessage ? (
          <p>
            {t(eventReasonKey(type, reasonCode, memoryLimit, checkResult), {
              exitCode: exitCode ?? "",
              memory: memoryLimit ? memoryLimitLabel(memoryLimit) : "",
              result: checkResult ?? "",
            })}
            {instanceId
              ? ` · ${t("services.eventsInstanceReference", { id: instanceId })}`
              : ""}
          </p>
        ) : null}
        {fromCount != null && toCount != null ? (
          <p>
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
          <p>
            {t("services.eventsDiskUsage", {
              disk: details?.diskName ?? "",
              mountPath: details?.mountPath ?? "",
              usage: usagePercent.toFixed(1),
              threshold: thresholdPercent.toFixed(0),
            })}
          </p>
        ) : null}
        {type === "display_name_changed" && details?.displayNameTo ? (
          <p>
            {t("services.eventsDisplayNameChange", {
              from: details.displayNameFrom ?? "",
              to: details.displayNameTo,
            })}
          </p>
        ) : null}
        {autoDeployReason ? (
          <p>{t(autoDeployReason as Parameters<typeof t>[0])}</p>
        ) : null}
        {redeployTrigger ? (
          <p>{t(redeployTrigger as Parameters<typeof t>[0])}</p>
        ) : null}
        {type === "artifact_source_changed" &&
        (fromBuildSourceId || toBuildSourceId) ? (
          <ArtifactSourceChange from={fromBuildSourceId} to={toBuildSourceId} />
        ) : null}
        {branchFrom && branchTo ? (
          <p>
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
          <p>{t("services.eventsBranchDeleted", { branch: branchFrom })}</p>
        ) : null}
        {type === "commit_ignored" && commitId ? (
          <p>
            <CommitSha commitId={commitId} url={commitUrl} className="z-10" />
          </p>
        ) : null}
      </div>
      {actions ? (
        <div className={cn(EVENT_ACTIONS_CELL, "relative z-10")}>{actions}</div>
      ) : null}
      {deployId ? (
        <ChevronRight
          aria-hidden="true"
          className={cn(EVENT_CHEVRON_CELL, "text-muted-foreground size-4")}
        />
      ) : null}
    </div>
  );
}

/**
 * When an event happened, in the viewer's zone (deferred past hydration,
 * w6/m107): the date and clock reading, then how long ago — said as "ago", so
 * it never reads as how long the event took. This year's events leave the year
 * off, which repeated on every row. Its own component, so the minute tick that
 * moves "23 hours ago" redraws only this, not the row.
 */
function EventTime({ timestamp }: { timestamp: string }) {
  const { t, i18n } = useTranslations();
  const hydrated = useIsHydrated();
  const now = useNow();
  const exact = useLocalDateTime(timestamp);
  const local = hydrated ? timestamp : null;
  const language = i18n.language;
  const thisYear =
    !!local && new Date(local).getFullYear() === new Date(now).getFullYear();
  const ago = local
    ? ` · ${formatTimeAgo(local, { now, language, justNow: t("common.justNow") })}`
    : null;
  return (
    <time
      dateTime={timestamp}
      title={exact ?? undefined}
      className={EVENT_TIME_CELL}
    >
      {/* A phone's line under the title, then the wide layout's column: one
          is always display:none, so each is read once. */}
      <span className="block @2xl/page:hidden">
        {local
          ? (thisYear ? formatMonthDayTime : formatMediumDateTime)(
              local,
              language,
            )
          : null}
        {ago}
      </span>
      <span className="hidden @2xl/page:block">
        {local
          ? (thisYear ? formatMonthDay : formatMediumDate)(local, language)
          : null}
      </span>
      <span className="hidden whitespace-nowrap @2xl/page:block">
        {local ? formatTimeToMinute(local, language) : null}
        {ago}
      </span>
    </time>
  );
}

/**
 * "Deploy dep-live-001": the word in the UI font, as the line's other labels
 * are, and only the id in mono, as the commit hash beside it is. The id goes
 * wherever the language's own phrase puts it.
 */
function DeployReference({ id }: { id: string }) {
  const { t } = useTranslations();
  const [before, after] = aroundPlaceholder(
    t("services.eventsDeployReference", { id: PLACEHOLDER_SENTINEL }),
  );
  return (
    <span>
      {before}
      <span className="font-mono">{id}</span>
      {after}
    </span>
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
    <div className="space-y-1">
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
    <p>
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
    <p>
      {t("services.eventsPlanChange", {
        from: from
          ? (byID(from)?.name ?? from)
          : t("services.eventsPlacementNone"),
        to: byID(to)?.name ?? to,
      })}
    </p>
  );
}

/** "Build bld-…": mirrors DeployReference for artifact_fetch_failed (w4/m510). */
function BuildReference({ id }: { id: string }) {
  const { t } = useTranslations();
  const [before, after] = aroundPlaceholder(
    t("services.eventsBuildReference", { id: PLACEHOLDER_SENTINEL }),
  );
  return (
    <span>
      {before}
      <span className="font-mono">{id}</span>
      {after}
    </span>
  );
}

/**
 * An artifact_source_changed event's build-source link sides (w4/m510).
 * Resolves bds-… ids to their build source names when still present, falling
 * back to the id when deleted or loading. Mounted only on build-source link
 * rows so other events do not fetch the build-source list.
 */
function ArtifactSourceChange({
  from,
  to,
}: {
  from: string | null;
  to: string | null;
}) {
  const { t } = useTranslations();
  const { sources } = useBuildSources(true);
  const label = (id: string) =>
    sources.find((item) => item.id === id)?.name ?? id;
  if (from && to) {
    return (
      <p>
        {t("services.eventsArtifactSourceChanged", {
          from: label(from),
          to: label(to),
        })}
      </p>
    );
  }
  if (to) {
    return <p>{t("services.eventsArtifactSourceLinked", { to: label(to) })}</p>;
  }
  if (from) {
    return (
      <p>{t("services.eventsArtifactSourceUnlinked", { from: label(from) })}</p>
    );
  }
  return null;
}
