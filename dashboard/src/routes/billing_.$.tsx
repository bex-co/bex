import { createFileRoute } from "@tanstack/react-router";
import {
  redirectPreservingSuffix,
  splatParts,
} from "@/common/lib/render-alias";

/**
 * Render's billing sub-URLs (w1/m45, renamed w5/m70). `/billing` itself is the
 * real bex billing page now; this catch-all keeps the two deeper shapes Render
 * mints sitewide landing somewhere sensible: `/billing/update-plan` (every
 * upgrade CTA) opens the change-plan dialog; any other sub-path folds back to
 * the billing page. The incoming query (Render appends `?next=…` etc.) has no
 * bex meaning and folds into the landing's own.
 */
export const Route = createFileRoute("/billing_/$")({
  beforeLoad: ({ params, location }) => {
    const [first] = splatParts(params._splat);
    redirectPreservingSuffix(
      first === "update-plan" ? "/workspace/settings?plan=change" : "/billing",
      location,
    );
  },
});
