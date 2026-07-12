import { createFileRoute } from "@tanstack/react-router";
import { AutoscalingSection } from "@/features/services/components/autoscaling-section";

export const Route = createFileRoute("/services/$serviceId/scaling")({
  component: ServiceScalingPage,
  head: ({ params }) => ({
    meta: [{ title: `${params.serviceId} · Scaling · bex dashboard` }],
  }),
});

function ServiceScalingPage() {
  const { serviceId } = Route.useParams();
  return (
    <div className="space-y-6">
      <AutoscalingSection serviceId={serviceId} />
    </div>
  );
}
