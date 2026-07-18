import { useEffect, useMemo, useState } from "react";
import { Link } from "@tanstack/react-router";
import { Layers, Plus } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Skeleton } from "@/common/components/ui/skeleton";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/common/components/ui/select";
import { useTranslations } from "@/common/hooks/use-translations";
import { useEnvironments } from "@/features/environments/hooks/use-environments";
import { EnvironmentCard } from "@/features/environments/components/environment-card";
import { NewEnvironmentDialog } from "@/features/environments/components/new-environment-dialog";
import type { ServiceView } from "@/features/services/types";
import type {
  PendingLifecycle,
  RunServiceAction,
} from "@/features/services/hooks/use-service-lifecycle";
import type { DatabaseView } from "@/features/databases/types";
import type { KeyValueView } from "@/features/keyvalue/types";
import { useEnvGroups } from "@/features/env-groups/hooks/use-env-groups";
import type { ResourceRow } from "@/features/projects/types";
import { ResourceTable } from "@/features/projects/components/resource-table";
import { ProjectResourceFilters } from "@/features/projects/components/project-resource-filters";
import {
  countProjectResources,
  filterProjectResources,
  type ProjectResourceFilterState,
} from "@/features/projects/lib/resource-filter";

const UNASSIGNED_ENVIRONMENT = "unassigned";

export interface EnvironmentsPanelProps {
  projectId: string;
  services: ServiceView[];
  databases: DatabaseView[];
  keyValues: KeyValueView[];
  /** Every Project member, used to keep legacy project-only resources reachable. */
  projectRows?: ResourceRow[];
  servicePending: PendingLifecycle | null;
  onRunServiceAction: RunServiceAction;
  onDatabaseDeleted: (id: string) => void;
  onKeyValueDeleted: (id: string) => void;
  resourceFilter?: ProjectResourceFilterState;
  onResourceFilterChange?: (filter: ProjectResourceFilterState) => void;
}

/** One URL-selected Environment operating surface, plus an honest unassigned view. */
export function EnvironmentsPanel({
  projectId,
  services,
  databases,
  keyValues,
  projectRows = [],
  servicePending,
  onRunServiceAction,
  onDatabaseDeleted,
  onKeyValueDeleted,
  resourceFilter = { environmentId: null, query: "", kind: "all" },
  onResourceFilterChange = () => undefined,
}: EnvironmentsPanelProps) {
  const { t } = useTranslations();
  const { environments, loading, error } = useEnvironments(projectId);
  const { groups: envGroups } = useEnvGroups();
  const [newOpen, setNewOpen] = useState(false);

  const unassignedRows = useMemo(() => {
    const assigned = new Set<string>();
    for (const environment of environments) {
      for (const id of environment.serviceIds) assigned.add(`service:${id}`);
      for (const id of environment.databaseIds) assigned.add(`database:${id}`);
      for (const id of environment.keyValueIds) assigned.add(`keyvalue:${id}`);
    }
    return projectRows.filter((row) => !assigned.has(`${row.kind}:${row.id}`));
  }, [environments, projectRows]);

  const requestedId = resourceFilter.environmentId;
  const requestedEnvironment = environments.find(
    (env) => env.id === requestedId,
  );
  // Keep an explicitly requested Unassigned URL stable even if Environment
  // data wins the network race against the Project resource lists. Without
  // this, the transient empty `projectRows` canonicalizes a shareable
  // `?env=unassigned` URL to the first Environment before its rows arrive.
  const canShowUnassigned =
    unassignedRows.length > 0 || requestedId === UNASSIGNED_ENVIRONMENT;
  const selectedId = requestedEnvironment
    ? requestedEnvironment.id
    : requestedId === UNASSIGNED_ENVIRONMENT && canShowUnassigned
      ? UNASSIGNED_ENVIRONMENT
      : (environments[0]?.id ??
        (canShowUnassigned ? UNASSIGNED_ENVIRONMENT : null));
  const selectedEnvironment = environments.find((env) => env.id === selectedId);

  // Canonicalize missing/deleted ids once data arrives so copied URLs never
  // leave the browser showing a fallback that differs from its address bar.
  useEffect(() => {
    if (loading || error || selectedId === requestedId) return;
    onResourceFilterChange({
      environmentId: selectedId,
      query: resourceFilter.query,
      kind: resourceFilter.kind,
    });
  }, [
    error,
    loading,
    onResourceFilterChange,
    requestedId,
    resourceFilter.kind,
    resourceFilter.query,
    selectedId,
  ]);

  const selectedFilter: ProjectResourceFilterState = {
    ...resourceFilter,
    environmentId: selectedId,
  };
  const contextualEnvironmentId = selectedEnvironment?.id;

  return (
    <section className="space-y-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div className="space-y-2">
          <div className="flex items-center gap-2">
            <Layers className="size-4 text-muted-foreground" />
            <h2 className="text-lg font-semibold">
              {t("environments.heading")}
            </h2>
          </div>
          {selectedId ? (
            <Select
              value={selectedId}
              onValueChange={(environmentId) =>
                onResourceFilterChange({
                  environmentId,
                  query: "",
                  kind: "all",
                })
              }
            >
              <SelectTrigger
                className="w-64"
                aria-label={t("environments.selectorLabel")}
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {environments.map((environment) => (
                  <SelectItem key={environment.id} value={environment.id}>
                    {environment.name}
                  </SelectItem>
                ))}
                {canShowUnassigned ? (
                  <SelectItem value={UNASSIGNED_ENVIRONMENT}>
                    {t("environments.unassignedOption")}
                  </SelectItem>
                ) : null}
              </SelectContent>
            </Select>
          ) : null}
        </div>
        <div className="flex flex-wrap gap-2">
          <Button asChild size="sm">
            <Link
              to="/services/new"
              search={{
                projectId,
                ...(contextualEnvironmentId
                  ? { environmentId: contextualEnvironmentId }
                  : {}),
              }}
            >
              <Plus />
              {t("projects.newService")}
            </Link>
          </Button>
          <Button variant="outline" size="sm" onClick={() => setNewOpen(true)}>
            <Plus />
            {t("environments.newButton")}
          </Button>
        </div>
      </div>

      {loading ? (
        <Skeleton className="h-48 w-full rounded-xl" />
      ) : error ? (
        <p className="text-sm text-muted-foreground">
          {t("environments.errorBody")}
        </p>
      ) : selectedEnvironment ? (
        <EnvironmentCard
          key={selectedEnvironment.id}
          environment={selectedEnvironment}
          allEnvironments={environments}
          services={services}
          databases={databases}
          keyValues={keyValues}
          servicePending={servicePending}
          onRunServiceAction={onRunServiceAction}
          onDatabaseDeleted={onDatabaseDeleted}
          onKeyValueDeleted={onKeyValueDeleted}
          envGroups={envGroups}
          resourceFilter={selectedFilter}
          onResourceFilterChange={onResourceFilterChange}
        />
      ) : selectedId === UNASSIGNED_ENVIRONMENT ? (
        <UnassignedResources
          rows={unassignedRows}
          filter={selectedFilter}
          onFilterChange={onResourceFilterChange}
          servicePending={servicePending}
          onRunServiceAction={onRunServiceAction}
          onDatabaseDeleted={onDatabaseDeleted}
          onKeyValueDeleted={onKeyValueDeleted}
        />
      ) : (
        <Card>
          <CardContent className="py-8 text-center text-sm text-muted-foreground">
            {t("environments.emptyBody")}
          </CardContent>
        </Card>
      )}

      <NewEnvironmentDialog
        projectId={projectId}
        open={newOpen}
        onOpenChange={setNewOpen}
      />
    </section>
  );
}

function UnassignedResources({
  rows,
  filter,
  onFilterChange,
  servicePending,
  onRunServiceAction,
  onDatabaseDeleted,
  onKeyValueDeleted,
}: {
  rows: ResourceRow[];
  filter: ProjectResourceFilterState;
  onFilterChange: (filter: ProjectResourceFilterState) => void;
  servicePending: PendingLifecycle | null;
  onRunServiceAction: RunServiceAction;
  onDatabaseDeleted: (id: string) => void;
  onKeyValueDeleted: (id: string) => void;
}) {
  const { t } = useTranslations();
  const visibleRows = filterProjectResources(rows, filter);
  return (
    <Card>
      <CardHeader>
        <CardTitle>
          <h3>{t("environments.unassignedOption")}</h3>
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <ProjectResourceFilters
          environmentName={t("environments.unassignedOption")}
          filter={filter}
          counts={countProjectResources(rows)}
          onChange={onFilterChange}
        />
        {visibleRows.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            {rows.length === 0
              ? t("environments.cardEmpty")
              : t("projects.noMatchesBody")}
          </p>
        ) : (
          <ResourceTable
            rows={visibleRows}
            servicePending={servicePending}
            onRunServiceAction={onRunServiceAction}
            onDatabaseDeleted={onDatabaseDeleted}
            onKeyValueDeleted={onKeyValueDeleted}
            projectMetadata
          />
        )}
      </CardContent>
    </Card>
  );
}
