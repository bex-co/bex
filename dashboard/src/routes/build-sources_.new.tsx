import { createFileRoute } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { translatedTitleHead } from "@/common/lib/document-head";
import { CreateBuildSourcePage } from "@/features/build-sources/components/create-build-source-page";
import { CreateBuildSourceRouteSkeleton } from "@/features/build-sources/components/build-source-skeletons";
import { parseNewBuildSourceSearch } from "@/features/build-sources/lib/search-params";

// The trailing underscore escapes the /build-sources list route's component
// hierarchy while keeping the public URL `/build-sources/new` (same pattern as
// `webhooks_.new.tsx` and `sandbox-groups_.$sandboxGroupId.tsx`).
export const Route = createFileRoute("/build-sources_/new")({
  staticData: { chrome: true },
  component: BuildSourcesNewRoutePage,
  pendingComponent: CreateBuildSourceRouteSkeleton,
  beforeLoad: requireAuth(),
  validateSearch: parseNewBuildSourceSearch,
  head: ({ match }) => translatedTitleHead("buildSources.createTitle", match),
});

function BuildSourcesNewRoutePage() {
  const search = Route.useSearch();
  return <CreateBuildSourcePage initialProjectId={search.projectId} />;
}
