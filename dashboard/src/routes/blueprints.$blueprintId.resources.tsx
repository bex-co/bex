import { createFileRoute } from "@tanstack/react-router";
import { BlueprintResourcesSkeleton } from "@/common/components/route-skeletons";
import { BlueprintResourcesPage } from "@/features/blueprints/pages/blueprint-resources-page";
import { blueprintPageHead } from "@/features/blueprints/lib/blueprint-page-head";

/** Render's `/blueprint/<id>/resources`, the Resources page its sidebar links (w3/m677). */
export const Route = createFileRoute("/blueprints/$blueprintId/resources")({
  head: blueprintPageHead("blueprints.navResources"),
  component: BlueprintResourcesPage,
  pendingComponent: BlueprintResourcesSkeleton,
});
