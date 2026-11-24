import { createFileRoute } from "@tanstack/react-router";
import { BuildSourceSettingsTab } from "@/features/build-sources/components/build-source-settings-tab";
import { BuildSourceSettingsTabSkeleton } from "@/features/build-sources/components/build-source-skeletons";

export const Route = createFileRoute("/build-sources_/$buildSourceId/settings")({
  component: BuildSourceSettingsTab,
  pendingComponent: BuildSourceSettingsTabSkeleton,
});
