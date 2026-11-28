import { createFileRoute } from "@tanstack/react-router";
import { redirectPreservingSuffix } from "@/common/lib/render-alias";

/**
 * Render's billing sub-URLs (w1/m45, renamed w5/m70). The required `first`
 * segment is deliberate: a bare splat also matches `/billing` with an empty
 * value, outranks the real billing page during SSR, and redirects it to itself.
 * Requiring one segment keeps `/billing` on the page route while preserving
 * the compatibility behavior for every actual `/billing/*` URL.
 *
 * Render's plan pages (`/billing/update-plan`, `-preview`, `-confirm`) are
 * routes of their own (w3/m708), which outrank this splat; any other sub-path
 * folds back to the billing page. The incoming query (Render appends
 * `?next=…` etc.) has no bex meaning here and folds into the landing's own.
 */
export const Route = createFileRoute("/billing_/$first/$")({
  beforeLoad: ({ location }) => {
    redirectPreservingSuffix("/billing", location);
  },
});
