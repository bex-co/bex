import { createFileRoute } from "@tanstack/react-router";
import {
  redirectOrNotFound,
  sandboxGroupAliasPath,
  splatParts,
} from "@/common/lib/render-alias";

/**
 * Render's sandbox group URLs, `/sandbox-group/<sbg-id>/…` and
 * `/sandbox-group/new` (w3/m705, docs/render-artifacts/sandbox-groups.md):
 * redirect to bex's `/sandbox-groups/<sbg-id>` pages and drawers, or the
 * create dialog, with the query and hash kept (`sandboxGroupAliasPath`).
 */
export const Route = createFileRoute("/sandbox-group/$")({
  beforeLoad: ({ params, location }) => {
    redirectOrNotFound(
      sandboxGroupAliasPath(splatParts(params._splat)),
      location,
    );
  },
});
