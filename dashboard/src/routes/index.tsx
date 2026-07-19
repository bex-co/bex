import { useEffect, useMemo } from "react";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { translatedTitleHead } from "@/common/lib/document-head";
import { Plus } from "lucide-react";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { useTranslations } from "@/common/hooks/use-translations";
import { Button } from "@/common/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu.tsx";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/common/components/ui/alert.tsx";
import { NewServiceMenuItems } from "@/features/services/components/new-service-menu-items";
import { useServices } from "@/features/services/hooks/use-services";
import { useServiceLifecycle } from "@/features/services/hooks/use-service-lifecycle";
import { useDatabases } from "@/features/databases/hooks/use-databases";
import { computeStats as computeDatabaseStats } from "@/features/databases/lib/status";
import { CreateDatabaseDialog } from "@/features/databases/components/create-database-dialog";
import { useKeyValues } from "@/features/keyvalue/hooks/use-key-values";
import { computeStats as computeKeyValueStats } from "@/features/keyvalue/lib/status";
import { useProjects } from "@/features/projects/hooks/use-projects";
import { useGroupedResources } from "@/features/projects/hooks/use-grouped-resources";
import { ResourceTable } from "@/features/projects/components/resource-table";
import { NewProjectDialog } from "@/features/projects/components/new-project-dialog";
import { ProjectCard } from "@/features/projects/components/project-card";
import { NewProjectCard } from "@/features/projects/components/new-project-card";

export const Route = createFileRoute("/")({
  component: HomePage,
  // No-arg requireAuth: `next` defaults to the requested href, so
  // `?new=database` — the create-dialog param Render's New-menu aliases land
  // with (w1/m45) — survives the SSR login bounce.
  beforeLoad: requireAuth(),
  // `?new=database|project` opens the matching create dialog (w1/m45) — the
  // URL owns the dialog state (the `?plan=change` pattern,
  // routes/workspace.settings.tsx) so Render's New-menu URLs (`/new/database`,
  // `/d/new`, `/new/project`) can alias onto it. Optional key, same rationale
  // as workspace.settings' validateSearch.
  validateSearch: (
    search: Record<string, unknown>,
  ): { new?: "database" | "project" } =>
    search.new === "database" || search.new === "project"
      ? { new: search.new }
      : {},
  head: ({ match }) => {
    if (match.search.new === "database") {
      return translatedTitleHead("databases.createTitle", match);
    }
    if (match.search.new === "project") {
      return translatedTitleHead("projects.createTitle", match);
    }
    return translatedTitleHead("projects.overviewTitle", match);
  },
});

export function HomePage() {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const {
    services,
    loading: servicesLoading,
    error: servicesError,
    refetch: refetchServices,
  } = useServices();
  const {
    databases,
    loading: databasesLoading,
    error: databasesError,
    refetch: refetchDatabases,
    startPolling: startDatabasePolling,
    stopPolling: stopDatabasePolling,
  } = useDatabases();
  const {
    keyValues,
    loading: keyValuesLoading,
    error: keyValuesError,
    refetch: refetchKeyValues,
    startPolling: startKeyValuePolling,
    stopPolling: stopKeyValuePolling,
  } = useKeyValues();
  const {
    projects,
    loading: projectsLoading,
    refetch: refetchProjects,
  } = useProjects();
  const { pending, run } = useServiceLifecycle({ refetch: refetchServices });
  // The URL owns which create dialog is open (`?new=…`), so Render's New-menu
  // aliases land directly in the dialog; `replace` keeps open/close out of the
  // back-button history (the workspace.settings `?plan=change` idiom).
  const { new: newParam } = Route.useSearch();
  const routeNavigate = Route.useNavigate();
  const newDatabaseOpen = newParam === "database";
  const newProjectOpen = newParam === "project";
  function setCreateDialog(kind: "database" | "project", open: boolean) {
    void routeNavigate({
      search: open ? { new: kind } : {},
      replace: true,
    });
  }
  const setNewDatabaseOpen = (open: boolean) =>
    setCreateDialog("database", open);
  const setNewProjectOpen = (open: boolean) => setCreateDialog("project", open);

  const databaseStats = useMemo(
    () => computeDatabaseStats(databases),
    [databases],
  );
  const keyValueStats = useMemo(
    () => computeKeyValueStats(keyValues),
    [keyValues],
  );

  // Poll while a just-created database/key-value is still provisioning, so it
  // converges to Available on its own.
  useEffect(() => {
    if (databaseStats.creating > 0) startDatabasePolling(3000);
    else stopDatabasePolling();
    return () => stopDatabasePolling();
  }, [databaseStats.creating, startDatabasePolling, stopDatabasePolling]);
  useEffect(() => {
    if (keyValueStats.creating > 0) startKeyValuePolling(3000);
    else stopKeyValuePolling();
    return () => stopKeyValuePolling();
  }, [keyValueStats.creating, startKeyValuePolling, stopKeyValuePolling]);

  const { groups, ungrouped } = useGroupedResources({
    projects,
    services,
    databases,
    keyValues,
  });

  const loading =
    servicesLoading || databasesLoading || keyValuesLoading || projectsLoading;
  const error = servicesError || databasesError || keyValuesError;
  const totalResources = services.length + databases.length + keyValues.length;

  // Only treat loading/error as page-level states while there's nothing to show;
  // a transient poll error or background refetch must not blank an existing list.
  const showSkeleton = loading && totalResources === 0;
  const showError = !loading && error && totalResources === 0;
  const showEmpty =
    !loading && !error && totalResources === 0 && projects.length === 0;

  function refetchAll() {
    void refetchServices();
    void refetchDatabases();
    void refetchKeyValues();
    void refetchProjects();
  }

  return (
    <DashboardLayout>
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="w-full space-y-6">
          <div className="flex flex-row items-center justify-between gap-2">
            <h1 className="text-xl font-semibold">
              {t("projects.overviewTitle")}
            </h1>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button size="sm">
                  <Plus className="size-4" />
                  {t("projects.newButton")}
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <NewServiceMenuItems triggerLabelKey="projects.newService" />
                <DropdownMenuItem onSelect={() => setNewDatabaseOpen(true)}>
                  {t("projects.newDatabase")}
                </DropdownMenuItem>
                <DropdownMenuItem asChild>
                  <Link to="/keyvalue/new">{t("projects.newKeyValue")}</Link>
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem onSelect={() => setNewProjectOpen(true)}>
                  {t("projects.newProjectButton")}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>

          {showError ? (
            <Alert variant="destructive">
              <AlertTitle>{t("projects.errorTitle")}</AlertTitle>
              <AlertDescription>{t("projects.errorBody")}</AlertDescription>
            </Alert>
          ) : showEmpty ? (
            <div className="py-10 text-center">
              <p className="font-medium">{t("projects.emptyTitle")}</p>
              <p className="text-sm text-muted-foreground">
                {t("projects.emptyBody")}
              </p>
            </div>
          ) : (
            <>
              <section className="space-y-3">
                <h2 className="text-sm font-semibold">
                  {t("projects.projectsHeading")}
                </h2>
                <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
                  {groups.map((g) => (
                    <ProjectCard
                      key={g.id}
                      id={g.id}
                      name={g.name}
                      rows={g.rows}
                    />
                  ))}
                  <NewProjectCard onClick={() => setNewProjectOpen(true)} />
                </div>
              </section>

              {(ungrouped.length > 0 || showSkeleton) && (
                <section className="space-y-3">
                  <h2 className="text-sm font-semibold">
                    {t("projects.ungroupedHeading")}
                  </h2>
                  <ResourceTable
                    rows={ungrouped}
                    loading={showSkeleton}
                    servicePending={pending}
                    onRunServiceAction={run}
                    onDatabaseDeleted={refetchAll}
                    onKeyValueDeleted={refetchAll}
                  />
                </section>
              )}
            </>
          )}
        </div>
      </div>
      <CreateDatabaseDialog
        open={newDatabaseOpen}
        onOpenChange={setNewDatabaseOpen}
        // refetchAll, not just the databases list: a create with a project or
        // environment assignment also changes the Projects query this page's
        // grouping is built from.
        onCreated={(databaseId) => {
          refetchAll();
          return navigate({
            to: "/databases/$databaseId",
            params: { databaseId },
          });
        }}
      />
      <NewProjectDialog
        open={newProjectOpen}
        onOpenChange={setNewProjectOpen}
        onCreated={(id) => {
          void refetchProjects();
          return navigate({
            to: "/project/$projectId",
            params: { projectId: id },
          });
        }}
      />
    </DashboardLayout>
  );
}
