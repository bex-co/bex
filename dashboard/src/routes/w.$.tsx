import { useEffect } from "react";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import {
  redirectPreservingSuffix,
  splatParts,
} from "@/common/lib/render-alias";
import { WorkspaceAliasSkeleton } from "@/common/components/route-skeletons";
import { useNotFoundRedirect } from "@/common/hooks/use-not-found-redirect";
import { useWorkspace } from "@/features/workspaces/context/hooks";

/**
 * Render's workspace-scoped URL scheme (w1/m45, live capture 2026-07-16 —
 * docs/render-artifacts/dashboard-routes.md § Workspace/user/create scheme):
 * `/w/settings` (id-less; Render canonicalizes it to the current workspace)
 * and `/w/{tea-id}/…` (settings, billing). bex's workspace pages are
 * switcher-driven, so a NAMED tea-id must first become the switcher's
 * selection — a bare redirect would silently show a DIFFERENT workspace
 * whenever the switcher points elsewhere, the confused-deputy shape the
 * backend's resolver refuses. The selection is membership-checked against the
 * switcher's own list (which only ever contains memberships): a foreign or
 * unknown id redirects home with a not-found toast (w9/m55), never a silent
 * fallback to the caller's own workspace.
 */
export const Route = createFileRoute("/w/$")({
  staticData: { chrome: true },
  beforeLoad: (args) => {
    // No-arg requireAuth: `next` is the requested href, so after login the
    // browser comes back through the alias and the named workspace still gets
    // selected.
    requireAuth()(args);
    const parts = splatParts(args.params._splat);
    // Id-less forms need no selection change — pure redirects.
    if (parts.length === 0 || parts[0] === "settings") {
      redirectPreservingSuffix("/workspace/settings", args.location);
    }
  },
  component: WorkspaceAliasPage,
});

/** The bex landing for each Render workspace sub-page. Billing lands on bex's
 *  own /billing page (renamed from /usage in w5/m70; the usage API keeps its
 *  ADR023 name); the bare workspace root is the overview; unknown sub-pages
 *  fall back to settings — the workspace-scoped page that exists. */
function landingFor(
  sub: string | undefined,
): "/" | "/billing" | "/workspace/settings" {
  if (sub === undefined) return "/";
  if (sub === "billing") return "/billing";
  return "/workspace/settings";
}

function WorkspaceAliasPage() {
  const { _splat } = Route.useParams();
  const navigate = useNavigate();
  const { workspaces, loading, currentWorkspaceId, setCurrentWorkspaceId } =
    useWorkspace();

  const [teaId, sub] = splatParts(_splat);
  const isMember = workspaces.some((w) => w.id === teaId);

  // A foreign or unknown workspace id redirects home with a toast (w9/m55) —
  // still never a silent fallback to the caller's own workspace.
  useNotFoundRedirect(!loading && !isMember);

  useEffect(() => {
    if (loading || !isMember) return;
    if (teaId !== currentWorkspaceId) setCurrentWorkspaceId(teaId);
    void navigate({ to: landingFor(sub), replace: true });
  }, [
    loading,
    isMember,
    teaId,
    sub,
    currentWorkspaceId,
    setCurrentWorkspaceId,
    navigate,
  ]);

  // Preserve the exact destination geometry throughout: while membership
  // resolves, while a member selection lands, and while an unknown-id
  // redirect home is in flight.
  const destination =
    sub === "billing" ? "billing" : sub === undefined ? "overview" : "settings";
  return <WorkspaceAliasSkeleton destination={destination} />;
}
