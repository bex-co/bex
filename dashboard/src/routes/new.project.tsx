import { createFileRoute } from "@tanstack/react-router";
import { redirectPreservingSuffix } from "@/common/lib/render-alias";

/** Render's New-menu project create URL (`/new/project`, live capture
 *  2026-07-16). bex creates projects via the overview's dialog — the URL owns
 *  its open state (`?new=project`). */
export const Route = createFileRoute("/new/project")({
  beforeLoad: ({ location }) => {
    redirectPreservingSuffix("/?new=project", location);
  },
});
