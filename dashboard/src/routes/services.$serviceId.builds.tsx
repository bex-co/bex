import { serviceTabHead } from "@/features/services/lib/service-tab-head";
import { createFileRoute } from "@tanstack/react-router";
import { DeployHistoryView } from "@/features/deploys/components/deploy-history-view";
import { DeploysListSkeleton } from "@/common/components/route-skeletons";
import { redirectHistoryTabByType } from "@/features/services/lib/service-root-redirect";
import { parseDeployListSearch } from "@/features/deploys/lib/deploy-list-search";

export const Route = createFileRoute("/services/$serviceId/builds")({
  head: serviceTabHead("services.navBuilds", "/services/$serviceId"),
  // Only a cron job has a Builds page; another type lands on its own tab.
  beforeLoad: ({ context, params }) =>
    redirectHistoryTabByType(context.client, params.serviceId, "builds"),
  component: RouteComponent,
  pendingComponent: DeploysListSkeleton,
  validateSearch: parseDeployListSearch,
});

// A cron job's Builds page (w2/m360): Render splits a cron job's history into
// Builds and Runs (changelog 2026-09-03), and a cron job's deploy is its build,
// so this is the deploy history under Render's name; its rows still open the
// per-deploy route.
function RouteComponent() {
  const { serviceId } = Route.useParams();
  const search = Route.useSearch();
  const navigate = Route.useNavigate();
  return (
    <DeployHistoryView
      serviceId={serviceId}
      titleKey="deploys.buildsTitle"
      search={search}
      // The functional form commits the explicit undefineds, which is what
      // clears a key.
      replaceSearch={(next) =>
        void navigate({ search: () => next, replace: true })
      }
    />
  );
}
