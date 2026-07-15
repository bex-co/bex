import { useState } from "react";
import { Link } from "@tanstack/react-router";
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
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { useDeploys, type DeployRow } from "../hooks/use-deploys";
import {
  deployStatusVariant,
  deployStatusKey,
  deployTriggerKey,
  preDeployStatusKey,
} from "../lib/deploy-status";

// Radix Select can't hold "" — the log-filter-bar's "all" sentinel idiom.
const ALL = "all";

// The status filter's vocabulary: the deploy-status enum the store writes
// (store.Deploy*), labeled by the same keys the badges use.
const STATUS_OPTIONS = [
  "live",
  "update_in_progress",
  "update_failed",
  "canceled",
] as const;

export interface DeploysListPageProps {
  serviceId: string;
}

type Translate = ReturnType<typeof useTranslations>["t"];

function triggerLabel(d: DeployRow, t: Translate): string {
  if (d.rollbackOf) return t("deploys.triggerRollback", { deployId: d.rollbackOf });
  const key = deployTriggerKey(d.trigger);
  return key ? t(key as Parameters<Translate>[0]) : d.trigger;
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
  const { deploys, loading, loadingMore, error, hasMore, loadMore } =
    useDeploys(serviceId, status ? [status] : []);

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
  } else {
    body = (
      <div className="divide-y">
        {deploys.map((d) => {
          const preDeploy = preDeployStatusKey(d.preDeployStatus);
          return (
            <Link
              key={d.id}
              to="/services/$serviceId/deploys/$deployId"
              params={{ serviceId, deployId: d.id }}
              className="block py-3"
            >
              <div className="flex items-center gap-2">
                <Badge variant={deployStatusVariant(d.status)}>
                  {t(deployStatusKey(d.status) as Parameters<typeof t>[0])}
                </Badge>
                <span className="text-xs capitalize text-muted-foreground">
                  {triggerLabel(d, t)}
                </span>
                <span className="font-mono text-xs text-muted-foreground">
                  {d.id}
                </span>
              </div>
              {d.commitId && (
                <p className="mt-1 truncate text-xs text-muted-foreground">
                  <span className="font-mono">{d.commitId.slice(0, 7)}</span>
                  {d.commitMessage && <> {d.commitMessage.split("\n")[0]}</>}
                </p>
              )}
              {preDeploy && (
                <p
                  className={`mt-1 text-xs ${
                    d.preDeployStatus === "failed"
                      ? "text-destructive"
                      : "text-muted-foreground"
                  }`}
                >
                  {t(preDeploy as Parameters<typeof t>[0])}
                </p>
              )}
              {d.createdAt && (
                <p className="mt-1 text-xs text-muted-foreground">
                  {new Date(d.createdAt).toLocaleString()}
                </p>
              )}
            </Link>
          );
        })}
      </div>
    );
  }

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle>{t("deploys.listTitle")}</CardTitle>
        <Select
          value={status === "" ? ALL : status}
          onValueChange={(v) => setStatus(v === ALL ? "" : v)}
        >
          <SelectTrigger
            size="sm"
            className="w-44"
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
