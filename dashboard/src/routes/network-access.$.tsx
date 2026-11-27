import { createFileRoute } from "@tanstack/react-router";
import { redirectPreservingSuffix } from "@/common/lib/render-alias";

/**
 * Render's Network Access page (`/network-access`, w3/m705): its one panel,
 * Inbound IP Restrictions, is the workspace's Inbound IP Rules card on bex's
 * Workspace Settings.
 */
export const Route = createFileRoute("/network-access/$")({
  beforeLoad: ({ location }) => {
    redirectPreservingSuffix("/workspace/settings#inbound-ip-rules", location);
  },
});
