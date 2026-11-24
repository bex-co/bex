import { createFileRoute } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import {
  titleLoaderFetchPolicy,
  translatedTitleHead,
} from "@/common/lib/document-head";
import { prefetchInParallel } from "@/common/lib/prefetch";
import { BuildSourcesDocument, ProjectsDocument } from "@/graphql/definitions";
import { BuildSourcesListPage } from "@/features/build-sources/components/build-sources-list-page";
import { BuildSourcesListRouteSkeleton } from "@/features/build-sources/components/build-source-skeletons";
import { parseBuildSourcesListSearch } from "@/features/build-sources/lib/search-params";

export const Route = createFileRoute("/build-sources")({
  staticData: { chrome: true },
  component: BuildSourcesRoutePage,
  pendingComponent: BuildSourcesListRouteSkeleton,
  beforeLoad: requireAuth(),
  validateSearch: parseBuildSourcesListSearch,
  loader: ({ context, cause }) => {
    const ownerId = context.workspaceId;
    if (ownerId == null) return;
    const fetchPolicy = titleLoaderFetchPolicy(cause);
    return prefetchInParallel([
      () =>
        context.client.query({
          query: BuildSourcesDocument,
          variables: { ownerId },
          fetchPolicy,
          errorPolicy: "all",
        }),
      () =>
        context.client.query({
          query: ProjectsDocument,
          variables: { ownerId },
          fetchPolicy,
          errorPolicy: "all",
        }),
    ]);
  },
  head: ({ match }) => translatedTitleHead("buildSources.pageTitle", match),
});

function BuildSourcesRoutePage() {
  const search = Route.useSearch();
  const navigate = Route.useNavigate();
  return (
    <BuildSourcesListPage
      initialQuery={search.q}
      initialKind={search.kind}
      onFilterChange={(next) =>
        void navigate({
          search: {
            q: next.q,
            kind: next.kind,
          },
          replace: true,
        })
      }
    />
  );
}
