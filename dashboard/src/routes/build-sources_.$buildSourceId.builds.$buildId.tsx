import { createFileRoute } from "@tanstack/react-router";
import { BuildDetailPage } from "@/features/build-sources/components/build-detail-page";
import { BuildDetailRouteSkeleton } from "@/features/build-sources/components/build-source-skeletons";

export const Route = createFileRoute(
  "/build-sources_/$buildSourceId/builds/$buildId",
)({
  component: BuildDetailRouteComponent,
  pendingComponent: BuildDetailRouteSkeleton,
});

function BuildDetailRouteComponent() {
  const { buildSourceId, buildId } = Route.useParams();
  return <BuildDetailPage buildSourceId={buildSourceId} buildId={buildId} />;
}
