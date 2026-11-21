import { useState } from "react";
import { Link, createFileRoute } from "@tanstack/react-router";
import { AlertTriangle, Box, Plus } from "lucide-react";
import { requireAuth } from "@/common/lib/auth/auth";
import {
  translatedTitleHead,
  titleLoaderFetchPolicy,
} from "@/common/lib/document-head";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { PageFrame, PageHeader } from "@/common/components/page-frame";
import { CopyButton } from "@/common/components/copy-button";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import { Card, CardContent } from "@/common/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/common/components/ui/table";
import { useTranslations } from "@/common/hooks/use-translations";
import { SandboxGroupsDocument } from "@/graphql/definitions";
import { NewSandboxGroupDialog } from "@/features/sandbox-groups/components/new-sandbox-group-dialog";
import {
  SandboxGroupsListContentSkeleton,
  SandboxGroupsListRouteSkeleton,
} from "@/features/sandbox-groups/components/sandbox-group-skeletons";
import { useSandboxGroups } from "@/features/sandbox-groups/hooks/use-sandbox-groups";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { useWorkspace } from "@/features/workspaces/context/hooks";

export const Route = createFileRoute("/sandbox-groups")({
  staticData: { chrome: true },
  component: SandboxGroupsPage,
  pendingComponent: SandboxGroupsListRouteSkeleton,
  beforeLoad: requireAuth(),
  loader: ({ context, cause }) => {
    const ownerId = context.workspaceId;
    if (ownerId == null) return;
    return context.client.query({
      query: SandboxGroupsDocument,
      variables: { ownerId },
      fetchPolicy: titleLoaderFetchPolicy(cause),
      errorPolicy: "all",
    });
  },
  head: ({ match }) => translatedTitleHead("sandboxGroups.pageTitle", match),
});

function SandboxGroupsPage() {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const { canCreate } = useCapabilities();
  const { groups, loading, error, refetch } = useSandboxGroups();
  const [createOpen, setCreateOpen] = useState(false);

  return (
    <DashboardLayout>
      <PageFrame
        header={
          <PageHeader
            title={t("sandboxGroups.pageTitle")}
            description={t("sandboxGroups.pageDescription")}
            actions={
              canCreate ? (
                <Button size="sm" onClick={() => setCreateOpen(true)}>
                  <Plus aria-hidden="true" />
                  {t("sandboxGroups.createTitle")}
                </Button>
              ) : null
            }
          />
        }
      >
        <NewSandboxGroupDialog
          key={currentWorkspaceId ?? "none"}
          open={createOpen}
          onOpenChange={(next) => {
            setCreateOpen(next);
            if (!next) void refetch();
          }}
        />

        {loading ? (
          <SandboxGroupsListContentSkeleton />
        ) : error && groups.length === 0 ? (
          <div className="flex flex-col items-center gap-2 py-12 text-center">
            <AlertTriangle
              className="text-muted-foreground size-6"
              aria-hidden="true"
            />
            <p className="text-muted-foreground text-sm">
              {t("sandboxGroups.listLoadError")}
            </p>
            <Button size="sm" variant="outline" onClick={() => void refetch()}>
              {t("common.tryAgain")}
            </Button>
          </div>
        ) : groups.length === 0 ? (
          <div className="flex flex-col items-center gap-2 rounded-lg border border-dashed py-12 text-center">
            <Box
              className="text-muted-foreground size-7"
              aria-hidden="true"
            />
            <p className="text-sm font-medium">
              {t("sandboxGroups.listEmptyTitle")}
            </p>
            <p className="text-muted-foreground max-w-md text-xs">
              {t("sandboxGroups.listEmptyDescription")}
            </p>
            {canCreate ? (
              <Button
                size="sm"
                variant="outline"
                className="mt-2"
                onClick={() => setCreateOpen(true)}
              >
                <Plus aria-hidden="true" />
                {t("sandboxGroups.createTitle")}
              </Button>
            ) : null}
          </div>
        ) : (
          <div className="space-y-4">
            {/* Desktop table */}
            <div className="hidden overflow-hidden rounded-lg border md:block">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="w-[26%]">
                      {t("sandboxGroups.fieldName")}
                    </TableHead>
                    <TableHead className="w-[24%]">
                      {t("sandboxGroups.fieldId")}
                    </TableHead>
                    <TableHead className="w-[16%]">
                      {t("sandboxGroups.fieldRegion")}
                    </TableHead>
                    <TableHead className="w-[18%]">
                      {t("sandboxGroups.fieldConcurrencyLimit")}
                    </TableHead>
                    <TableHead className="w-[16%] text-right">
                      {t("sandboxGroups.credColActions")}
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {groups.map((group) => (
                    <TableRow key={group.id}>
                      <TableCell className="py-3.5">
                        <div className="flex items-center gap-2">
                          <Link
                            to="/sandbox-groups/$sandboxGroupId"
                            params={{ sandboxGroupId: group.id }}
                            className="font-medium hover:underline"
                          >
                            {group.name}
                          </Link>
                          {group.isDefault ? (
                            <Badge variant="secondary">
                              {t("sandboxGroups.defaultBadge")}
                            </Badge>
                          ) : null}
                        </div>
                      </TableCell>
                      <TableCell className="py-3.5">
                        <div className="flex items-center gap-1.5 font-mono text-xs">
                          <span>{group.id}</span>
                          <CopyButton
                            value={group.id}
                            label={t("sandboxGroups.copyId")}
                            successText={t("sandboxGroups.copied")}
                            errorText={t("sandboxGroups.copyError")}
                          />
                        </div>
                      </TableCell>
                      <TableCell className="py-3.5 text-sm">
                        {group.region}
                      </TableCell>
                      <TableCell className="py-3.5 tabular-nums text-sm">
                        {group.concurrencyLimit}
                      </TableCell>
                      <TableCell className="py-3.5 text-right">
                        <Button asChild size="sm" variant="outline">
                          <Link
                            to="/sandbox-groups/$sandboxGroupId"
                            params={{ sandboxGroupId: group.id }}
                          >
                            {t("sandboxGroups.manageCredentialsButton")}
                          </Link>
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>

            {/* Narrow-mobile stacked cards (< md, 390px) */}
            <div className="space-y-3 md:hidden">
              {groups.map((group) => (
                <Card key={group.id} className="gap-3 py-4">
                  <CardContent className="space-y-3 px-4">
                    <div className="flex items-center justify-between gap-2">
                      <Link
                        to="/sandbox-groups/$sandboxGroupId"
                        params={{ sandboxGroupId: group.id }}
                        className="truncate font-medium hover:underline"
                      >
                        {group.name}
                      </Link>
                      {group.isDefault ? (
                        <Badge variant="secondary">
                          {t("sandboxGroups.defaultBadge")}
                        </Badge>
                      ) : null}
                    </div>
                    <div className="flex items-center gap-1.5 font-mono text-xs">
                      <span className="truncate">{group.id}</span>
                      <CopyButton
                        value={group.id}
                        label={t("sandboxGroups.copyId")}
                        successText={t("sandboxGroups.copied")}
                        errorText={t("sandboxGroups.copyError")}
                      />
                    </div>
                    <div className="flex flex-wrap items-center justify-between gap-2 pt-1 text-xs">
                      <span className="text-muted-foreground">
                        {group.region} ·{" "}
                        {t("sandboxGroups.fieldConcurrencyLimit")}:{" "}
                        {group.concurrencyLimit}
                      </span>
                      <Button asChild size="sm" variant="outline">
                        <Link
                          to="/sandbox-groups/$sandboxGroupId"
                          params={{ sandboxGroupId: group.id }}
                        >
                          {t("sandboxGroups.manageCredentialsButton")}
                        </Link>
                      </Button>
                    </div>
                  </CardContent>
                </Card>
              ))}
            </div>
          </div>
        )}
      </PageFrame>
    </DashboardLayout>
  );
}
