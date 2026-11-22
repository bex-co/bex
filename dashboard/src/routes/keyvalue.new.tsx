import { createFileRoute } from "@tanstack/react-router";
import { redirectPreservingSuffix } from "@/common/lib/render-alias";

/** bex's old Key Value create URL; the page is Render's `/new/redis`. */
export const Route = createFileRoute("/keyvalue/new")({
  beforeLoad: ({ location }) => {
    redirectPreservingSuffix("/new/redis", location);
  },
});
