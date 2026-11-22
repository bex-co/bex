import { serviceTabHead } from "@/features/services/lib/service-tab-head";
import { createFileRoute } from "@tanstack/react-router";
import { CronRunsSection } from "@/features/services/components/cron-runs-section";
import { ServiceCronRunsSkeleton } from "@/common/components/route-skeletons";
import { redirectRunsByType } from "@/features/services/lib/service-root-redirect";

export const Route = createFileRoute("/services/$serviceId/runs")({
  head: serviceTabHead("services.navRuns", "/services/$serviceId"),
  // Only a cron job has a Runs page; another type lands on its own tab.
  beforeLoad: ({ context, params }) =>
    redirectRunsByType(context.client, params.serviceId),
  component: RouteComponent,
  pendingComponent: ServiceCronRunsSkeleton,
});

// A cron job's Runs page (w2/m360), as on Render: run history, each run's
// detail and logs, Trigger Run and cancel. It used to be a section under the
// Events feed, and it is where a cron job's bare URL lands, as Render shows
// Runs at `/cron/<id>` (w3/m554).
function RouteComponent() {
  const { serviceId } = Route.useParams();
  return <CronRunsSection key={serviceId} serviceId={serviceId} />;
}
