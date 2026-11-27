import { createFileRoute } from "@tanstack/react-router";
import {
  redirectPreservingSuffix,
  renderCreateSlugLanding,
} from "@/common/lib/render-alias";

/**
 * `/env-groups/new`, one of Render's create slugs (w3/m705): New Environment
 * Group, not a group named `new` under `env-groups_.$groupId`.
 */
export const Route = createFileRoute("/env-groups_/new")({
  beforeLoad: ({ location }) => {
    redirectPreservingSuffix(renderCreateSlugLanding("env-groups"), location);
  },
});
