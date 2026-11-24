import { createFileRoute } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import {
  loadRouteResource,
  routeResourceTitle,
  titleHead,
  titleLoaderFetchPolicy,
  translatedText,
} from "@/common/lib/document-head";
import { useLoaderErrorRetry } from "@/common/hooks/use-loader-error-retry";
import { BuildSourceDocument } from "@/graphql/definitions";
import { BuildSourceLayout } from "@/features/build-sources/components/build-source-layout";
import { BuildSourceDetailRouteSkeleton } from "@/features/build-sources/components/build-source-skeletons";

// The trailing underscore escapes the `/build-sources` list route's component
// hierarchy while keeping the public URL `/build-sources/$buildSourceId`.
export const Route = createFileRoute("/build-sources_/$buildSourceId")({
  staticData: { chrome: true },
  component: BuildSourceDetailLayoutRoute,
  pendingComponent: BuildSourceDetailRouteSkeleton,
  pendingMs: 0,
  beforeLoad: requireAuth(),
  loader: ({ context, params, cause }) =>
    loadRouteResource(
      () =>
        context.client.query({
          query: BuildSourceDocument,
          variables: { id: params.buildSourceId },
          fetchPolicy: titleLoaderFetchPolicy(cause),
          errorPolicy: "all",
        }),
      (data) => (data?.buildSource?.name?.trim() ? data.buildSource : null),
    ),
  head: ({ loaderData, match }) =>
    titleHead(
      routeResourceTitle(loaderData, (source) => [
        source.name,
        translatedText("buildSources.resourceType"),
      ]),
      match,
    ),
});

function BuildSourceDetailLayoutRoute() {
  const { buildSourceId } = Route.useParams();
  useLoaderErrorRetry(Route.useLoaderData(), buildSourceId);
  return <BuildSourceLayout buildSourceId={buildSourceId} />;
}
