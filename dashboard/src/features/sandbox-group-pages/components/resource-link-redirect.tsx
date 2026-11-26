import { useEffect } from "react";
import { Link, useNavigate } from "@tanstack/react-router";
import { useQuery } from "@apollo/client/react";
import { AlertTriangle } from "lucide-react";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { PageFrame, PageHeader } from "@/common/components/page-frame";
import { Button } from "@/common/components/ui/button";
import { Skeleton } from "@/common/components/ui/skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  ResourceTypeBySandboxIdDocument,
  ResourceTypeBySandboxSnapshotIdDocument,
} from "@/graphql/definitions";
import { useWorkspace } from "@/features/workspaces/context/hooks";

/**
 * A link to a sandbox or a snapshot by its id alone (w1/m658), as Render's
 * `resourceTypeBySandboxId` and `resourceTypeBySandboxSnapshotId` resolve
 * one: read the group it is in and land on that group's page with its
 * drawer open. A sandbox is found in any workspace the caller belongs to
 * (bex-api reads it there, and the group's page adopts that workspace); a
 * snapshot in the selected workspace. Anything else finds nothing.
 */
export function SandboxLinkRedirect({ sandboxId }: { sandboxId: string }) {
  const navigate = useNavigate();
  const { data, loading, error } = useQuery(ResourceTypeBySandboxIdDocument, {
    variables: { id: sandboxId },
    fetchPolicy: "network-only",
    errorPolicy: "all",
  });
  const groupId = data?.sandbox?.sandboxGroupId ?? null;
  useEffect(() => {
    if (!groupId) return;
    void navigate({
      to: "/sandbox-groups/$sandboxGroupId",
      params: { sandboxGroupId: groupId },
      search: { sandbox: sandboxId },
      replace: true,
    });
  }, [groupId, navigate, sandboxId]);
  if (loading || groupId) return <Resolving />;
  return (
    <NotFound kind="sandbox" id={sandboxId} failed={Boolean(error) && !data} />
  );
}

export function SnapshotLinkRedirect({ snapshotId }: { snapshotId: string }) {
  const navigate = useNavigate();
  const { currentWorkspaceId } = useWorkspace();
  const { data, loading, error } = useQuery(
    ResourceTypeBySandboxSnapshotIdDocument,
    {
      variables: { id: snapshotId, ownerId: currentWorkspaceId ?? undefined },
      skip: currentWorkspaceId == null,
      fetchPolicy: "network-only",
      errorPolicy: "all",
    },
  );
  const groupId = data?.sandboxSnapshot?.sandboxGroupId ?? null;
  useEffect(() => {
    if (!groupId) return;
    void navigate({
      to: "/sandbox-groups/$sandboxGroupId/snapshots",
      params: { sandboxGroupId: groupId },
      search: { snapshot: snapshotId },
      replace: true,
    });
  }, [groupId, navigate, snapshotId]);
  if (loading || groupId || currentWorkspaceId == null) return <Resolving />;
  return (
    <NotFound
      kind="snapshot"
      id={snapshotId}
      failed={Boolean(error) && !data}
    />
  );
}

function Resolving() {
  return (
    <DashboardLayout>
      <PageFrame
        pending="sandbox-link"
        header={<PageHeader title={<Skeleton className="h-6 w-44" />} />}
      >
        <Skeleton className="h-24 w-full" />
      </PageFrame>
    </DashboardLayout>
  );
}

function NotFound({
  kind,
  id,
  failed,
}: {
  kind: "sandbox" | "snapshot";
  id: string;
  failed: boolean;
}) {
  const { t } = useTranslations();
  return (
    <DashboardLayout>
      <PageFrame
        header={
          <PageHeader
            title={
              kind === "sandbox"
                ? t("sandboxGroupPages.sandboxGoneTitle")
                : t("sandboxGroupPages.snapshotGoneTitle")
            }
          />
        }
      >
        <div className="flex flex-col items-center gap-2 py-12 text-center">
          <AlertTriangle
            className="text-muted-foreground size-6"
            aria-hidden="true"
          />
          <p className="text-muted-foreground text-sm">
            {failed
              ? t("sandboxGroupPages.linkLoadError")
              : kind === "sandbox"
                ? t("sandboxGroupPages.sandboxGoneDescription", { id })
                : t("sandboxGroupPages.snapshotGoneDescription", { id })}
          </p>
          <Button asChild size="sm" variant="outline">
            <Link to="/sandbox-groups">
              {t("sandboxGroupPages.backToGroups")}
            </Link>
          </Button>
        </div>
      </PageFrame>
    </DashboardLayout>
  );
}
