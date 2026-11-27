import { useTranslations } from "@/common/hooks/use-translations";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table";
import { BlueprintSyncRowsSkeleton } from "@/common/components/route-skeletons";
import { RelativeAge } from "@/common/components/relative-time";
import { PlannedSyncPanel } from "@/features/blueprints/components/planned-sync-panel";
import { usePlannedSync } from "@/features/blueprints/hooks/use-planned-sync";
import { useBlueprintSyncs } from "@/features/blueprints/hooks/use-blueprint-syncs";
import { BlueprintStatusBadge } from "@/features/blueprints/components/blueprint-status-badge";
import { useBlueprintDetail } from "@/features/blueprints/lib/blueprint-detail";

/**
 * A Blueprint's Syncs page, Render's `/blueprint/<id>/syncs` (w3/m677): the
 * plan waiting for approval, then the sync history, newest first. w3/m678
 * replaces both with Render's sync cards, each opening its sync's page.
 */
export function BlueprintSyncsPage() {
  const { t } = useTranslations();
  const { blueprint } = useBlueprintDetail();
  const { syncs, loading: syncsLoading } = useBlueprintSyncs(blueprint.id);
  // Any Blueprint can wait on a plan (a re-plan, or the first plan a
  // no-changes re-plan kept, w32/073), and Render links every waiting plan to
  // its dashboard, so the card shows whenever GraphQL returns one (w32/076).
  // Only a created Blueprint, whose way forward is the plan, keeps the card
  // while it loads or when none waits; any other reserves no card for a plan
  // it usually does not have.
  const plannedSync = usePlannedSync(blueprint.id, true);
  const showPlannedSync =
    blueprint.status === "created" || plannedSync.planned !== null;

  return (
    <>
      {showPlannedSync ? (
        <PlannedSyncPanel
          key={plannedSync.planned?.id ?? "none"}
          planned={plannedSync.planned}
          loading={plannedSync.loading}
          busy={plannedSync.busy}
          onApprove={plannedSync.approve}
          onReplan={plannedSync.replan}
        />
      ) : null}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">
            {t("blueprints.syncHistoryTitle")}
          </CardTitle>
        </CardHeader>
        <CardContent>
          {syncs.length > 0 || syncsLoading ? (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("blueprints.syncColCommit")}</TableHead>
                  <TableHead>{t("blueprints.syncColState")}</TableHead>
                  <TableHead>{t("blueprints.syncColStarted")}</TableHead>
                  <TableHead>{t("blueprints.syncColCompleted")}</TableHead>
                  <TableHead>{t("blueprints.syncColError")}</TableHead>
                </TableRow>
              </TableHeader>
              {syncs.length > 0 ? (
                <TableBody>
                  {syncs.map((run) => (
                    <TableRow key={run.id}>
                      <TableCell className="font-mono text-xs">
                        {run.commitId ? run.commitId.slice(0, 8) : "—"}
                      </TableCell>
                      <TableCell>
                        <BlueprintStatusBadge status={run.state} />
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        <RelativeAge value={run.startedAt} />
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        <RelativeAge value={run.completedAt} />
                      </TableCell>
                      <TableCell className="max-w-xs whitespace-normal break-words text-muted-foreground">
                        {run.errorMessage && <p>{run.errorMessage}</p>}
                        {run.note && <p>{run.note}</p>}
                        {!run.errorMessage && !run.note && "—"}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              ) : (
                // The syncs query is not awaited by the route's title loader,
                // so a fresh visit has a real in-flight window. Reserve the
                // ready table's shape (same header, three representative
                // rows) instead of flashing the empty copy (w6/043).
                <BlueprintSyncRowsSkeleton className="animate-pulse motion-reduce:animate-none" />
              )}
            </Table>
          ) : (
            <p className="text-sm text-muted-foreground">
              {t("blueprints.syncHistoryEmpty")}
            </p>
          )}
        </CardContent>
      </Card>
    </>
  );
}
