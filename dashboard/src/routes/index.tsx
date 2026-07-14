import { useEffect, useMemo, useState } from "react";
import { createFileRoute, Link } from "@tanstack/react-router";
import { Plus } from "lucide-react";
import { requireAuth } from "@/common/lib/auth/auth";
import { DashboardLayout } from "@/common/components/dashboard-layout";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card.tsx";
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
import { useServices } from "@/features/services/hooks/use-services";
import { useServiceLifecycle } from "@/features/services/hooks/use-service-lifecycle";
import { useDatabases } from "@/features/databases/hooks/use-databases";
import { computeStats as computeDatabaseStats } from "@/features/databases/lib/status";
import { CreateDatabaseDialog } from "@/features/databases/components/create-database-dialog";
import { useKeyValues } from "@/features/keyvalue/hooks/use-key-values";
import { computeStats as computeKeyValueStats } from "@/features/keyvalue/lib/status";
import { useProjects } from "@/features/projects/hooks/use-projects";
import { useGroupedResources } from "@/features/projects/hooks/use-grouped-resources";
import { ProjectSection } from "@/features/projects/components/project-section";
import { ResourceTable } from "@/features/projects/components/resource-table";
import { NewProjectDialog } from "@/features/projects/components/new-project-dialog";

export const Route = createFileRoute("/")({
  component: HomePage,
  beforeLoad: requireAuth("/"),
  head: () => ({
    meta: [{ title: "bex dashboard" }],
  }),
});

export function HomePage() {
  const { t } = useTranslations();
  const { services, loading: servicesLoading, error: servicesError, refetch: refetchServices } = useServices();
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
  const { projects, loading: projectsLoading, refetch: refetchProjects } = useProjects();
  const { pending, run } = useServiceLifecycle({ refetch: refetchServices });
  const [newDatabaseOpen, setNewDatabaseOpen] = useState(false);
  const [newProjectOpen, setNewProjectOpen] = useState(false);

  const databaseStats = useMemo(() => computeDatabaseStats(databases), [databases]);
  const keyValueStats = useMemo(() => computeKeyValueStats(keyValues), [keyValues]);

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

  const loading = servicesLoading || databasesLoading || keyValuesLoading || projectsLoading;
  const error = servicesError || databasesError || keyValuesError;
  const totalResources = services.length + databases.length + keyValues.length;

  // Only treat loading/error as page-level states while there's nothing to show;
  // a transient poll error or background refetch must not blank an existing list.
  const showSkeleton = loading && totalResources === 0;
  const showError = !loading && error && totalResources === 0;
  const showEmpty = !loading && !error && totalResources === 0;

  function refetchAll() {
    void refetchServices();
    void refetchDatabases();
    void refetchKeyValues();
    void refetchProjects();
  }

  return (
    <DashboardLayout>
      <div className="flex-1 overflow-auto p-4 sm:p-6">
        <div className="mx-auto w-full max-w-4xl space-y-6">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between gap-2 space-y-0">
              <CardTitle>{t("projects.cardTitle")}</CardTitle>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button size="sm">
                    <Plus className="size-4" />
                    {t("projects.newButton")}
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem asChild>
                    <Link to="/services/new">{t("projects.newService")}</Link>
                  </DropdownMenuItem>
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
            </CardHeader>
            <CardContent>
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
                <div className="space-y-4">
                  {groups.map((g) => (
                    <ProjectSection
                      key={g.id}
                      id={g.id}
                      name={g.name}
                      rows={g.rows}
                      loading={showSkeleton}
                      servicePending={pending}
                      onRunServiceAction={run}
                      onDatabaseDeleted={refetchAll}
                      onKeyValueDeleted={refetchAll}
                      onChanged={refetchAll}
                    />
                  ))}
                  {(ungrouped.length > 0 || groups.length === 0) && (
                    <div>
                      <p className="mb-1 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                        {t("projects.ungroupedLabel")}
                      </p>
                      <ResourceTable
                        rows={ungrouped}
                        loading={showSkeleton}
                        servicePending={pending}
                        onRunServiceAction={run}
                        onDatabaseDeleted={refetchAll}
                        onKeyValueDeleted={refetchAll}
                      />
                    </div>
                  )}
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      </div>
      <CreateDatabaseDialog
        open={newDatabaseOpen}
        onOpenChange={setNewDatabaseOpen}
        onCreated={() => void refetchDatabases()}
      />
      <NewProjectDialog
        open={newProjectOpen}
        onOpenChange={setNewProjectOpen}
        onCreated={() => void refetchProjects()}
      />
    </DashboardLayout>
  );
}
