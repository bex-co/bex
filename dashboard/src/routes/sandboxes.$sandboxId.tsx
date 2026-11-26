import { createFileRoute } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { SandboxLinkRedirect } from "@/features/sandbox-group-pages/components/resource-link-redirect";

/**
 * A sandbox by its id alone lands on its group's Sandboxes page with its
 * drawer open (w1/m658), as Render's `resourceTypeBySandboxId` redirect does.
 */
export const Route = createFileRoute("/sandboxes/$sandboxId")({
  staticData: { chrome: true },
  beforeLoad: requireAuth(),
  component: RouteComponent,
});

function RouteComponent() {
  const { sandboxId } = Route.useParams();
  return <SandboxLinkRedirect sandboxId={sandboxId} />;
}
