import { Loader2 } from "lucide-react";
import { useTranslations } from "@/common/hooks/use-translations";
import { Button } from "@/common/components/ui/button";
import { ResourceLoadError } from "@/common/components/resource-load-error";
import { BlueprintSyncCardsSkeleton } from "@/common/components/route-skeletons";
import { BlueprintSyncCard } from "@/features/blueprints/components/blueprint-sync-card";
import { useBlueprintSyncs } from "@/features/blueprints/hooks/use-blueprint-syncs";
import { useBlueprintDetail } from "@/features/blueprints/lib/blueprint-detail";

/**
 * A Blueprint's Syncs page, Render's `/blueprint/<id>/syncs` (w3/m678,
 * docs/render-artifacts/blueprint-pages.md § Syncs): its sync cards, newest
 * first, each opening its sync's page, a plan waiting for approval among
 * them; "No syncs yet." when there are none; Load more for older ones.
 */
export function BlueprintSyncsPage() {
  const { t } = useTranslations();
  const { blueprint, linkId } = useBlueprintDetail();
  const { syncs, loading, loadingMore, error, hasMore, loadMore } =
    useBlueprintSyncs(blueprint.id);

  if (syncs.length === 0) {
    // The syncs read is not awaited by the route's title loader, so a fresh
    // visit has a real in-flight window: the list's own shape, never the
    // empty copy (w6/043).
    if (loading) return <BlueprintSyncCardsSkeleton />;
    return error ? (
      <ResourceLoadError />
    ) : (
      <p className="text-sm text-muted-foreground">
        {t("blueprints.syncHistoryEmpty")}
      </p>
    );
  }
  return (
    <div className="space-y-4">
      <ul className="space-y-3" aria-label={t("blueprints.navSyncs")}>
        {syncs.map((sync) => (
          <li key={sync.id}>
            <BlueprintSyncCard
              sync={sync}
              repo={blueprint.repo}
              blueprintId={linkId}
            />
          </li>
        ))}
      </ul>
      {hasMore ? (
        <div className="flex justify-center">
          <Button
            variant="outline"
            size="sm"
            onClick={loadMore}
            disabled={loadingMore}
          >
            {loadingMore ? (
              <Loader2 aria-hidden="true" className="animate-spin" />
            ) : null}
            {t("blueprints.syncsLoadMore")}
          </Button>
        </div>
      ) : null}
    </div>
  );
}
