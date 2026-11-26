import { SeparatedMeta } from "@/common/components/separated-meta";
import { STRETCHED_LINK_CLASS } from "@/common/lib/stretched-link";
import { useMemo, useState } from "react";
import { Link } from "@tanstack/react-router";
import { AlertCircle, Search } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
import { useIsHydrated } from "@/common/hooks/use-is-hydrated";
import { useNow } from "@/common/hooks/use-now";
import { useUrlEcho } from "@/common/hooks/use-url-echo";
import { formatTimeAgo } from "@/common/lib/format";
import {
  DeploysCountSlot,
  DeploysListFrame,
  DeploysTableFrame,
  DeploysTableHeader,
  deployRowGrid,
} from "./deploys-list-frame";
import { cn } from "@/common/lib/utils/utils";
import { InstantTooltip } from "@/common/components/instant-tooltip";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableRow,
} from "@/common/components/ui/table";
import { useServer } from "@/features/services/hooks/use-server";
import { useDeployActions } from "@/features/capabilities/hooks/use-resource-actions";
import { repoCommitUrl } from "@/features/services/lib/repo";
import { DeployStatusIcon } from "@/features/deploys/components/deploy-status-icon";
import { CommitSha } from "@/features/deploys/components/commit-sha";
import { useDeploys, type DeployRow } from "../hooks/use-deploys";
import {
  DEPLOY_STATUS_FILTERS,
  NO_DEPLOY_LIST_FILTERS,
  type DeployListFilters,
} from "../lib/deploy-list-search";
import {
  deployStatusVariant,
  deployStatusKey,
  deployTriggerLabel,
  isCancelableDeployStatus,
  isTerminalDeployStatus,
  preDeployStatusKey,
} from "../lib/deploy-status";
import {
  deployDurationValue,
  deployMatchesSearch,
  deployRowTimestamp,
  deploySubject,
} from "../lib/deploy-presentation";
import { DeployActions } from "./deploy-actions";
import { ManualBuildButton } from "./manual-build-button";
import { isCron } from "@/features/services/lib/service-type";
import { DeployFailureReason } from "./deploy-failure-reason";

// Radix Select can't hold "" — the log-filter-bar's "all" sentinel idiom.
const ALL = "all";

export interface DeploysListPageProps {
  serviceId: string;
  /**
   * The filters the URL names (w4/284). The page starts from them and adopts
   * them again when a navigation it did not write changes them; without a
   * host the filters are local.
   */
  urlFilters?: DeployListFilters;
  /** Fired with the whole filter state on each change, for the URL write. */
  onFiltersChange?: (filters: DeployListFilters) => void;
}

type Translate = ReturnType<typeof useTranslations>["t"];

function filtersKey(filters: DeployListFilters) {
  return JSON.stringify([filters.status, filters.search]);
}

// The Duration column value, or an em-dash for a deploy that never started
// (created/queued). The column header supplies the "Duration" label, so the
// cell shows the bare value.
function durationLabel(d: DeployRow, t: Translate): string {
  return deployDurationValue(d, t) || t("deploys.notYet");
}

/**
 * Below the full table a row's Cancel or Rollback is placed over the row at
 * touch height (44 px), centred on the row's last line, so it also reaches the
 * lower half of the line above (w3/111). Every line it can reach leaves the
 * action's width free: a line that can be the row's last (the trigger line, a
 * reason); with no reason, the meta line above the trigger line on a phone,
 * and the headline above the one-line meta once the list has room for that
 * but not yet for the table's columns. The meta line takes a margin rather
 * than padding, so an item too long for the line is cut at the action's edge
 * instead of running under it.
 */
const ACTION_CLEARANCE = {
  last: "@max-3xl/deploys:pr-28",
  meta: "@max-2xl/deploys:mr-28",
  headline: "@2xl/deploys:@max-3xl/deploys:pr-28",
} as const;

/**
 * The dedicated Deploys tab (w9/002): Render's standalone deploy-history
 * list — every deploy, filterable by status and paged with the keyset cursor
 * `deploys(serviceId, …)` has carried since w2/m31 — separate from the Events
 * feed that interleaves deploys with other activity. Rows link to the same
 * per-deploy page (w9/m1) the Events rows do; status badges come from the
 * shared deploy-status mapping so the three surfaces can't drift.
 */
export function DeploysListPage({
  serviceId,
  urlFilters = NO_DEPLOY_LIST_FILTERS,
  onFiltersChange,
}: DeploysListPageProps) {
  const { t, i18n } = useTranslations();
  const hydrated = useIsHydrated();
  // Row times are elapsed ("Deployed 2 hours ago", Render's deploy list)
  // rather than absolute: timezone-neutral, so they render on the SSR pass
  // too, and one page-level minute ticker keeps every row's text moving. The
  // exact instant lives in each row's hover tooltip (InstantTooltip), which is
  // where the viewer-local reading is deferred to (w6/m107).
  const now = useNow();
  const justNow = t("common.justNow");
  // The service's repo turns each row's commit SHA into a link to its diff —
  // a secondary, non-polling read of the document the layout already owns.
  const { service } = useServer(serviceId, { poll: false });
  const repo = service?.repo ?? null;
  // A repo-backed cron job builds from here, as on Render; an image-backed
  // one has nothing to build (w3/m555).
  const manualBuild = Boolean(service && isCron(service) && repo);
  // The inputs stay local so typing never waits on the router; the URL
  // mirrors them, which is what brings them back after a deploy's detail.
  const [localFilters, setLocalFilters] = useState(urlFilters);
  const urlEcho = useUrlEcho(filtersKey(urlFilters));
  let filters = localFilters;
  if (urlEcho.foreign) {
    filters = urlFilters;
    setLocalFilters(filters);
  }
  urlEcho.showing(filtersKey(filters));
  const { status, search } = filters;
  const changeFilters = (patch: Partial<DeployListFilters>) => {
    const next = { ...filters, ...patch };
    setLocalFilters(next);
    onFiltersChange?.(next);
  };
  const { deploys, loading, loadingMore, error, hasMore, loadMore, retry } =
    useDeploys(serviceId, status ? [status] : []);
  const visibleDeploys = useMemo(
    () => deploys.filter((deploy) => deployMatchesSearch(deploy, search)),
    [deploys, search],
  );
  const projection = useDeployActions(
    !loading &&
      visibleDeploys.some(
        (deploy) =>
          isCancelableDeployStatus(deploy.status) ||
          deploy.status === "deactivated",
      )
      ? serviceId
      : null,
  );

  // Native plural keys (w6/062): `_one`/`_other` are resolved by i18next from
  // the numeric `count` param passed at the render site.
  const countKey = hasMore ? "deploys.listCountLoaded" : "deploys.listCount";

  const searching = search.trim() !== "";
  const failed = Boolean(error) && deploys.length === 0;

  const noMatch =
    !failed && !loading && deploys.length > 0 && visibleDeploys.length === 0;
  const loadMoreButton = (
    <Button
      variant="outline"
      size="sm"
      className="max-lg:h-11"
      disabled={loadingMore}
      onClick={loadMore}
    >
      {t("deploys.listLoadMore")}
    </Button>
  );

  let body;
  if (failed) {
    body = (
      <div
        role="alert"
        className="flex flex-col items-center gap-1 rounded-lg border border-dashed px-4 py-10 text-center"
      >
        <AlertCircle
          aria-hidden="true"
          className="mb-2 size-6 text-muted-foreground"
        />
        <p className="font-medium">{t("deploys.listLoadErrorTitle")}</p>
        <p className="max-w-sm text-sm text-balance text-muted-foreground">
          {mutationErrorMessage(error, t("deploys.listLoadErrorBody"))}
        </p>
        <Button
          variant="outline"
          size="sm"
          className="mt-3 max-lg:h-11"
          onClick={retry}
        >
          {t("common.tryAgain")}
        </Button>
      </div>
    );
  } else if (loading) {
    body = <DeploysTableFrame commits={!service || Boolean(repo)} />;
  } else if (deploys.length === 0) {
    body = (
      <p className="rounded-lg border border-dashed px-4 py-10 text-center text-sm text-muted-foreground">
        {status ? t("deploys.listEmptyFiltered") : t("deploys.listEmpty")}
      </p>
    );
  } else if (noMatch) {
    body = (
      <div className="flex flex-col items-center gap-3 rounded-lg border border-dashed px-4 py-10 text-center">
        <p className="text-sm text-muted-foreground">
          {t("deploys.listEmptySearch")}
        </p>
        <div className="flex flex-wrap justify-center gap-2">
          <Button
            variant="outline"
            size="sm"
            className="max-lg:h-11"
            onClick={() => changeFilters({ search: "" })}
          >
            {t("deploys.listClearSearch")}
          </Button>
          {hasMore ? loadMoreButton : null}
        </div>
      </div>
    );
  } else {
    body = (
      <Table role="table" className="block">
        <DeploysTableHeader />
        <TableBody role="rowgroup" className="block">
          {visibleDeploys.map((d) => {
            const preDeploy = preDeployStatusKey(d.preDeployStatus);
            // Verb + instant follow the deploy's terminal state (w6/051): a
            // canceled or failed row must not read "Deployed", and a shipped
            // row shows when it went live, not when its row was opened.
            const rowStamp = deployRowTimestamp(d);
            const ago = formatTimeAgo(rowStamp.iso, {
              now,
              language: i18n.language,
              justNow,
            });
            const commitUrl =
              repo && d.commitId ? repoCommitUrl(repo, d.commitId) : null;
            const triggerLabel = deployTriggerLabel(d.trigger, d.rollbackOf, t);
            // A previous deploy whose build the registry no longer holds
            // cannot be rolled back to, so it gets no action (w8/m156).
            const hasListAction =
              isCancelableDeployStatus(d.status) ||
              (d.status === "deactivated" && d.buildAvailable !== false);
            const isLive = d.status === "live";
            const statusLabel = t(
              deployStatusKey(d.status) as Parameters<typeof t>[0],
            );
            const subject = deploySubject(d.commitMessage);
            // The icon and the row's verb ("Deployed", "Failed", "Canceled")
            // say how a finished deploy ended. A deploy still under way has
            // no such verb, and a failure with no stored reason has nothing
            // else naming which step failed, so those rows spell the status —
            // unless the pre-deploy line already names it, where the spelled
            // status only repeated the verb ("Failed · Failed 4 months ago").
            const spellStatus =
              !isTerminalDeployStatus(d.status) ||
              (deployStatusVariant(d.status) === "destructive" &&
                !d.failureReason &&
                d.preDeployStatus !== "failed");
            const reasonClass = cn(
              "col-start-2 line-clamp-2 break-words text-pretty",
              hasListAction && ACTION_CLEARANCE.last,
            );
            // A reason is the row's last line when it has one, so the action
            // sits beside the reason and the lines above it stay as they are.
            const actionReachesAbove =
              hasListAction &&
              !d.failureReason &&
              !d.cancelReason &&
              !d.stallReason;
            return (
              <TableRow
                key={d.id}
                role="row"
                className={cn(
                  deployRowGrid,
                  "group/row relative active:bg-muted/60",
                )}
              >
                <TableCell
                  role="cell"
                  className="grid min-w-0 grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-1 px-2 py-3 whitespace-normal"
                >
                  <DeployStatusIcon
                    status={d.status}
                    label={statusLabel}
                    className="mt-px"
                  />
                  {/* Render's row: the commit subject is the headline and
                      opens the deploy. The link wraps only that line: the
                      commit SHA below is its own link (to the diff) and the
                      timestamp is a tooltip trigger, and neither may nest in
                      an anchor. Its ::after covers the row, so the whole row
                      opens the deploy; the other controls sit above it. */}
                  <Link
                    to="/services/$serviceId/deploys/$deployId"
                    params={{ serviceId, deployId: d.id }}
                    title={d.id}
                    className={cn(
                      "col-start-2 min-w-0 after:rounded-md",
                      STRETCHED_LINK_CLASS,
                      actionReachesAbove && ACTION_CLEARANCE.headline,
                    )}
                  >
                    {subject ? (
                      <>
                        <span className="line-clamp-2 text-sm font-medium break-words text-pretty text-foreground underline-offset-2 group-hover/row:underline @3xl/deploys:line-clamp-1">
                          {subject}
                        </span>
                        {/* The deploy's own id names the link for assistive
                            tech and stays searchable; the row shows the
                            commit, as Render's does. */}
                        <span className="sr-only">{d.id}</span>
                      </>
                    ) : (
                      // No commit subject (an image-backed deploy): the
                      // deploy's id is the only name it has.
                      <span className="block truncate font-mono text-sm text-foreground underline-offset-2 group-hover/row:underline">
                        {d.id}
                      </span>
                    )}
                  </Link>
                  {/* Meta line, then trigger and duration: stacked on a
                      phone, one line once the list has room for it, and the
                      table's own columns from there up. */}
                  <div className="col-start-2 flex min-w-0 flex-col gap-y-1 @2xl/deploys:flex-row @2xl/deploys:items-center @2xl/deploys:gap-x-1.5">
                    <SeparatedMeta
                      className={cn(
                        "text-xs text-muted-foreground @2xl/deploys:shrink-0",
                        actionReachesAbove && ACTION_CLEARANCE.meta,
                      )}
                    >
                      {/* Render's list links the short SHA to the commit's
                        diff on the forge; with no browsable repo (an
                        image-backed service) it stays plain text. */}
                      {d.commitId ? (
                        <CommitSha
                          commitId={d.commitId}
                          url={commitUrl}
                          // Above the row's stretched link, so it opens the
                          // diff rather than the deploy.
                          className={
                            commitUrl
                              ? "z-10 text-foreground/80 hover:text-foreground"
                              : undefined
                          }
                        />
                      ) : null}
                      {spellStatus ? (
                        <span
                          className={
                            deployStatusVariant(d.status) === "destructive"
                              ? "text-destructive"
                              : "font-medium text-foreground"
                          }
                        >
                          {statusLabel}
                        </span>
                      ) : null}
                      {rowStamp.iso && ago ? (
                        <InstantTooltip
                          value={rowStamp.iso}
                          // Hover and keyboard reveal the exact instant; a touch
                          // has neither, so a tap falls through to the row.
                          className="relative z-10 no-underline hover:underline pointer-coarse:pointer-events-none"
                        >
                          {t(rowStamp.key as Parameters<typeof t>[0], {
                            timestamp: ago,
                          })}
                        </InstantTooltip>
                      ) : null}
                      {preDeploy ? (
                        // The meta line's longest item in some languages
                        // ("Comando previo al despliegue en ejecución"): on
                        // a line of its own too narrow for it, it ends in an
                        // ellipsis.
                        <span
                          className={cn(
                            "min-w-0 truncate",
                            d.preDeployStatus === "failed" &&
                              "text-destructive",
                          )}
                        >
                          {t(preDeploy as Parameters<typeof t>[0])}
                        </span>
                      ) : null}
                    </SeparatedMeta>
                    {/* Until the list is wide enough for the full table,
                      Trigger/Duration fold into a line of their own, which
                      keeps clear of the row's action on a phone. */}
                    <div
                      className={cn(
                        "flex min-w-0 text-xs whitespace-nowrap text-muted-foreground @2xl/deploys:before:mr-1.5 @2xl/deploys:before:content-['·'] @3xl/deploys:hidden",
                        (hasListAction || isLive) && ACTION_CLEARANCE.last,
                      )}
                    >
                      <span className="truncate" title={triggerLabel}>
                        {triggerLabel}
                      </span>
                      {/* A deploy that never ran has no duration to fold in;
                        the table's Duration column still shows its dash.
                        Only the trigger gives way: a clipped duration reads
                        as a different number. */}
                      {d.startedAt ? (
                        <span className="shrink-0 tabular-nums before:mx-1.5 before:content-['·']">
                          {durationLabel(d, t)}
                        </span>
                      ) : null}
                    </div>
                  </div>
                  {/* Absent on non-failed / non-superseded rows, so their
                      height is unchanged. A reason wraps to two lines; the
                      deploy's own page has all of it. */}
                  <DeployFailureReason
                    reason={d.failureReason}
                    template={d.failureReasonTemplate}
                    className={reasonClass}
                  />
                  <DeployFailureReason
                    reason={d.cancelReason}
                    template={d.cancelReasonTemplate}
                    tone="neutral"
                    className={reasonClass}
                  />
                  {/* w4/m112: an open deploy's stall cause, cleared server-side
                      the moment it goes terminal — so a settled row is
                      unchanged. */}
                  <DeployFailureReason
                    reason={d.stallReason}
                    template={d.stallReasonTemplate}
                    tone="neutral"
                    className={reasonClass}
                  />
                </TableCell>
                <TableCell
                  role="cell"
                  title={triggerLabel}
                  className="hidden truncate px-2 text-sm text-muted-foreground @3xl/deploys:block"
                >
                  {triggerLabel}
                </TableCell>
                <TableCell
                  role="cell"
                  className="hidden px-2 text-sm whitespace-nowrap tabular-nums text-muted-foreground @3xl/deploys:block"
                >
                  {durationLabel(d, t)}
                </TableCell>
                {/* The row's right edge says what the deploy is to the
                    service now: the live one is marked, a previous one can
                    be rolled back to, an open one canceled. Below the full
                    table it is placed over the row at touch height, centred
                    on its last line; the lines it reaches keep clear of it
                    (ACTION_CLEARANCE). */}
                <TableCell
                  role="cell"
                  className="absolute right-0 -bottom-0.5 z-10 flex h-11 items-center p-0 px-2 empty:hidden [&_button]:min-h-11 [&_button:enabled]:text-foreground/80 [&_button:enabled:hover]:text-foreground @3xl/deploys:static @3xl/deploys:h-auto @3xl/deploys:justify-self-end @3xl/deploys:empty:block @3xl/deploys:[&_button]:min-h-8"
                >
                  {hasListAction ? (
                    <DeployActions
                      projection={projection}
                      serviceId={serviceId}
                      deployId={d.id}
                      status={d.status}
                      commitId={d.commitId}
                      image={d.image}
                      commitMessage={d.commitMessage}
                      trigger={d.trigger}
                      buildAvailable={d.buildAvailable}
                      autoDeployOn={
                        service ? service.autoDeploy === true : undefined
                      }
                    />
                  ) : isLive ? (
                    <Badge className="pointer-events-none mr-2">
                      {statusLabel}
                    </Badge>
                  ) : null}
                </TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
    );
  }

  let count: string | null = null;
  if (!loading && !failed) {
    count = searching
      ? t("deploys.listMatchCount", {
          matched: visibleDeploys.length,
          total: deploys.length,
        })
      : t(countKey, { count: visibleDeploys.length });
  }

  return (
    <DeploysListFrame
      action={
        manualBuild ? <ManualBuildButton serviceId={serviceId} /> : undefined
      }
      count={
        count !== null ? (
          <p
            className="text-xs tabular-nums text-muted-foreground"
            aria-live="polite"
          >
            {count}
          </p>
        ) : loading ? (
          <DeploysCountSlot />
        ) : null
      }
      controls={
        <>
          <div className="relative min-w-0 flex-1">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={search}
              // Uneditable until handlers attach: a query typed into the
              // server HTML is wiped by hydration (w4/m219).
              disabled={!hydrated || failed}
              onChange={(event) =>
                changeFilters({ search: event.target.value })
              }
              placeholder={t("deploys.listSearchPlaceholder")}
              aria-label={t("deploys.listSearchLabel")}
              className="truncate pl-9 max-lg:h-11 pointer-coarse:h-11"
            />
          </div>
          <Select
            disabled={failed}
            value={status === "" ? ALL : status}
            onValueChange={(v) => changeFilters({ status: v === ALL ? "" : v })}
          >
            <SelectTrigger
              size="sm"
              className="w-32 shrink-0 max-lg:data-[size=sm]:h-11 pointer-coarse:data-[size=sm]:h-11 @2xl/deploys:w-44"
              aria-label={t("deploys.listStatusFilterLabel")}
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent
              // Always under its trigger: flipping upward on a phone covered
              // the service header and the top bar.
              side="bottom"
              align="end"
              avoidCollisions={false}
            >
              <SelectItem value={ALL} className="max-lg:min-h-11">
                {t("deploys.listStatusAll")}
              </SelectItem>
              {DEPLOY_STATUS_FILTERS.map((s) => (
                <SelectItem key={s} value={s} className="max-lg:min-h-11">
                  {t(deployStatusKey(s) as Parameters<typeof t>[0])}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </>
      }
    >
      {body}
      {hasMore && deploys.length > 0 && !noMatch ? (
        <div className="mt-4 flex justify-center">{loadMoreButton}</div>
      ) : null}
    </DeploysListFrame>
  );
}
