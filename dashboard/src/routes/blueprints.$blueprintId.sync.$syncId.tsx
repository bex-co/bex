import { createFileRoute } from "@tanstack/react-router";
import { BlueprintSyncPageSkeleton } from "@/common/components/route-skeletons";
import { BlueprintSyncPage } from "@/features/blueprints/pages/blueprint-sync-page";
import { blueprintPageHead } from "@/features/blueprints/lib/blueprint-page-head";

/**
 * One sync's page, Render's `/blueprint/<id>/sync/<syncId>` (w3/m678): the
 * page every sync `dashboardUrl` bex-api emits opens, and where Manual sync's
 * plan is reviewed and approved. Syncs is the open sidebar entry.
 */
export const Route = createFileRoute("/blueprints/$blueprintId/sync/$syncId")({
  head: blueprintPageHead("blueprints.syncDocumentTitle"),
  component: RouteComponent,
  pendingComponent: BlueprintSyncPageSkeleton,
});

function RouteComponent() {
  return <BlueprintSyncPage syncId={Route.useParams().syncId} />;
}
