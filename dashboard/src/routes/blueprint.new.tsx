import { createFileRoute } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import { translatedTitleHead } from "@/common/lib/document-head";
import { BlueprintCreatePageSkeleton } from "@/common/components/route-skeletons";
import { NewBlueprintPage } from "@/features/blueprints/pages/new-blueprint-page";
import { prefetchCreatePage } from "@/common/lib/create-page-prefetch";

/** Render's Blueprint create URL (`/blueprint/new`, singular —
 *  docs/render-artifacts/dashboard-routes.md); bex's old `/blueprints/new`
 *  lands here. */
export const Route = createFileRoute("/blueprint/new")({
  staticData: { chrome: true },
  component: NewBlueprintPage,
  pendingComponent: BlueprintCreatePageSkeleton,
  // The Deploy-to-bex button (/deploy?repo=…) prefills the public-URL source
  // (w8/163).
  validateSearch: (
    search: Record<string, unknown>,
  ): { repo?: string; branch?: string; path?: string } => ({
    ...(typeof search.repo === "string" && { repo: search.repo }),
    ...(typeof search.branch === "string" && { branch: search.branch }),
    ...(typeof search.path === "string" && { path: search.path }),
  }),
  beforeLoad: requireAuth(),
  loader: (args) => prefetchCreatePage(args, { git: true }),
  head: ({ match }) => translatedTitleHead("blueprints.createTitle", match),
});
