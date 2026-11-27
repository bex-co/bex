import { createFileRoute, redirect } from "@tanstack/react-router";

/**
 * Any other page under an environment (`/project/<id>/environment/<envId>/
 * <page>`) lands on the project's page, as Render's `:environmentPage` route
 * sends every page but `settings` there (w1/m683). One segment, as Render's:
 * a splat would also take the environment's own URL, which is the project
 * page with its section outlined (w1/m682).
 */
export const Route = createFileRoute(
  "/project/$projectId/environment/$environmentId/$page",
)({
  beforeLoad: ({ params }) => {
    throw redirect({
      to: "/project/$projectId",
      params: { projectId: params.projectId },
      replace: true,
    });
  },
});
