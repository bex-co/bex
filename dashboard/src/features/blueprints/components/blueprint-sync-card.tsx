import { Link } from "@tanstack/react-router";
import { Workflow } from "lucide-react";
import { Badge } from "@/common/components/ui/badge";
import { RelativeAge } from "@/common/components/relative-time";
import { useTranslations } from "@/common/hooks/use-translations";
import { CommitSha } from "@/features/deploys/components/commit-sha";
import { StatusKindIcon } from "@/features/deploys/components/deploy-status-icon";
import { deploySubject } from "@/features/deploys/lib/deploy-presentation";
import type { DeployStatusKind } from "@/features/deploys/lib/deploy-status";
import { repoCommitUrl } from "@/features/services/lib/repo";
import {
  SYNC_STATE_LABEL,
  isApplyingSyncState,
} from "@/features/blueprints/lib/sync-state";
import type { BlueprintSyncView } from "@/features/blueprints/types";

// Each state's icon on Render's sync card (IACSyncCard-*.js), drawn with the
// deploy list's status icons: a success's check, an error's cross, a running
// sync's spinner. A plan waiting for approval has Render's workflow icon.
const STATE_ICON: Record<string, DeployStatusKind> = {
  pending: "in_progress",
  running: "in_progress",
  success: "succeeded",
  error: "failed",
};

/** A sync's state as Render's card draws it: one icon, named by the state. */
export function BlueprintSyncStateIcon({ state }: { state: string }) {
  const { t } = useTranslations();
  const labelKey = SYNC_STATE_LABEL[state];
  const label = labelKey ? t(labelKey) : state;
  const kind = STATE_ICON[state];
  if (kind) return <StatusKindIcon kind={kind} label={label} />;
  return (
    <Workflow
      role="img"
      aria-label={label}
      className="size-[18px] shrink-0 text-muted-foreground"
    />
  );
}

/**
 * One sync as Render's sync card shows it (w3/m678,
 * docs/render-artifacts/blueprint-pages.md § The sync card): the state's
 * icon; the commit's short id, linked to the commit on its Git host, and its
 * message; **View details** to the sync's page; a badge only while the sync
 * applies; when it started. The sync's own page draws it without View
 * details.
 */
export function BlueprintSyncCard({
  sync,
  repo,
  blueprintId,
  showDetails = true,
}: {
  sync: BlueprintSyncView;
  repo: string;
  /** The Blueprint id as the URL spells it. */
  blueprintId: string;
  showDetails?: boolean;
}) {
  const { t } = useTranslations();
  const subject = deploySubject(sync.commitMessage);
  return (
    <div
      data-testid="blueprint-sync-card"
      className="flex items-start gap-3 rounded-lg border bg-card px-4 py-3"
    >
      <span className="flex h-5 items-center">
        <BlueprintSyncStateIcon state={sync.state} />
      </span>
      <div className="flex min-w-0 flex-1 flex-col gap-2 md:flex-row md:items-center md:justify-between md:gap-4">
        <div className="min-w-0 space-y-1">
          {sync.commitId ? (
            <p className="flex min-w-0 text-sm leading-5">
              <CommitSha
                commitId={sync.commitId}
                url={repoCommitUrl(repo, sync.commitId)}
                // No enlarged touch target: it would cover View details,
                // 4px below.
                className="shrink-0 max-lg:after:content-none"
              />
              {subject ? (
                <>
                  <span className="shrink-0 font-mono">:</span>
                  <span
                    className="ml-1.5 min-w-0 truncate"
                    title={sync.commitMessage}
                  >
                    {subject}
                  </span>
                </>
              ) : null}
            </p>
          ) : null}
          {showDetails ? (
            <p className="text-sm leading-5">
              <Link
                to="/blueprints/$blueprintId/sync/$syncId"
                params={{ blueprintId, syncId: sync.id }}
                className="underline decoration-muted-foreground/60 underline-offset-2 hover:decoration-foreground"
              >
                {t("blueprints.syncViewDetails")}
              </Link>
            </p>
          ) : null}
        </div>
        {/* As tall as the badge, so a card keeps its height when the badge
            comes and goes. */}
        <div className="flex min-h-[22px] shrink-0 items-center gap-3">
          {isApplyingSyncState(sync.state) ? (
            <Badge variant="secondary">
              {t(SYNC_STATE_LABEL[sync.state] ?? "blueprints.stateRunning")}
            </Badge>
          ) : null}
          <RelativeAge
            value={sync.startedAt}
            title={sync.startedAt ?? undefined}
            className="whitespace-nowrap text-sm leading-5 text-muted-foreground"
          />
        </div>
      </div>
    </div>
  );
}
