import { createFileRoute, getRouteApi } from "@tanstack/react-router";
import { CronPlanPage } from "@/features/services/components/cron-plan-page";
import { ServiceCronPlanSkeleton } from "@/common/components/route-skeletons";
import { redirectPlanByType } from "@/features/services/lib/service-root-redirect";

/**
 * A cron job's plan page, Render's `/cron/<id>/plan` (w3/105): its plan cards,
 * ready to change. A suspended cron job's lands on Settings. Every other type's
 * Plan URL, the Plan tab (w5/m7) before Render's Compute page (w6/m483), lands
 * on its Compute page, query and hash kept. No head of its own: Render's cron
 * title map names `/plan` null, so the service's title stays.
 */
export const Route = createFileRoute("/services/$serviceId/plan")({
  beforeLoad: ({ context, params, location }) =>
    redirectPlanByType(context.client, params.serviceId, location),
  component: RouteComponent,
  pendingComponent: ServiceCronPlanPending,
});

function ServiceCronPlanPending() {
  const parent = getRouteApi("/services/$serviceId").useLoaderData();
  return (
    <ServiceCronPlanSkeleton
      currentPlan={
        parent?.state === "ready" ? (parent.resource.plan ?? null) : undefined
      }
    />
  );
}

function RouteComponent() {
  const { serviceId } = Route.useParams();
  return <CronPlanPage serviceId={serviceId} />;
}
