import { createFileRoute } from "@tanstack/react-router";
import { BuildSourceBuildsTab } from "@/features/build-sources/components/build-source-builds-tab";
import { BuildSourceBuildsTabSkeleton } from "@/features/build-sources/components/build-source-skeletons";

export const Route = createFileRoute("/build-sources_/$buildSourceId/")({
  component: BuildSourceBuildsTab,
  pendingComponent: BuildSourceBuildsTabSkeleton,
});
