import { createFileRoute } from "@tanstack/react-router";
import { BlueprintSyncsSkeleton } from "@/common/components/route-skeletons";
import { BlueprintSyncsPage } from "@/features/blueprints/pages/blueprint-syncs-page";
import { blueprintPageHead } from "@/features/blueprints/lib/blueprint-page-head";

/** A Blueprint's Syncs page, Render's `/blueprint/<id>/syncs` (w3/m677). */
export const Route = createFileRoute("/blueprints/$blueprintId/syncs")({
  head: blueprintPageHead("blueprints.navSyncs"),
  component: BlueprintSyncsPage,
  pendingComponent: BlueprintSyncsSkeleton,
});
