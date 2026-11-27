import { useEffect } from "react";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { requireAuth } from "@/common/lib/auth/auth";
import {
  organizationAliasPath,
  redirectOrNotFound,
  splatParts,
} from "@/common/lib/render-alias";
import { WorkspaceAliasSkeleton } from "@/common/components/route-skeletons";
import { useWorkspace } from "@/features/workspaces/context/hooks";

/**
 * Render's organization URLs, `/organization/<org-id>/…` (w3/m705): redirect
 * to bex's `/org/<org-id>/…` pages with the query and hash kept
 * (`organizationAliasPath`); those pages refuse an org the caller is not in.
 * Render's org Billing is the org's primary workspace's Billing in bex, which
 * holds the org's plan (ADR115 D2): the component finds that workspace in the
 * switcher's list and hands it to `/w/<tea-id>/billing`, whose alias selects
 * it after checking the caller is in it. An org none of the caller's
 * workspaces belongs to goes to the org's page and its refusal.
 */
export const Route = createFileRoute("/organization/$")({
  staticData: { chrome: true },
  beforeLoad: (args) => {
    requireAuth()(args);
    const parts = splatParts(args.params._splat);
    if (parts[1] !== "billing" || !parts[0]) {
      redirectOrNotFound(organizationAliasPath(parts), args.location);
    }
  },
  component: OrganizationBillingAlias,
});

function OrganizationBillingAlias() {
  const { _splat } = Route.useParams();
  const [orgId] = splatParts(_splat);
  const navigate = useNavigate();
  const { workspaces, loading } = useWorkspace();
  const primary = workspaces.find((w) => w.organization?.id === orgId)
    ?.organization?.primaryWorkspaceId;

  useEffect(() => {
    if (loading) return;
    if (primary) {
      void navigate({
        to: "/w/$",
        params: { _splat: `${primary}/billing` },
        replace: true,
      });
    } else {
      void navigate({
        to: "/org/$orgId/workspaces",
        params: { orgId },
        replace: true,
      });
    }
  }, [loading, primary, orgId, navigate]);

  return <WorkspaceAliasSkeleton destination="billing" />;
}
