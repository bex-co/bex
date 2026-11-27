import { createFileRoute } from "@tanstack/react-router";
import { redirectPreservingSuffix } from "@/common/lib/render-alias";

/**
 * Render's `/settings/spend-limit` (w3/m705): Render sends a team's owner to
 * its workspace settings' `#build-pipeline` (`AccountSpendLimitRedirect`), the
 * card where bex sets the build pipeline's spend limit (w6/m309).
 */
export const Route = createFileRoute("/settings_/spend-limit")({
  beforeLoad: ({ location }) => {
    redirectPreservingSuffix("/workspace/settings#build-pipeline", location);
  },
});
