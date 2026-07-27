import { createFileRoute } from "@tanstack/react-router";
import {
  redirectPreservingSuffix,
  splatParts,
} from "@/common/lib/render-alias";

/**
 * Render's billing URLs (w1/m45). Collection-management UI remains deferred
 * (docs/ADR023-usage-metering.md — Usage is the deliberate counterpart), but
 * the two shapes Render mints sitewide still deserve a landing:
 * `/billing/update-plan` (every upgrade CTA) opens the change-plan dialog;
 * anything else under `/billing` lands on Usage. The incoming query (Render
 * appends `?next=…` etc.) has no bex meaning and folds into the landing's own.
 */
export const Route = createFileRoute("/billing/$")({
  beforeLoad: ({ params, location }) => {
    const [first] = splatParts(params._splat);
    redirectPreservingSuffix(
      first === "update-plan" ? "/workspace/settings?plan=change" : "/usage",
      location,
    );
  },
});
