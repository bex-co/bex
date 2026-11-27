import { createFileRoute } from "@tanstack/react-router";
import { BlueprintSyncPageSkeleton } from "@/common/components/route-skeletons";
import { BlueprintPlannedSyncPage } from "@/features/blueprints/pages/blueprint-sync-page";
import { blueprintPageHead } from "@/features/blueprints/lib/blueprint-page-head";

/**
 * Render's `/blueprint/<id>/sync` with no id (w3/m678): the plan Manual sync
 * just made that saved no sync, handed over in the navigation's state, else
 * Syncs.
 */
export const Route = createFileRoute("/blueprints/$blueprintId/sync/")({
  head: blueprintPageHead("blueprints.syncDocumentTitle"),
  component: BlueprintPlannedSyncPage,
  pendingComponent: BlueprintSyncPageSkeleton,
});
