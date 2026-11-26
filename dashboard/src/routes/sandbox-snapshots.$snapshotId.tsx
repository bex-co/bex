import { createFileRoute } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { SnapshotLinkRedirect } from "@/features/sandbox-group-pages/components/resource-link-redirect";

/**
 * A sandbox snapshot by its id alone lands on its group's Snapshots page with
 * its drawer open (w1/m658), as Render's `resourceTypeBySandboxSnapshotId`
 * redirect does.
 */
export const Route = createFileRoute("/sandbox-snapshots/$snapshotId")({
  staticData: { chrome: true },
  beforeLoad: requireAuth(),
  component: RouteComponent,
});

function RouteComponent() {
  const { snapshotId } = Route.useParams();
  return <SnapshotLinkRedirect snapshotId={snapshotId} />;
}
