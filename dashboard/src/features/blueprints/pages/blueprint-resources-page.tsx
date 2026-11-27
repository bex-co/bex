import { useMemo } from "react";
import { useApolloClient } from "@apollo/client/react";
import { useTranslations } from "@/common/hooks/use-translations";
import { Card, CardContent } from "@/common/components/ui/card";
import { ResourceLoadError } from "@/common/components/resource-load-error";
import {
  BlueprintBuildSourcesFrame,
  BlueprintEnvGroupsFrame,
  BlueprintResourcesFrame,
  BlueprintResourceTableFrame,
} from "@/common/components/route-skeletons";
import { useBlueprintDetail } from "@/features/blueprints/lib/blueprint-detail";
import {
  blueprintResourceTables,
  blueprintResourcesShape,
  isBlueprintServiceType,
  type BlueprintResourceTables,
} from "@/features/blueprints/lib/resource-tables";
import { BlueprintErrorList } from "@/features/blueprints/components/blueprint-error-list";
import { BlueprintBuildSourcesTable } from "@/features/blueprints/components/blueprint-build-sources-table";
import { groupBuildSources } from "@/features/blueprints/lib/build-source-groups";
import { useBlueprintSyncs } from "@/features/blueprints/hooks/use-blueprint-syncs";
import { useBlueprintSync } from "@/features/blueprints/hooks/use-blueprint-sync";
import type {
  BlueprintResource,
  BlueprintValidationError,
} from "@/features/blueprints/types";
import { ResourceTable } from "@/features/projects/components/resource-table";
import { BulkResourceToolbar } from "@/features/projects/components/bulk-resource-toolbar";
import { useResourceSelection } from "@/features/projects/hooks/use-resource-selection";
import { useProjects } from "@/features/projects/hooks/use-projects";
import {
  toDatabaseRow,
  toKeyValueRow,
  toServiceRow,
} from "@/features/projects/hooks/use-grouped-resources";
import { MoveDisabledContext } from "@/features/projects/lib/move-disabled";
import type { ResourceRow } from "@/features/projects/types";
import { useServices } from "@/features/services/hooks/use-services";
import { useServiceLifecycle } from "@/features/services/hooks/use-service-lifecycle";
import { useDatabases } from "@/features/databases/hooks/use-databases";
import { useKeyValues } from "@/features/keyvalue/hooks/use-key-values";
import { useBuildSourcesList } from "@/features/build-sources/hooks/use-build-sources-list";
import { useEnvGroups } from "@/features/env-groups/hooks/use-env-groups";
import { useEnvGroupScopeIndex } from "@/features/env-groups/hooks/use-env-group-scope-index";
import { EnvGroupsTable } from "@/features/env-groups/components/env-groups-table";

/**
 * A Blueprint's Resources page, Render's bare `/blueprint/<id>` and
 * `/resources` (w3/m677), drawn as Render's is (w3/m680): its services,
 * Postgres and Key Value in the Overview's resource table, with each row's
 * menu and the selection toolbar, neither offering Move; its build sources in
 * the Build Sources table; its environment groups in the Environment Groups
 * table; each table only when it has rows. A Blueprint managing nothing reads
 * Render's "No resources managed by this Blueprint", and one whose newest
 * sync failed on its file lists the file's problems instead, as Render's page
 * does while its file does not validate.
 */
export function BlueprintResourcesPage() {
  const { blueprint } = useBlueprintDetail();
  const tables = useMemo(
    () => blueprintResourceTables(blueprint.resources),
    [blueprint.resources],
  );
  const { syncs, loading } = useBlueprintSyncs(blueprint.id);
  const newest = syncs.at(0);

  if (loading && !newest) {
    return <BlueprintResourcesFrame shape={blueprintResourcesShape(tables)} />;
  }
  if (newest?.state === "error") {
    return (
      <FailedSyncResources
        blueprintId={blueprint.id}
        syncId={newest.id}
        tables={tables}
      />
    );
  }
  return <BlueprintResourceTables tables={tables} />;
}

/**
 * The page while the newest sync is a failed one: that sync's errors under
 * Render's sentence when they are the file's (located at a line or a field),
 * else the tables, as a sync that failed after planning (a deploy, a busy
 * Blueprint) leaves the file valid.
 */
function FailedSyncResources({
  blueprintId,
  syncId,
  tables,
}: {
  blueprintId: string;
  syncId: string;
  tables: BlueprintResourceTables;
}) {
  const { sync, loading } = useBlueprintSync(blueprintId, syncId);
  if (!sync && loading) {
    return <BlueprintResourcesFrame shape={blueprintResourcesShape(tables)} />;
  }
  const errors = sync?.errors ?? [];
  if (errors.some(isFileError))
    return <BlueprintFileProblems errors={errors} />;
  return <BlueprintResourceTables tables={tables} />;
}

/** An error the Blueprint file has: one bex-api placed at a line or a field. */
function isFileError(error: BlueprintValidationError): boolean {
  return error.line > 0 || error.path !== "";
}

/**
 * Render's "There was a problem syncing with your Blueprint file:" ("were a
 * few problems" for several) over each located error.
 */
function BlueprintFileProblems({
  errors,
}: {
  errors: BlueprintValidationError[];
}) {
  const { t } = useTranslations();
  return (
    <Card data-testid="blueprint-file-problems">
      <CardContent className="space-y-3 text-sm">
        <p className="text-muted-foreground">
          {t("blueprints.resourcesFileProblems", { count: errors.length })}
        </p>
        <BlueprintErrorList
          className="text-destructive"
          errors={errors.map((e) => e.error)}
          details={errors}
        />
      </CardContent>
    </Card>
  );
}

/** The page's tables, each only when it has rows, or Render's empty card. */
function BlueprintResourceTables({
  tables,
}: {
  tables: BlueprintResourceTables;
}) {
  const { t } = useTranslations();
  if (
    tables.resources.length === 0 &&
    tables.buildSources.length === 0 &&
    tables.envGroups.length === 0
  ) {
    return (
      <Card data-testid="blueprint-resources-empty">
        <CardContent className="text-sm text-muted-foreground">
          {t("blueprints.resourcesEmpty")}
        </CardContent>
      </Card>
    );
  }
  return (
    // Render's disableMove: a Blueprint's resources are not moved from here.
    <MoveDisabledContext.Provider value>
      <div className="space-y-8">
        {tables.resources.length > 0 ? (
          <BlueprintServicesTable resources={tables.resources} />
        ) : null}
        {tables.buildSources.length > 0 ? (
          <BlueprintBuildSources resources={tables.buildSources} />
        ) : null}
        {tables.envGroups.length > 0 ? (
          <BlueprintEnvGroups resources={tables.envGroups} />
        ) : null}
      </div>
    </MoveDisabledContext.Provider>
  );
}

/** The ids of `resources`, which pick a Blueprint's rows out of a list. */
function idsOf(resources: readonly BlueprintResource[]): Set<string> {
  return new Set(resources.map((resource) => resource.id));
}

/**
 * The Blueprint's services, Postgres and Key Value, in the file's order, in
 * the resource table the Overview uses (`projectMetadata`: status, runtime,
 * region, updated), read from the workspace's own lists so each row is the
 * Overview's, with its live status and menu. A resource a list lacks is gone
 * since the Blueprint was read, and is left out.
 */
function BlueprintServicesTable({
  resources,
}: {
  resources: BlueprintResource[];
}) {
  const client = useApolloClient();
  const services = useServices();
  const databases = useDatabases();
  const keyValues = useKeyValues();
  const { pending, run } = useServiceLifecycle({
    refetch: services.refetch,
  });
  const reads = [
    resources.some((r) => isBlueprintServiceType(r.type)) && services,
    resources.some((r) => r.type === "postgres") && databases,
    resources.some((r) => r.type === "key_value") && keyValues,
  ].filter((read) => read !== false);

  const rows = useMemo((): ResourceRow[] => {
    const byId = new Map<string, ResourceRow>([
      ...services.services.map((s) => [s.id, toServiceRow(s)] as const),
      ...databases.databases.map((d) => [d.id, toDatabaseRow(d)] as const),
      ...keyValues.keyValues.map((k) => [k.id, toKeyValueRow(k)] as const),
    ]);
    return resources.flatMap((resource) => byId.get(resource.id) ?? []);
  }, [resources, services.services, databases.databases, keyValues.keyValues]);
  const selection = useResourceSelection(rows);

  const unread = reads.find((read) => read.error && !read.loading);
  if (reads.some((read) => read.loading) && rows.length < resources.length) {
    return <BlueprintResourceTableFrame rows={resources.length} />;
  }
  if (unread && rows.length < resources.length) {
    return <ResourceLoadError onRetry={() => void unread.refetch()} />;
  }
  if (rows.length === 0) return null;

  // A deleted datastore leaves the Blueprint's resources too.
  const deleted = (refetch: () => Promise<unknown>) => () => {
    void refetch();
    void client.refetchQueries({ include: ["Blueprint"] });
  };
  return (
    <div className="space-y-3">
      <BulkResourceToolbar selection={selection} />
      <ResourceTable
        rows={rows}
        projectMetadata
        servicePending={pending}
        onRunServiceAction={run}
        onDatabaseDeleted={deleted(databases.refetch)}
        onKeyValueDeleted={deleted(keyValues.refetch)}
        selectedKeys={selection.selected}
        onSelectedKeysChange={selection.setSelected}
      />
    </div>
  );
}

/** The Blueprint's build sources in Render's Build Sources table. */
function BlueprintBuildSources({
  resources,
}: {
  resources: BlueprintResource[];
}) {
  const { buildSources, loading, error, refetch } = useBuildSourcesList();
  const { projects } = useProjects({ poll: false });
  const groups = useMemo(() => {
    const ids = idsOf(resources);
    const names = new Map(projects.map((p) => [p.id, p.name]));
    return groupBuildSources(
      buildSources.filter((source) => ids.has(source.id)),
      (projectId) => names.get(projectId),
    );
  }, [resources, buildSources, projects]);

  if (buildSources.length === 0 && loading) {
    return <BlueprintBuildSourcesFrame rows={resources.length} />;
  }
  if (buildSources.length === 0 && error) {
    return <ResourceLoadError onRetry={() => void refetch()} />;
  }
  if (groups.length === 0) return null;
  return <BlueprintBuildSourcesTable groups={groups} />;
}

/** The Blueprint's environment groups in Render's Env Groups table. */
function BlueprintEnvGroups({ resources }: { resources: BlueprintResource[] }) {
  const { groups, loading, error, refetch } = useEnvGroups();
  const scope = useEnvGroupScopeIndex();
  const managed = useMemo(() => {
    const ids = idsOf(resources);
    return groups.filter((group) => ids.has(group.id));
  }, [resources, groups]);

  if (groups.length === 0 && loading) {
    return <BlueprintEnvGroupsFrame rows={resources.length} />;
  }
  if (groups.length === 0 && error) {
    return <ResourceLoadError onRetry={() => void refetch()} />;
  }
  if (managed.length === 0) return null;
  return <EnvGroupsTable groups={managed} scope={scope} />;
}
