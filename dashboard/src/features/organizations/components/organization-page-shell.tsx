import type { ReactNode } from "react";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { PageFrame } from "@/common/components/page-frame";
import {
  useOrganization,
  type UseOrganizationResult,
} from "@/features/organizations/hooks/use-organization";
import { useOrganizationLoginCheck } from "@/features/organizations/hooks/use-organization-login-check";
import { OrganizationPageHeader } from "@/features/organizations/components/organization-page-header";
import { OrganizationLoginBlocked } from "@/features/organizations/components/organization-login-blocked";
import { OrganizationUnavailable } from "@/features/organizations/components/organization-unavailable";
import type {
  Organization,
  OrganizationSection,
} from "@/features/organizations/types";

/**
 * Every org page's frame (w7/m464): the header with the org's name, the
 * caller's role and the org navigation, then the page's own body. While the
 * org loads it draws `skeleton` (the body of the route's pending frame); an
 * org that could not be read is said so; a read org renders `children`.
 *
 * A person the org's own login setting refuses (w7/m466, docs/ADR012-auth.md
 * 9c) sees one blocked card in place of the body, whichever page they are on:
 * the server refuses the org's own reads and actions with the coded 403 that
 * names only the org, the auth error link records it, and
 * `useOrganizationLoginCheck` reads the record and asks on the pages whose
 * reads are exempt.
 */
export function OrganizationPageShell({
  organizationId,
  section,
  skeleton,
  children,
}: {
  organizationId: string;
  section: OrganizationSection;
  skeleton: ReactNode;
  children: (
    ready: Omit<UseOrganizationResult, "organization"> & {
      organization: Organization;
    },
  ) => ReactNode;
}) {
  const state = useOrganization(organizationId);
  const { organization, role, loading, notFound } = state;
  const block = useOrganizationLoginCheck(organizationId, organization);
  return (
    <DashboardLayout>
      <PageFrame
        header={
          <OrganizationPageHeader
            organizationId={organizationId}
            section={section}
            name={organization?.name ?? null}
            role={role}
            pending={loading}
          />
        }
      >
        {block ? (
          <OrganizationLoginBlocked block={block} />
        ) : loading ? (
          skeleton
        ) : organization ? (
          children({ ...state, organization })
        ) : (
          <OrganizationUnavailable notFound={notFound} />
        )}
      </PageFrame>
    </DashboardLayout>
  );
}
