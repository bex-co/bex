import { createFileRoute } from "@tanstack/react-router";
import { redirectPreservingSuffix } from "@/common/lib/render-alias";

/**
 * Render's New Environment Group URL (`/new/env-group`, w3/m705): bex creates
 * an environment group in the list's dialog, whose open state the URL owns
 * (`?new=env-group`). Render's own full page is w1/m684 t003.
 */
export const Route = createFileRoute("/new/env-group")({
  beforeLoad: ({ location }) => {
    redirectPreservingSuffix("/env-groups?new=env-group", location);
  },
});
