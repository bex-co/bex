import { createFileRoute, redirect } from "@tanstack/react-router";
import { getDashboardOrigin } from "@/common/lib/document-head";
import { getReferrer } from "@/common/lib/referrer";
import {
  parseDeployReferrer,
  parseDeployRepo,
} from "@/features/blueprints/lib/deploy-link";

/**
 * Render's "Deploy to Render" button links to `render.com/deploy?repo=<url>`;
 * this is its bex counterpart (w8/163). It opens Blueprint creation for that
 * repository with the repo (and a `/tree/<branch>`) prefilled; nothing is
 * created until the user approves there, after sign-in if needed. As on Render,
 * a link with no `repo` deploys the repository page that linked here, when the
 * browser said which page that was (w33/003). Resolving it before the redirect
 * puts it in the URL the sign-in bounce keeps. A link that names no usable
 * repository lands on the empty wizard.
 */
export const Route = createFileRoute("/deploy")({
  validateSearch: (
    search: Record<string, unknown>,
  ): { repo?: string; path?: string } => ({
    ...(typeof search.repo === "string" && { repo: search.repo }),
    ...(typeof search.path === "string" && { path: search.path }),
  }),
  beforeLoad: ({ search }) => {
    const link = search.repo?.trim()
      ? parseDeployRepo(search.repo)
      : parseDeployReferrer(getReferrer(), getDashboardOrigin());
    throw redirect({
      to: "/blueprint/new",
      search: link
        ? { ...link, ...(search.path && { path: search.path }) }
        : {},
      replace: true,
    });
  },
});
