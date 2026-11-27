import { createFileRoute } from "@tanstack/react-router";
import {
  newPathLanding,
  redirectPreservingSuffix,
  splatParts,
} from "@/common/lib/render-alias";

/**
 * Render's `/new` chooser and `/new/<slug>` (w3/m705): `/new` and an unknown
 * slug open the create landing (`RENDER_NEW_LANDING`); a service type, Render's
 * other create slugs (`postgres`, `kv`, `blueprints`, …) and `team` open their
 * create pages, read case-insensitively, with the query and hash kept.
 * `/new/database`, `/new/redis`, `/new/project`, `/new/workspace` and
 * `/new/env-group` are routes of their own and answer first.
 */
export const Route = createFileRoute("/new/$")({
  beforeLoad: ({ params, location }) => {
    redirectPreservingSuffix(
      newPathLanding(splatParts(params._splat)[0]),
      location,
    );
  },
});
