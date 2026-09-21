import { useMemo, useState } from "react";
import { Link } from "@tanstack/react-router";
import { Search } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import { useNow } from "@/common/hooks/use-now";
import { formatTimeAgo } from "@/common/lib/format";
import { EmptyState } from "@/common/components/empty-state";
import { InstantTooltip } from "@/common/components/instant-tooltip";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import { Input } from "@/common/components/ui/input";
import { Skeleton } from "@/common/components/ui/skeleton";
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
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table";
import { useServer } from "@/features/services/hooks/use-server";
import { repoCommitUrl } from "@/features/services/lib/repo";
import { useDeploys, type DeployRow } from "../hooks/use-deploys";
import {
  deployStatusVariant,
  deployStatusKey,
  deployTriggerLabel,
  isCancelableDeployStatus,
  isTerminalDeployStatus,
  preDeployStatusKey,
} from "../lib/deploy-status";
import {
  deployMatchesSearch,
  deployRowTimestamp,
  formatDeployDuration,
} from "../lib/deploy-presentation";
import { DeployActions } from "./deploy-actions";
import { DeployFailureReason } from "./deploy-failure-reason";

// Radix Select can't hold "" — the log-filter-bar's "all" sentinel idiom.
const ALL = "all";

// The status filter's vocabulary: the deploy-status enum the store writes
// (store.Deploy*), labeled by the same keys the badges use.
const STATUS_OPTIONS = [
  "created",
  "queued",
  "build_in_progress",
  "build_failed",
  "pre_deploy_in_progress",
  "pre_deploy_failed",
  "update_in_progress",
  "update_failed",
  "live",
  "deactivated",
  "canceled",
] as const;

export interface DeploysListPageProps {
  serviceId: string;
}

type Translate = ReturnType<typeof useTranslations>["t"];

// The Duration column value: the settled elapsed time once a deploy finishes,
// a running-elapsed marker while an active deploy is still building/deploying,
// and an em-dash for a deploy that never started (created/queued). The column
// header supplies the "Duration" label, so the cell shows the bare value —
// unlike the detail header's `deploys.durationValue` ("Duration {duration}").
function durationLabel(d: DeployRow, t: Translate): string {
  const duration = formatDeployDuration(d.startedAt, d.finishedAt);
  if (duration) return duration;
  if (d.startedAt && !isTerminalDeployStatus(d.status))
    return t("deploys.durationActive");
  return t("deploys.notYet");
}

/**
 * The dedicated Deploys tab (w9/002): Render's standalone deploy-history
 * list — every deploy, filterable by status and paged with the keyset cursor
 * `deploys(serviceId, …)` has carried since w2/m31 — separate from the Events
 * feed that interleaves deploys with other activity. Rows link to the same
 * per-deploy page (w9/m1) the Events rows do; status badges come from the
 * shared deploy-status mapping so the three surfaces can't drift.
 */
export function DeploysListPage({ serviceId }: DeploysListPageProps) {
  const { t, i18n } = useTranslations();
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
  const [status, setStatus] = useState("");
  const [search, setSearch] = useState("");
  const { deploys, loading, loadingMore, error, hasMore, loadMore } =
    useDeploys(serviceId, status ? [status] : []);
  const visibleDeploys = useMemo(
    () => deploys.filter((deploy) => deployMatchesSearch(deploy, search)),
    [deploys, search],
  );

  // Native plural keys (w6/062): `_one`/`_other` are resolved by i18next from
  // the numeric `count` param passed at the render site.
  const countKey = hasMore ? "deploys.listCountLoaded" : "deploys.listCount";

  let body;
  if (error && deploys.length === 0) {
    body = (
      <EmptyState
        iconName="AlertCircle"
        title={t("deploys.listTitle")}
        description={error.message}
      />
    );
  } else if (loading) {
    body = (
      <div className="space-y-3">
        {[0, 1, 2].map((i) => (
          <Skeleton key={i} className="h-14 w-full" />
        ))}
      </div>
    );
  } else if (deploys.length === 0) {
    body = (
      <p className="text-sm text-muted-foreground">
        {status ? t("deploys.listEmptyFiltered") : t("deploys.listEmpty")}
      </p>
    );
  } else if (visibleDeploys.length === 0) {
    body = (
      <p className="text-sm text-muted-foreground">
        {t("deploys.listEmptySearch")}
      </p>
    );
  } else {
    body = (
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t("deploys.columnDeploy")}</TableHead>
            <TableHead className="hidden whitespace-nowrap @3xl/deploys:table-cell">
              {t("deploys.columnTrigger")}
            </TableHead>
            <TableHead className="hidden whitespace-nowrap @3xl/deploys:table-cell">
              {t("deploys.columnDuration")}
            </TableHead>
            <TableHead className="w-0 whitespace-nowrap text-right">
              <span className="sr-only">{t("deploys.columnActions")}</span>
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
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
            const hasListAction =
              isCancelableDeployStatus(d.status) || d.status === "deactivated";
            return (
              <TableRow key={d.id}>
                <TableCell className="min-w-0 align-top">
                  {/* The detail link wraps only the identity line: the commit
                      SHA below is its own link (to the diff) and the timestamp
                      is a tooltip trigger, and neither may nest in an anchor. */}
                  <Link
                    to="/services/$serviceId/deploys/$deployId"
                    params={{ serviceId, deployId: d.id }}
                    className="inline-block max-w-full rounded-md focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
                  >
                    <div className="flex min-w-0 flex-wrap items-center gap-2">
                      <Badge variant={deployStatusVariant(d.status)}>
                        {t(
                          deployStatusKey(d.status) as Parameters<typeof t>[0],
                        )}
                      </Badge>
                      <span
                        className="max-w-[12rem] truncate font-mono text-xs text-muted-foreground"
                        title={d.id}
                      >
                        {d.id}
                      </span>
                    </div>
                  </Link>
                  {/* Absent on non-failed / non-superseded rows, so their
                      height is unchanged. */}
                  <DeployFailureReason
                    reason={d.failureReason}
                    truncate
                    className="mt-1 max-w-[16rem] sm:max-w-md lg:max-w-lg"
                  />
                  <DeployFailureReason
                    reason={d.cancelReason}
                    tone="neutral"
                    truncate
                    className="mt-1 max-w-[16rem] sm:max-w-md lg:max-w-lg"
                  />
                  {/* w4/m112: an open deploy's stall cause, cleared server-side
                      the moment it goes terminal — so a settled row is
                      unchanged. */}
                  <DeployFailureReason
                    reason={d.stallReason}
                    tone="neutral"
                    truncate
                    className="mt-1 max-w-[16rem] sm:max-w-md lg:max-w-lg"
                  />
                  {d.commitId ? (
                    <p className="mt-1 max-w-[16rem] truncate text-sm text-foreground sm:max-w-md lg:max-w-lg">
                      {/* Render's list links the short SHA to the commit's
                          diff on the forge; with no browsable repo (an
                          image-backed service) it stays plain text. */}
                      {commitUrl ? (
                        <a
                          href={commitUrl}
                          target="_blank"
                          rel="noreferrer"
                          className="font-mono text-xs text-muted-foreground underline underline-offset-2 hover:text-foreground"
                          title={d.commitId}
                        >
                          {d.commitId.slice(0, 7)}
                        </a>
                      ) : (
                        <span
                          className="font-mono text-xs text-muted-foreground"
                          title={d.commitId}
                        >
                          {d.commitId.slice(0, 7)}
                        </span>
                      )}
                      {d.commitMessage ? (
                        <> {d.commitMessage.split("\n")[0]}</>
                      ) : null}
                    </p>
                  ) : null}
                  <div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground">
                    {rowStamp.iso && ago ? (
                      <InstantTooltip value={rowStamp.iso}>
                        {t(rowStamp.key as Parameters<typeof t>[0], {
                          timestamp: ago,
                        })}
                      </InstantTooltip>
                    ) : null}
                    {preDeploy ? (
                      <span
                        className={
                          d.preDeployStatus === "failed"
                            ? "text-destructive"
                            : undefined
                        }
                      >
                        {t(preDeploy as Parameters<typeof t>[0])}
                      </span>
                    ) : null}
                  </div>
                  {/* Until the card is wide enough for the full table, fold
                      Trigger/Duration under the deploy identity instead. */}
                  <div className="mt-2 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground @3xl/deploys:hidden">
                    <span>{deployTriggerLabel(d.trigger, d.rollbackOf, t)}</span>
                    <span aria-hidden="true">·</span>
                    <span className="tabular-nums">{durationLabel(d, t)}</span>
                  </div>
                </TableCell>
                <TableCell className="hidden whitespace-nowrap align-top text-sm text-muted-foreground @3xl/deploys:table-cell">
                  {deployTriggerLabel(d.trigger, d.rollbackOf, t)}
                </TableCell>
                <TableCell className="hidden whitespace-nowrap align-top tabular-nums text-sm text-muted-foreground @3xl/deploys:table-cell">
                  {durationLabel(d, t)}
                </TableCell>
                <TableCell className="w-0 whitespace-nowrap align-top text-right">
                  {hasListAction ? (
                    <div className="flex justify-end">
                      <DeployActions
                        serviceId={serviceId}
                        deployId={d.id}
                        status={d.status}
                        commitId={d.commitId}
                        commitMessage={d.commitMessage}
                        trigger={d.trigger}
                      />
                    </div>
                  ) : null}
                </TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
    );
  }

  return (
    <Card className="@container/deploys">
      <CardHeader className="space-y-3">
        <div className="flex flex-wrap items-baseline justify-between gap-2">
          <CardTitle>{t("deploys.listTitle")}</CardTitle>
          {!loading ? (
            <p className="text-xs text-muted-foreground" aria-live="polite">
              {t(countKey, {
                count: visibleDeploys.length,
              })}
            </p>
          ) : null}
        </div>
        <div className="flex flex-col gap-2 sm:flex-row">
          <div className="relative min-w-0 flex-1">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder={t("deploys.listSearchPlaceholder")}
              aria-label={t("deploys.listSearchLabel")}
              className="pl-9"
            />
          </div>
          <Select
            value={status === "" ? ALL : status}
            onValueChange={(v) => setStatus(v === ALL ? "" : v)}
          >
            <SelectTrigger
              size="sm"
              className="w-full sm:w-44"
              aria-label={t("deploys.listStatusFilterLabel")}
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={ALL}>{t("deploys.listStatusAll")}</SelectItem>
              {STATUS_OPTIONS.map((s) => (
                <SelectItem key={s} value={s}>
                  {t(deployStatusKey(s) as Parameters<typeof t>[0])}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </CardHeader>
      <CardContent>
        {body}
        {hasMore && deploys.length > 0 && (
          <div className="mt-4 flex justify-center">
            <Button
              variant="outline"
              size="sm"
              disabled={loadingMore}
              onClick={loadMore}
            >
              {t("deploys.listLoadMore")}
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
