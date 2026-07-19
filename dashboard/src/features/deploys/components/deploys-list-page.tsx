import { useMemo, useState } from "react";
import { Link } from "@tanstack/react-router";
import { Search } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import { EmptyState } from "@/common/components/empty-state";
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
import { useDeploys, type DeployRow } from "../hooks/use-deploys";
import {
  deployStatusVariant,
  deployStatusKey,
  deployTriggerKey,
  isCancelableDeployStatus,
  isTerminalDeployStatus,
  preDeployStatusKey,
} from "../lib/deploy-status";
import {
  deployMatchesSearch,
  formatDeployDuration,
  formatDeployTimestamp,
} from "../lib/deploy-presentation";
import { DeployActions } from "./deploy-actions";

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

function triggerLabel(d: DeployRow, t: Translate): string {
  if (d.rollbackOf)
    return t("deploys.triggerRollback", { deployId: d.rollbackOf });
  const key = deployTriggerKey(d.trigger);
  return key ? t(key as Parameters<Translate>[0]) : d.trigger;
}

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
  const { t } = useTranslations();
  const [status, setStatus] = useState("");
  const [search, setSearch] = useState("");
  const { deploys, loading, loadingMore, error, hasMore, loadMore } =
    useDeploys(serviceId, status ? [status] : []);
  const visibleDeploys = useMemo(
    () => deploys.filter((deploy) => deployMatchesSearch(deploy, search)),
    [deploys, search],
  );

  const countKey = hasMore
    ? visibleDeploys.length === 1
      ? "deploys.listCountLoadedOne"
      : "deploys.listCountLoadedMany"
    : visibleDeploys.length === 1
      ? "deploys.listCountOne"
      : "deploys.listCountMany";

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
            <TableHead className="hidden sm:table-cell">
              {t("deploys.columnTrigger")}
            </TableHead>
            <TableHead className="hidden sm:table-cell">
              {t("deploys.columnDuration")}
            </TableHead>
            <TableHead className="w-0 text-right">
              <span className="sr-only">{t("deploys.columnActions")}</span>
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {visibleDeploys.map((d) => {
            const preDeploy = preDeployStatusKey(d.preDeployStatus);
            const createdAt = formatDeployTimestamp(d.createdAt);
            const hasListAction =
              isCancelableDeployStatus(d.status) || d.status === "deactivated";
            return (
              <TableRow key={d.id}>
                <TableCell className="min-w-0 align-top">
                  <Link
                    to="/services/$serviceId/deploys/$deployId"
                    params={{ serviceId, deployId: d.id }}
                    className="group block rounded-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
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
                    {d.commitId ? (
                      <p className="mt-1 max-w-[16rem] truncate text-sm text-foreground sm:max-w-md lg:max-w-lg">
                        <span
                          className="font-mono text-xs text-muted-foreground"
                          title={d.commitId}
                        >
                          {d.commitId.slice(0, 7)}
                        </span>
                        {d.commitMessage ? (
                          <> {d.commitMessage.split("\n")[0]}</>
                        ) : null}
                      </p>
                    ) : null}
                    <div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground">
                      {createdAt ? (
                        <span>
                          {t("deploys.deployedAt", { timestamp: createdAt })}
                        </span>
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
                  </Link>
                  {/* On mobile the Trigger/Duration columns are hidden, so fold
                      their values under the deploy identity instead. */}
                  <div className="mt-2 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground sm:hidden">
                    <span className="capitalize">{triggerLabel(d, t)}</span>
                    <span aria-hidden="true">·</span>
                    <span className="tabular-nums">{durationLabel(d, t)}</span>
                  </div>
                </TableCell>
                <TableCell className="hidden align-top text-sm capitalize text-muted-foreground sm:table-cell">
                  {triggerLabel(d, t)}
                </TableCell>
                <TableCell className="hidden align-top tabular-nums text-sm text-muted-foreground sm:table-cell">
                  {durationLabel(d, t)}
                </TableCell>
                <TableCell className="align-top text-right">
                  {hasListAction ? (
                    <div className="flex justify-end">
                      <DeployActions
                        serviceId={serviceId}
                        deployId={d.id}
                        status={d.status}
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
    <Card>
      <CardHeader className="space-y-3">
        <div className="flex flex-wrap items-baseline justify-between gap-2">
          <CardTitle>{t("deploys.listTitle")}</CardTitle>
          {!loading ? (
            <p className="text-xs text-muted-foreground" aria-live="polite">
              {t(countKey as Parameters<typeof t>[0], {
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
