import { createFileRoute } from "@tanstack/react-router";
import {
  redirectOrNotFound,
  resourceAliasPath,
  splatParts,
} from "@/common/lib/render-alias";

/**
 * Render's `/resource/<id>[/…]` (w3/m705; its `ResourceRedirector`): opens the
 * page of the resource the id names, read from the id's prefix, with the
 * sub-path, query and hash kept (`resourceAliasPath`). A resource the id does
 * not find meets that page's own not-found handling.
 */
export const Route = createFileRoute("/resource/$")({
  beforeLoad: ({ params, location }) => {
    redirectOrNotFound(resourceAliasPath(splatParts(params._splat)), location);
  },
});
