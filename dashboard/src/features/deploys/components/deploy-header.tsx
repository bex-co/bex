import { Card, CardContent } from "@/common/components/ui/card";
import { Badge } from "@/common/components/ui/badge";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import type { ReactNode } from "react";
type Translate = ReturnType<typeof useTranslations>["t"];
import {
  deployStatusVariant,
  deployStatusKey,
  deployTriggerKey,
  preDeployStatusKey,
} from "@/features/deploys/lib/deploy-status";
import {
  formatDeployDuration,
  formatDeployTimestamp,
} from "@/features/deploys/lib/deploy-presentation";
import type { DeployView } from "../hooks/use-deploy";

function triggerLabel(
  trigger: string,
  rollbackOf: string,
  t: Translate,
): string {
  if (rollbackOf) return t("deploys.triggerRollback", { deployId: rollbackOf });
  const key = deployTriggerKey(trigger);
  return key ? t(key as Parameters<Translate>[0]) : trigger;
}

export interface DeployHeaderProps {
  /** Nullish while the header `deploy` query is still in flight (w9/m62 t002). */
  deploy?: DeployView | null;
  actions?: ReactNode;
}

/**
 * The deploy detail page's header (w9/m1/t003): Render's compact deploy-page
 * banner — status badge, trigger/rollback provenance, image, pre-deploy
 * outcome, and the created/updated/started/finished timestamps. Reuses the same
 * status→badge mapping as the Events tab (deploy-status.ts) so the two
 * surfaces can't drift on what a given status looks like.
 */
export function DeployHeader({ deploy, actions }: DeployHeaderProps) {
  const { t } = useTranslations();
  if (!deploy) {
    return (
      <Card>
        <CardContent className="flex items-start justify-between gap-4 pt-6">
          <div
            className="space-y-2"
            role="status"
            aria-label={t("common.loading")}
          >
            <Skeleton className="h-6 w-64" />
            <Skeleton className="h-4 w-40" />
          </div>
          {actions}
        </CardContent>
      </Card>
    );
  }
  const preDeploy = preDeployStatusKey(deploy.preDeployStatus);
  const commitCreatedAt = formatDeployTimestamp(deploy.commitCreatedAt);
  const facts = [
    {
      label: t("deploys.created"),
      value: formatDeployTimestamp(deploy.createdAt),
    },
    {
      label: t("deploys.updated"),
      value: formatDeployTimestamp(deploy.updatedAt),
    },
    {
      label: t("deploys.started"),
      value: formatDeployTimestamp(deploy.startedAt),
    },
    {
      label: t("deploys.finished"),
      value: formatDeployTimestamp(deploy.finishedAt),
    },
    {
      label: t("deploys.duration"),
      value: formatDeployDuration(deploy.startedAt, deploy.finishedAt),
    },
  ].filter((fact): fact is { label: string; value: string } => !!fact.value);

  return (
    <Card>
      <CardContent className="space-y-3 py-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-xs font-medium text-muted-foreground">
              {t("deploys.statusLabel")}
            </span>
            <Badge variant={deployStatusVariant(deploy.status)}>
              {t(deployStatusKey(deploy.status) as Parameters<typeof t>[0])}
            </Badge>
            <span className="text-xs capitalize text-muted-foreground">
              {triggerLabel(deploy.trigger, deploy.rollbackOf, t)}
            </span>
            <span className="font-mono text-xs text-muted-foreground">
              {deploy.id}
            </span>
          </div>
          {actions}
        </div>

        {/* Actionable cause of a failed deploy (w9/011) — the operator's
            diagnosis (crash loop with the $PORT hint, image-pull failure,
            build error) or a health-gate-timeout line. */}
        {deploy.failureReason && (
          <p className="text-xs text-destructive">{deploy.failureReason}</p>
        )}

        {preDeploy && (
          <p
            className={`text-xs ${
              deploy.preDeployStatus === "failed"
                ? "text-destructive"
                : "text-muted-foreground"
            }`}
          >
            {t(preDeploy as Parameters<typeof t>[0])}
          </p>
        )}

        {/* The resolved commit this deploy ran (w9/001 + w2/m42) — Render's
            deploy-page header leads with it for repo-backed deploys: short
            SHA + the message's first line + author date when available. Absent
            (image-backed, or no GitHub connection to resolve through) =>
            omitted, not faked. */}
        {deploy.commitId && (
          <p className="truncate text-xs text-foreground">
            <span className="font-mono text-muted-foreground">
              {deploy.commitId.slice(0, 7)}
            </span>
            {deploy.commitMessage && (
              <> {deploy.commitMessage.split("\n")[0]}</>
            )}
            {commitCreatedAt && (
              // The separator is a real text node, not just the margin it used
              // to be (w6/m45 t004): a margin leaves the paragraph one
              // unbroken text run, so the commit message and the date ran
              // together — "…Docker build contextAugust 22, 2026 at 6:16 PM" —
              // for a screen reader and for anyone copying the line.
              <span className="text-muted-foreground"> · {commitCreatedAt}</span>
            )}
          </p>
        )}

        {deploy.image ? (
          <p className="truncate font-mono text-xs text-muted-foreground">
            {deploy.image}
          </p>
        ) : null}

        {facts.length > 0 ? (
          <dl className="grid grid-cols-1 gap-x-6 gap-y-1 text-xs text-muted-foreground sm:grid-cols-2 lg:grid-cols-5">
            {facts.map((fact) => (
              <div key={fact.label}>
                <dt className="inline font-medium">{fact.label}: </dt>
                <dd className="inline">{fact.value}</dd>
              </div>
            ))}
          </dl>
        ) : null}
      </CardContent>
    </Card>
  );
}
