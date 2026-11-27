import { createFileRoute } from "@tanstack/react-router";
import { BlueprintResourcesSkeleton } from "@/common/components/route-skeletons";
import { BlueprintResourcesPage } from "@/features/blueprints/pages/blueprint-resources-page";
import { blueprintPageHead } from "@/features/blueprints/lib/blueprint-page-head";

/**
 * The bare `/blueprints/<id>` shows Resources, as Render's `/blueprint/<id>`
 * does (w3/m677). Every link into a Blueprint (the list, the search palette,
 * the post-create landing) opens here.
 */
export const Route = createFileRoute("/blueprints/$blueprintId/")({
  head: blueprintPageHead("blueprints.navResources"),
  component: BlueprintResourcesPage,
  pendingComponent: BlueprintResourcesSkeleton,
});
