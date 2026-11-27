import { createFileRoute } from "@tanstack/react-router";
import {
  envGroupAliasPath,
  redirectOrNotFound,
  splatParts,
} from "@/common/lib/render-alias";

/**
 * Render's environment group URL, `/env-group/<evg-id>` (w3/m705; Render's
 * `urls` chunk builds `/env-group/${id}`): redirects to bex's
 * `/env-groups/<evg-id>` with the query and hash kept. A sub-path lands on
 * the group, Render's group having one page; `/env-group/new` is Render's
 * New Environment Group, and a path with no id is not found.
 */
export const Route = createFileRoute("/env-group/$")({
  beforeLoad: ({ params, location }) => {
    redirectOrNotFound(envGroupAliasPath(splatParts(params._splat)), location);
  },
});
