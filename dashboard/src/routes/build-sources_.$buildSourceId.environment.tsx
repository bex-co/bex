import { createFileRoute } from "@tanstack/react-router";
import { BuildSourceEnvironmentTab } from "@/features/build-sources/components/build-source-environment-tab";
import { BuildSourceEnvironmentTabSkeleton } from "@/features/build-sources/components/build-source-skeletons";

export const Route = createFileRoute(
  "/build-sources_/$buildSourceId/environment",
)({
  component: BuildSourceEnvironmentTab,
  pendingComponent: BuildSourceEnvironmentTabSkeleton,
});
