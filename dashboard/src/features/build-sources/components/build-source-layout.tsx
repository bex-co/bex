import { useState } from "react";
import { Link, Outlet, useRouterState } from "@tanstack/react-router";
import { ArrowLeft, Box, FolderKanban, GitBranch, Play } from "lucide-react";
import { CopyButton } from "@/common/components/copy-button";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { PageFrame, PageHeader } from "@/common/components/page-frame";
import {
  AccessDenied,
  ResourceStatePage,
  ResourceStateView,
} from "@/common/components/resource-not-found";
import { Badge } from "@/common/components/ui/badge";
import { Button } from "@/common/components/ui/button";
import { resourceReadState } from "@/common/lib/resource-read-state";
import { useTranslations } from "@/common/hooks/use-translations";
import { regionLabel } from "@/common/lib/region-label";
import { cn } from "@/common/lib/utils/utils";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { useTriggerBuildGate } from "@/features/build-sources/hooks/use-trigger-build-gate";
import { useProjects } from "@/features/projects/hooks/use-projects";
import { formatRepoLabel } from "@/features/services/lib/repo";
import { useAdoptResourceWorkspace } from "@/features/workspaces/hooks/use-adopt-resource-workspace";
import { BuildSourceDetailContext } from "@/features/build-sources/components/build-source-detail-context";
import { BuildSourceDetailRouteSkeleton } from "@/features/build-sources/components/build-source-skeletons";
import { TriggerBuildDialog } from "@/features/build-sources/components/trigger-build-dialog";
import { useBuildSource } from "@/features/build-sources/hooks/use-build-source";

export interface BuildSourceLayoutProps {
  buildSourceId: string;
}

/**
 * Shared detail shell for `/build-sources/$buildSourceId` (`w2/m614/t001`):
 * header with source kind/runtime/region metadata, copyable `bds-…` ID,
 * primary Trigger Build CTA (git sources only, `can_create`-gated), and
 * tab navigation (`Builds`, `Services`, `Environment` [git only], `Settings`).
 */
export function BuildSourceLayout({ buildSourceId }: BuildSourceLayoutProps) {
  const { t } = useTranslations();
  const {
    buildSource: loaded,
    loading,
    error,
    refetch,
  } = useBuildSource(buildSourceId);
  const adopting = useAdoptResourceWorkspace(buildSourceId, loaded?.ownerId);
  const buildSource = adopting ? null : loaded;

  const triggerBuild = useTriggerBuildGate();
  const { projects } = useProjects({ poll: false });
  const [triggerOpen, setTriggerOpen] = useState(false);

  const pathname = useRouterState({
    select: (state) => (state.resolvedLocation ?? state.location).pathname,
  });

  // A dead id keeps its URL and draws Render's Page not found; a refused read
  // draws Access denied, which on Render's build source names no id; any
  // other failure draws "An error occurred" (w3/m706).
  const readState = adopting
    ? null
    : resourceReadState(loaded, loading, error);
  if (readState) {
    return (
      <ResourceStatePage>
        {readState === "forbidden" ? (
          <AccessDenied />
        ) : (
          <ResourceStateView state={readState} id={buildSourceId} />
        )}
      </ResourceStatePage>
    );
  }

  if (loading || adopting || !buildSource) {
    return <BuildSourceDetailRouteSkeleton />;
  }

  const isGit = buildSource.kind === "git";
  const region = buildSource.git?.region
    ? regionLabel(buildSource.git.region, t)
    : null;
  const project = buildSource.projectId
    ? projects.find((p) => p.id === buildSource.projectId)
    : undefined;
  const sourceSubtitle = isGit
    ? `${formatRepoLabel(buildSource.git?.repoUrl ?? "")}${
        buildSource.git?.branch ? ` · ${buildSource.git.branch}` : ""
      }`
    : (buildSource.image?.imageUrl ?? "");

  const basePath = `/build-sources/${buildSource.id}`;
  const isBuildsTab =
    pathname === basePath ||
    pathname === `${basePath}/` ||
    pathname.startsWith(`${basePath}/builds/`);

  const contextValue = {
    buildSource,
    refetch,
    openTriggerBuild: () => {
      if (isGit && triggerBuild.allowed) setTriggerOpen(true);
    },
  };

  return (
    <DashboardLayout>
      <PageFrame
        header={
          <div className="space-y-4">
            <PageHeader
              leading={
                <Button asChild size="icon-sm" variant="ghost">
                  <Link
                    to="/build-sources"
                    aria-label={t("buildSources.backToList")}
                  >
                    <ArrowLeft className="size-4" aria-hidden="true" />
                  </Link>
                </Button>
              }
              title={buildSource.name}
              titleAddon={
                <div className="flex flex-wrap items-center gap-1.5">
                  <Badge variant="secondary" className="gap-1">
                    {isGit ? (
                      <GitBranch className="size-3" aria-hidden="true" />
                    ) : (
                      <Box className="size-3" aria-hidden="true" />
                    )}
                    {t(
                      isGit
                        ? "buildSources.kindGit"
                        : "buildSources.kindImage",
                    )}
                  </Badge>
                  {isGit && buildSource.git?.runtime ? (
                    <Badge variant="outline" className="font-mono text-xs">
                      {buildSource.git.runtime}
                    </Badge>
                  ) : null}
                  {region ? <Badge variant="outline">{region}</Badge> : null}
                  {buildSource.projectId ? (
                    <Badge variant="outline" asChild className="gap-1">
                      <Link
                        to="/project/$projectId"
                        params={{ projectId: buildSource.projectId }}
                      >
                        <FolderKanban className="size-3" aria-hidden="true" />
                        {project?.name ?? buildSource.projectId}
                      </Link>
                    </Badge>
                  ) : null}
                </div>
              }
              description={
                <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs">
                  {sourceSubtitle ? (
                    <span
                      className={cn(
                        "text-foreground max-w-full truncate font-medium",
                        !isGit && "font-mono",
                      )}
                      title={
                        isGit
                          ? (buildSource.git?.repoUrl ?? "")
                          : (buildSource.image?.imageUrl ?? "")
                      }
                    >
                      {sourceSubtitle}
                    </span>
                  ) : null}
                  <span className="text-muted-foreground inline-flex items-center gap-1 font-mono">
                    <span>{buildSource.id}</span>
                    <CopyButton
                      value={buildSource.id}
                      label={t("buildSources.copyId")}
                      successText={t("buildSources.copied")}
                      errorText={t("buildSources.copyError")}
                    />
                  </span>
                </div>
              }
              actions={
                isGit ? (
                  <PermissionTooltip reason={triggerBuild.reason}>
                    <Button
                      size="sm"
                      disabled={!triggerBuild.allowed}
                      onClick={() => setTriggerOpen(true)}
                    >
                      <Play className="size-4" aria-hidden="true" />
                      {t("buildSources.triggerBuildButton")}
                    </Button>
                  </PermissionTooltip>
                ) : null
              }
            />

            <nav
              aria-label={t("buildSources.detailTabsLabel")}
              data-region="detail-tabs"
              className="flex gap-4 overflow-x-auto border-b"
            >
              <DetailTabLink
                to="/build-sources/$buildSourceId"
                buildSourceId={buildSource.id}
                active={isBuildsTab}
              >
                {t("buildSources.tabBuilds")}
              </DetailTabLink>
              <DetailTabLink
                to="/build-sources/$buildSourceId/services"
                buildSourceId={buildSource.id}
              >
                <span>{t("buildSources.tabServices")}</span>
                <Badge
                  variant="secondary"
                  className="ml-1.5 px-1.5 py-0 text-xs tabular-nums"
                >
                  {buildSource.serviceLinks.length}
                </Badge>
              </DetailTabLink>
              {isGit ? (
                <DetailTabLink
                  to="/build-sources/$buildSourceId/environment"
                  buildSourceId={buildSource.id}
                >
                  {t("buildSources.tabEnvironment")}
                </DetailTabLink>
              ) : null}
              <DetailTabLink
                to="/build-sources/$buildSourceId/settings"
                buildSourceId={buildSource.id}
              >
                {t("buildSources.tabSettings")}
              </DetailTabLink>
            </nav>
          </div>
        }
      >
        <BuildSourceDetailContext.Provider value={contextValue}>
          <Outlet />
        </BuildSourceDetailContext.Provider>
      </PageFrame>

      {isGit ? (
        <TriggerBuildDialog
          buildSource={buildSource}
          open={triggerOpen}
          onOpenChange={setTriggerOpen}
        />
      ) : null}
    </DashboardLayout>
  );
}

function DetailTabLink({
  to,
  buildSourceId,
  active,
  children,
}: {
  to:
    | "/build-sources/$buildSourceId"
    | "/build-sources/$buildSourceId/services"
    | "/build-sources/$buildSourceId/environment"
    | "/build-sources/$buildSourceId/settings";
  buildSourceId: string;
  active?: boolean;
  children: React.ReactNode;
}) {
  return (
    <Link
      to={to}
      params={{ buildSourceId }}
      activeOptions={{ exact: to === "/build-sources/$buildSourceId" }}
      className={cn(
        "text-muted-foreground hover:text-foreground -mb-px inline-flex shrink-0 items-center border-b-2 border-transparent px-1 py-2.5 text-sm font-medium transition-colors",
        active && "border-primary text-foreground",
      )}
      activeProps={{
        className: "border-primary text-foreground",
      }}
    >
      {children}
    </Link>
  );
}
