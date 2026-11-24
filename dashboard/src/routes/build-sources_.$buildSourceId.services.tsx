import { createFileRoute } from "@tanstack/react-router";
import { BuildSourceServicesTab } from "@/features/build-sources/components/build-source-services-tab";
import { BuildSourceServicesTabSkeleton } from "@/features/build-sources/components/build-source-skeletons";

export const Route = createFileRoute("/build-sources_/$buildSourceId/services")({
  component: BuildSourceServicesTab,
  pendingComponent: BuildSourceServicesTabSkeleton,
});
