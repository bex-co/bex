import { Card, CardContent } from "@/common/components/ui/card";
import { Badge } from "@/common/components/ui/badge";
import { useTranslations } from "@/common/hooks/use-translations";
type Translate = ReturnType<typeof useTranslations>["t"];
import {
  deployStatusVariant,
  deployStatusKey,
  deployTriggerKey,
  preDeployStatusKey,
} from "@/features/deploys/lib/deploy-status";
import type { DeployView } from "../hooks/use-deploy";

function triggerLabel(
  trigger: string,
  rollbackOf: string,
  t: Translate,
): string {
  if (rollbackOf)
    return t("deploys.triggerRollback", { deployId: rollbackOf });
  const key = deployTriggerKey(trigger);
  return key ? t(key as Parameters<Translate>[0]) : trigger;
}

function formatTimestamp(iso: string | null): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return d.toLocaleString();
}

export interface DeployHeaderProps {
  deploy: DeployView;
}

/**
 * The deploy detail page's header (w9/m1/t003): Render's compact deploy-page
 * banner — status badge, trigger/rollback provenance, image, pre-deploy
 * outcome, and the created/started/finished timestamps. Reuses the same
 * status→badge mapping as the Events tab (deploy-status.ts) so the two
 * surfaces can't drift on what a given status looks like.
 */
export function DeployHeader({ deploy }: DeployHeaderProps) {
  const { t } = useTranslations();
  const preDeploy = preDeployStatusKey(deploy.preDeployStatus);

  return (
    <Card>
      <CardContent className="space-y-3 py-4">
        <div className="flex flex-wrap items-center gap-2">
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

        {/* The resolved commit this deploy ran (w9/001) — Render's deploy-page
            header leads with it for repo-backed deploys: short SHA + the
            message's first line. Absent (image-backed, or no GitHub connection
            to resolve through) => omitted, not faked. */}
        {deploy.commitId && (
          <p className="truncate text-xs text-foreground">
            <span className="font-mono text-muted-foreground">
              {deploy.commitId.slice(0, 7)}
            </span>
            {deploy.commitMessage && (
              <> {deploy.commitMessage.split("\n")[0]}</>
            )}
          </p>
        )}

        {deploy.image && (
          <p className="truncate font-mono text-xs text-muted-foreground">
            {deploy.image}
          </p>
        )}

        <dl className="grid grid-cols-1 gap-x-6 gap-y-1 text-xs text-muted-foreground sm:grid-cols-3">
          <div>
            <dt className="inline font-medium">{t("deploys.created")}: </dt>
            <dd className="inline">{formatTimestamp(deploy.createdAt)}</dd>
          </div>
          <div>
            <dt className="inline font-medium">{t("deploys.started")}: </dt>
            <dd className="inline">
              {formatTimestamp(deploy.startedAt) || t("deploys.notYet")}
            </dd>
          </div>
          <div>
            <dt className="inline font-medium">{t("deploys.finished")}: </dt>
            <dd className="inline">
              {formatTimestamp(deploy.finishedAt) || t("deploys.notYet")}
            </dd>
          </div>
        </dl>
      </CardContent>
    </Card>
  );
}
