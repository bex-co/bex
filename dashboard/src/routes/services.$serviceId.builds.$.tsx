import { createFileRoute } from "@tanstack/react-router";
import { splatParts } from "@/common/lib/render-alias";
import { redirectBuildsByType } from "@/features/services/lib/service-root-redirect";

/**
 * A cron job's Builds URL, and a build's (w3/m554): Render's
 * `/cron/<id>/builds` replaces itself with `…/deploys` and keeps a build's
 * detail at `…/builds/<id>`, so `/services/<id>/builds` lands on the Deploys
 * page with its query and `/services/<id>/builds/<dep>` on that deploy. The
 * w5/m39 alias carries `/cron/<id>/builds…` here. It replaces w2/m360's Builds
 * page, so bex links minted for that page land too.
 */
export const Route = createFileRoute("/services/$serviceId/builds/$")({
  beforeLoad: ({ context, params }) =>
    redirectBuildsByType(
      context.client,
      params.serviceId,
      splatParts(params._splat)[0],
    ),
});
