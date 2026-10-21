import { createFileRoute, redirect } from "@tanstack/react-router";
import { parseDeployRepo } from "@/features/blueprints/lib/deploy-link";

/**
 * Render's "Deploy to Render" button links to `render.com/deploy?repo=<url>`;
 * this is its bex counterpart (w8/163). It opens Blueprint creation for that
 * repository with the repo (and a `/tree/<branch>`) prefilled; nothing is
 * created until the user approves there, after sign-in if needed. A missing or
 * unusable `repo` lands on the empty wizard.
 */
export const Route = createFileRoute("/deploy")({
  validateSearch: (
    search: Record<string, unknown>,
  ): { repo?: string; path?: string } => ({
    ...(typeof search.repo === "string" && { repo: search.repo }),
    ...(typeof search.path === "string" && { path: search.path }),
  }),
  beforeLoad: ({ search }) => {
    const link = parseDeployRepo(search.repo);
    throw redirect({
      to: "/blueprints/new",
      search: link
        ? { ...link, ...(search.path && { path: search.path }) }
        : {},
      replace: true,
    });
  },
});
