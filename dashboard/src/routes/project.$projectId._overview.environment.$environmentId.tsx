import { createFileRoute } from "@tanstack/react-router";

/**
 * Render's environment URL: the project's page with this environment's
 * section scrolled to and outlined (w1/m682). The `_overview` layout renders
 * it; an id the project has no section for lands on the project's own page.
 */
export const Route = createFileRoute(
  "/project/$projectId/_overview/environment/$environmentId",
)({});
