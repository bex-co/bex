import { createFileRoute } from "@tanstack/react-router";
import { BlueprintSettingsSkeleton } from "@/common/components/route-skeletons";
import { BlueprintSettingsPage } from "@/features/blueprints/pages/blueprint-settings-page";
import { blueprintPageHead } from "@/features/blueprints/lib/blueprint-page-head";

/** A Blueprint's Settings page, Render's `/blueprint/<id>/settings` (w3/m677). */
export const Route = createFileRoute("/blueprints/$blueprintId/settings")({
  head: blueprintPageHead("blueprints.navSettings"),
  component: BlueprintSettingsPage,
  pendingComponent: BlueprintSettingsSkeleton,
});
