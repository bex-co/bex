import { createFileRoute, redirect } from "@tanstack/react-router";
import { keyValueSubpathLanding } from "@/features/keyvalue/lib/key-value-pages";
import { keyValuePageRoute } from "@/features/keyvalue/components/key-value-nav";

/**
 * Every sub-path of a Key Value instance with no page of its own (w3/m649):
 * Render's `/plan` and `/metrics/<period>` arrive here through the `/r/<id>/…`
 * alias, and land where their control is, query and hash kept
 * (keyValueSubpathLanding). Anything else lands on Info, as Render's `*` route
 * shows Info.
 */
export const Route = createFileRoute("/keyvalue/$keyValueId/$")({
  beforeLoad: ({ params, location }) => {
    const landing = keyValueSubpathLanding(params._splat);
    throw redirect({
      to: landing.page
        ? keyValuePageRoute(landing.page)
        : "/keyvalue/$keyValueId",
      params: { keyValueId: params.keyValueId },
      search: true,
      hash: location.hash || landing.hash,
      replace: true,
    });
  },
});
