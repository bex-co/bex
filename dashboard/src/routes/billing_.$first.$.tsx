import { createFileRoute } from "@tanstack/react-router";
import { redirectPreservingSuffix } from "@/common/lib/render-alias";

/**
 * Render's billing sub-URLs (w1/m45, renamed w5/m70). The required `first`
 * segment is deliberate: a bare splat also matches `/billing` with an empty
 * value, outranks the real billing page during SSR, and redirects it to itself.
 * Requiring one segment keeps `/billing` on the page route while preserving
 * the compatibility behavior for every actual `/billing/*` URL.
 *
 * `/billing/update-plan` (every upgrade CTA) opens the change-plan dialog; any
 * other sub-path folds back to the billing page. The incoming query (Render
 * appends `?next=…` etc.) has no bex meaning and folds into the landing's own.
 */
export const Route = createFileRoute("/billing_/$first/$")({
  beforeLoad: ({ params, location }) => {
    redirectPreservingSuffix(
      params.first === "update-plan"
        ? "/workspace/settings?plan=change"
        : "/billing",
      location,
    );
  },
});
