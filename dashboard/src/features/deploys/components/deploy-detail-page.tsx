import { useMemo } from "react";
import { Link } from "@tanstack/react-router";
import { ChevronLeft } from "lucide-react";
import { EmptyState } from "@/common/components/empty-state";
import { Button } from "@/common/components/ui/button";
import { useNotFoundRedirect } from "@/common/hooks/use-not-found-redirect";
import { useTranslations } from "@/common/hooks/use-translations";
import { relayableReason } from "@/common/lib/graphql-error";
import { isUnauthenticatedError } from "@/common/apollo/auth-error-link";
import { ResourceLoadError } from "@/common/components/resource-load-error";
import { useServiceBase } from "@/features/services/lib/service-base";
import { useBuildSource } from "@/features/services/hooks/use-build-sources";
import { useServer } from "@/features/services/hooks/use-server";
import { ActivityGate } from "@/features/capabilities/components/activity-gate";
import { LogsGate } from "@/features/capabilities/components/logs-gate";
import { useDeploy, type DeployView } from "../hooks/use-deploy";
import { useDeployScaling } from "../hooks/use-deploy-scaling";
import { useCanceledStepEnd } from "../hooks/use-canceled-step-end";
import { deployWindowEnd } from "../lib/deploy-window";
import { deployTitle } from "../lib/deploy-presentation";
import { offersDashboardRollback } from "../lib/deploy-status";
import { rangeStart, type LogRange } from "../lib/log-range";
import { DeployHeader } from "./deploy-header";
import { DeployLogPanel } from "./deploy-log-panel";
import { DeployTimeline } from "./deploy-timeline";
import { DeployActions } from "./deploy-actions";
import { DeployRollback } from "./deploy-rollback";
import { DeployNotices } from "./deploy-notices";
import { DEPLOY_BODY_CLASS, DEPLOY_PAGE_TOP_CLASS } from "./deploy-page-layout";

export interface DeployDetailPageProps {
  serviceId: string;
  deployId: string;
  /** The `?r=` relative log window (w9/003); undefined = the deploy's own. */
  range?: LogRange;
  /** Writes the range back to the URL (undefined clears `?r=`). */
  onRangeChange?: (range: LogRange | undefined) => void;
  /**
   * The deploy's `/rollback` route (w1/m646): Render's Confirm Rollback
   * section stands in place of the log, and the header offers no Rollback.
   */
  rollback?: boolean;
}

/**
 * The per-deploy page (w9/m1/t003): bex's twin of Render's
 * `/web/srv-…/deploys/dep-…` — a status header above a log viewer scoped to
 * the deploy's own time window, or to the URL's `?r=<range>` relative window
 * when one is set (w9/003). Rendered inside the `services.$serviceId` layout
 * route's `<Outlet/>`, so the service chrome (header + tab nav) is already in
 * place; this owns only the deploy-specific content.
 * It sits behind the role's service-activity gate (w9/m621).
 */
export function DeployDetailPage(props: DeployDetailPageProps) {
  return (
    <ActivityGate>
      <DeployDetail {...props} />
    </ActivityGate>
  );
}

function DeployDetail({
  serviceId,
  deployId,
  range,
  onRangeChange,
  rollback = false,
}: DeployDetailPageProps) {
  const { t } = useTranslations();
  const base = useServiceBase();
  const { deploy, error, notFound, refetch } = useDeploy(serviceId, deployId);
  // The service chrome above already reads it; this shares the cached read.
  const { service, ownerId } = useServer(serviceId, { poll: false });
  const { source: buildSource } = useBuildSource(
    service?.buildSourceId ?? null,
  );
  const scaling = useDeployScaling(serviceId, service, deploy);

  // A dead deploy id under a live service redirects to that service's Deploys
  // tab — the nearest live parent — not all the way home (w9/m55). A dead
  // SERVICE id never reaches here: the services.$serviceId layout redirects
  // before mounting its Outlet. `useDeploy` already excludes query errors
  // from `notFound`, so outages keep the inline error state below.
  const deploysTab = useMemo(
    () => ({
      to: `${base}/$serviceId/deploys` as const,
      params: { serviceId },
      replace: true,
    }),
    [serviceId, base],
  );
  useNotFoundRedirect(notFound, deploysTab);

  // The way back to the list this deploy was opened from — on a phone the
  // sidebar that names it is a drawer away.
  const backLink = (
    <Link
      to={deploysTab.to}
      params={deploysTab.params}
      className="relative inline-flex h-5 items-center gap-1 text-xs text-muted-foreground hover:text-foreground max-lg:after:absolute max-lg:after:-inset-x-2 max-lg:after:-inset-y-3 max-lg:after:content-['']"
    >
      <ChevronLeft className="size-3.5" aria-hidden />
      {t("services.navDeploys")}
    </Link>
  );

  // A Live deploy keeps refreshing (w4/262), so a failed refresh must not
  // unmount the detail it already shows: only a first-load failure is
  // whole-page; afterwards the last good detail stays with an inline retry.
  // An expired session gets the detail pages' Sign in (w3/m80 t002), never
  // an outage's Try again.
  if (error && !deploy && isUnauthenticatedError(error)) {
    return (
      <div className={DEPLOY_PAGE_TOP_CLASS}>
        {backLink}
        <ResourceLoadError variant="unauthenticated" />
      </div>
    );
  }
  if (error && !deploy) {
    // Under the plain explanation goes the server's own words, for support,
    // when it gave any; a read that never got an answer has none worth
    // showing (w5/167).
    const errorDetail = relayableReason(error);
    return (
      <div className={DEPLOY_PAGE_TOP_CLASS}>
        {backLink}
        <EmptyState
          role="alert"
          iconName="AlertCircle"
          title={t("deploys.loadErrorTitle")}
          description={
            <>
              {t("deploys.loadErrorBody")}
              {errorDetail ? (
                <span className="mt-1 block font-mono text-xs">
                  {errorDetail}
                </span>
              ) : null}
            </>
          }
          action={
            <Button variant="outline" size="sm" onClick={refetch}>
              {t("common.tryAgain")}
            </Button>
          }
        />
      </div>
    );
  }

  // Each region renders (and skeletons) on its own instead of the whole page
  // blank-waiting on the header `deploy` query (w9/m62 t002): the header,
  // timeline, and log panel mount in parallel; the timeline/log queries stay
  // skipped until the deploy's window arrives (a `?r=` range lets the log panel
  // query immediately), so nothing serializes behind the header round trip.
  return (
    <div className="space-y-6">
      {error ? (
        <div
          role="alert"
          className="flex flex-wrap items-center gap-2 rounded-md border border-dashed px-3 py-2 text-sm text-muted-foreground"
        >
          <span>{t("deploys.detailRefreshError")}</span>
          <Button variant="outline" size="sm" onClick={refetch}>
            {t("common.tryAgain")}
          </Button>
        </div>
      ) : null}
      <div className={DEPLOY_PAGE_TOP_CLASS}>
        {backLink}
        <DeployHeader
          deploy={deploy}
          repo={
            service?.repo ||
            (buildSource?.kind === "git" ? buildSource.source : null)
          }
          buildSourceId={service?.buildSourceId}
          buildSourceName={buildSource?.name}
          scaling={scaling}
          actions={
            deploy && !rollback ? (
              <DeployActions
                serviceId={serviceId}
                deployId={deploy.id}
                status={deploy.status}
                commitId={deploy.commitId}
                trigger={deploy.trigger}
                buildAvailable={deploy.buildAvailable}
                offerRollback={offersDashboardRollback(service?.type)}
              />
            ) : undefined
          }
        />
      </div>
      <DeployNotices
        serviceId={serviceId}
        deploy={deploy}
        service={service}
        ownerId={ownerId}
      />
      {rollback ? (
        <DeployRollback
          serviceId={serviceId}
          deployId={deployId}
          status={deploy?.status}
          buildAvailable={deploy?.buildAvailable}
          autoDeployOn={service?.autoDeploy === true}
        />
      ) : (
        <DeployBody
          serviceId={serviceId}
          deployId={deployId}
          deploy={deploy}
          range={range}
          onRangeChange={onRangeChange}
        />
      )}
    </div>
  );
}

interface DeployBodyProps {
  serviceId: string;
  deployId: string;
  deploy: DeployView | null;
  range?: LogRange;
  onRangeChange?: (range: LogRange | undefined) => void;
}

/** The deploy's log beside its status timeline, the page's body off its
 *  `/rollback` route. */
function DeployBody({
  serviceId,
  deployId,
  deploy,
  range,
  onRangeChange,
}: DeployBodyProps) {
  // The log window: `?r=` overrides with [now - range, now) — open-ended so
  // the viewer keeps polling live, exactly like a still-running deploy.
  // Memoized on the range value so re-renders don't shift the window (and
  // re-fire the three log queries) every second.
  const window = useMemo(() => {
    if (range) return { start: rangeStart(range), end: undefined };
    return undefined;
  }, [range]);

  // A canceled deploy's continued pre-deploy step widens its own window to
  // the step's end fact; while that resolves the window stays unknown (the
  // panel's loading state), never the known-truncated cancellation bound.
  const stepEnd = useCanceledStepEnd(serviceId, deploy);
  const deployStart =
    stepEnd.status === "checking"
      ? undefined
      : (deploy?.createdAt ?? undefined);
  // A live deploy's primary URL block (w32/m567) and the Blueprint's
  // one-time initialDeployHook (w33/m435) come after the deploy is live, so
  // its log runs past `finishedAt`: the window stays open while the hook
  // runs and closes where the log ends.
  const deployEnd =
    stepEnd.status === "found" ? stepEnd.end : deployWindowEnd(deploy);

  const limitedWindow =
    stepEnd.status === "error"
      ? { onRetry: stepEnd.retry }
      : stepEnd.status === "missing"
        ? {}
        : undefined;

  // Render's deploy page goes from its facts straight to the log; the
  // step-by-step timeline is the supporting record, beside it on a wide
  // screen and under it otherwise.
  return (
    <div className={DEPLOY_BODY_CLASS}>
      {/* Keyed per deploy: the live build tail keeps its rows through a pause
          (w4/m192), so a same-service deploy switch must start a fresh reader
          rather than carry the previous build's streamed lines. A Viewer
          reads the deploy but not its logs (w9/145): the role's reason
          stands in for the panel, and neither its reads nor its build
          tail start. */}
      <LogsGate>
        <DeployLogPanel
          key={deployId}
          resource={serviceId}
          title={deployTitle(deploy?.commitMessage, deployId)}
          deployId={deployId}
          startTime={window ? window.start : deployStart}
          endTime={window ? window.end : deployEnd}
          // Render's deploy page passes the deploy's creation whatever the
          // range: its log lines' From start (w4/716).
          startedAt={deploy?.createdAt ?? undefined}
          hasPreDeploy={
            !!deploy?.preDeployStatus || !!deploy?.initialDeployHookStatus
          }
          followBuild={deploy?.status === "build_in_progress"}
          range={range}
          onRangeChange={onRangeChange}
          limitedWindow={limitedWindow}
        />
      </LogsGate>
      <DeployTimeline serviceId={serviceId} deploy={deploy} />
    </div>
  );
}
