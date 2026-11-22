import { createFileRoute } from "@tanstack/react-router";
import { redirectPreservingSuffix } from "@/common/lib/render-alias";

/** bex's old Blueprint create URL; the page is Render's `/blueprint/new`, and
 *  a Deploy-to-bex prefill (`?repo=…&branch=…`) carries over. */
export const Route = createFileRoute("/blueprints/new")({
  beforeLoad: ({ location }) => {
    redirectPreservingSuffix("/blueprint/new", location);
  },
});
