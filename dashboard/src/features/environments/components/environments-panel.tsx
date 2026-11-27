import { useId, useMemo, useState } from "react";
import { Plus } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { Card, CardContent } from "@/common/components/ui/card";
import { ProjectEnvironmentSectionSkeleton } from "@/common/components/route-skeletons";
import { useTranslations } from "@/common/hooks/use-translations";
import { useEnvironments } from "@/features/environments/hooks/use-environments";
import { EnvironmentSection } from "@/features/environments/components/environment-section";
import { NoMatchingResources } from "@/features/environments/components/no-matching-resources";
import { EnvironmentSectionHeader } from "@/features/environments/components/environment-section-header";
import type { ServiceView } from "@/features/services/types";
import type {
  PendingLifecycle,
  RunServiceAction,
} from "@/features/services/hooks/use-service-lifecycle";
import type { DatabaseView } from "@/features/databases/types";
import type { KeyValueView } from "@/features/keyvalue/types";
import { useEnvGroups } from "@/features/env-groups/hooks/use-env-groups";
import { useSandboxGroups } from "@/features/sandbox-groups/hooks/use-sandbox-groups";
import type { ResourceRow } from "@/features/projects/types";
import { ResourceTable } from "@/features/projects/components/resource-table";
import {
  EnvironmentResourceSearch,
  EnvironmentResourceSwitcher,
} from "@/features/projects/components/project-resource-filters";
import {
  countProjectResources,
  countsWithUnknown,
  environmentFilterCounts,
  matchesResourceQuery,
  type ProjectResourceCatalog,
  type ProjectResourceFamily,
} from "@/features/projects/lib/resource-filter";
import { useSectionFilter } from "@/features/environments/lib/section-filter";
import { useCatalogRetry } from "@/common/hooks/use-catalog-retry";
import { UnreadResourcesNotice } from "@/features/environments/components/unread-resources-notice";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import {
  unreadFamilies,
  type UnreadResources,
} from "@/features/environments/lib/unread-resources";
import { inEnvironment } from "@/features/environments/lib/link-id";
import { cn } from "@/common/lib/utils/utils";
import {
  focusedSectionClass,
  useFocusedSection,
} from "@/features/environments/hooks/use-focused-section";
import { UNASSIGNED_SECTION } from "@/features/environments/lib/sections";

export interface EnvironmentsPanelProps {
  projectId: string;
  /** The project's name, which each section's ••• menu names. */
  projectName: string;
  services: ServiceView[];
  databases: DatabaseView[];
  keyValues: KeyValueView[];
  /** Every Project member, used to keep legacy project-only resources reachable. */
  projectRows?: ResourceRow[];
  /** Which of the service, database and key value lists have answered (w4/m227); the panel adds Env Groups. */
  catalog?: ProjectResourceCatalog;
  servicePending: PendingLifecycle | null;
  onRunServiceAction: RunServiceAction;
  onDatabaseDeleted: (id: string) => void;
  onKeyValueDeleted: (id: string) => void;
  /** The section Render's environment URL names, outlined while set. */
  focusedId?: string | null;
  /** Opens the new-environment dialog, from the Add environment card. */
  onAddEnvironment?: () => void;
  /** Why Add environment is unavailable to this caller, if it is. */
  addEnvironmentReason?: string;
}

/**
 * Every environment of a project, each as its own section in the project's
 * order, as Render's project page stacks them (w1/m682); a project with one
 * environment shows Render's Add environment card under it. bex's
 * project-only resources, which Render has no counterpart for, keep a
 * section of their own after the environments while it has rows, so none
 * becomes unreachable.
 */
export function EnvironmentsPanel({
  projectId,
  projectName,
  services,
  databases,
  keyValues,
  projectRows = [],
  servicePending,
  onRunServiceAction,
  onDatabaseDeleted,
  onKeyValueDeleted,
  catalog,
  focusedId = null,
  onAddEnvironment = () => undefined,
  addEnvironmentReason,
}: EnvironmentsPanelProps) {
  const { t } = useTranslations();
  const { environments, loading, error } = useEnvironments(projectId);
  const {
    groups: envGroups,
    loading: envGroupsLoading,
    error: envGroupsError,
    refetch: refetchEnvGroups,
  } = useEnvGroups();
  // The environments' sandbox groups (w22/m603). Not a catalog family: a
  // bex-api without sandboxes refuses the query, and its groups are none, so
  // a section reads exactly as it does without them.
  const { groups: sandboxGroups } = useSandboxGroups();
  const base = catalog ?? NO_CATALOG;
  const { retry, retrying } = useCatalogRetry(() =>
    Promise.all([base.retry(), envGroupsError ? refetchEnvGroups() : null]),
  );
  const pendingFamilies: ProjectResourceFamily[] = envGroupsLoading
    ? [...base.pending, "envgroups"]
    : base.pending;
  const failedFamilies: ProjectResourceFamily[] = envGroupsError
    ? [...base.failed, "envgroups"]
    : base.failed;
  const unread = {
    pending: pendingFamilies,
    failed: failedFamilies,
    onRetry: () => void retry(),
    retrying,
  };

  const unassignedRows = useMemo(() => {
    const assigned = new Set<string>();
    for (const environment of environments) {
      for (const id of environment.serviceIds) assigned.add(`service:${id}`);
      for (const id of environment.databaseIds) assigned.add(`database:${id}`);
      for (const id of environment.keyValueIds) assigned.add(`keyvalue:${id}`);
    }
    return projectRows.filter((row) => !assigned.has(`${row.kind}:${row.id}`));
  }, [environments, projectRows]);
  // Unassigned rows come from the Project's members joined with the service,
  // database and key value lists (Env Groups never appear there), so while
  // any of those is unread its rows are unknown (w4/m227). A failed list may
  // hide some, so the section stays to say so.
  const unassignedUnknown = unreadFamilies(unread).filter(
    (family) => family !== "envgroups",
  );
  const showUnassigned =
    unassignedRows.length > 0 ||
    unread.failed.some((family) => family !== "envgroups");

  if (loading) return <ProjectEnvironmentSectionSkeleton />;
  if (error && environments.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">
        {t("environments.errorBody")}
      </p>
    );
  }

  return (
    <div className="space-y-16">
      {/* A failed refresh over data already shown (errorPolicy "all" keeps it)
          reads inline: replacing the sections would unmount their open
          dialogs and drafts, the same loss a poll skeleton caused (w1/m153). */}
      {error && environments.length > 0 ? (
        <p role="alert" className="text-sm text-muted-foreground">
          {t("environments.refreshErrorBody")}
        </p>
      ) : null}
      {environments.length === 0 ? (
        <Card>
          <CardContent className="space-y-4 py-6 text-center text-sm text-muted-foreground">
            <p>{t("environments.emptyBody")}</p>
            {/* No Environment yet, but unread lists may hide Unassigned
                members (w4/m227). */}
            <UnreadResourcesNotice
              unread={unread}
              families={unassignedUnknown}
            />
          </CardContent>
        </Card>
      ) : null}
      {environments.map((environment) => (
        <EnvironmentSection
          key={environment.id}
          environment={environment}
          projectName={projectName}
          allEnvironments={environments}
          services={services}
          databases={databases}
          keyValues={keyValues}
          servicePending={servicePending}
          onRunServiceAction={onRunServiceAction}
          onDatabaseDeleted={onDatabaseDeleted}
          onKeyValueDeleted={onKeyValueDeleted}
          envGroups={envGroups}
          sandboxGroups={sandboxGroups}
          unread={unread}
          focused={inEnvironment(focusedId, environment.id)}
        />
      ))}
      {environments.length <= 1 ? (
        <AddEnvironmentCard
          onClick={onAddEnvironment}
          reason={addEnvironmentReason}
        />
      ) : null}
      {showUnassigned ? (
        <UnassignedSection
          rows={unassignedRows}
          servicePending={servicePending}
          onRunServiceAction={onRunServiceAction}
          onDatabaseDeleted={onDatabaseDeleted}
          onKeyValueDeleted={onKeyValueDeleted}
          unread={unread}
          unknown={unassignedUnknown}
          focused={focusedId === UNASSIGNED_SECTION}
        />
      ) : null}
    </div>
  );
}

/**
 * Render's card under a project's only environment: a wide dashed button
 * that opens the new-environment dialog.
 */
function AddEnvironmentCard({
  onClick,
  reason,
}: {
  onClick: () => void;
  reason?: string;
}) {
  const { t } = useTranslations();
  return (
    <PermissionTooltip reason={reason}>
      <Button
        variant="outline"
        className="h-auto w-full border-dashed py-16"
        disabled={reason !== undefined}
        onClick={onClick}
      >
        <Plus aria-hidden="true" />
        {t("projects.addEnvironment")}
      </Button>
    </PermissionTooltip>
  );
}

function UnassignedSection({
  rows,
  servicePending,
  onRunServiceAction,
  onDatabaseDeleted,
  onKeyValueDeleted,
  unread,
  unknown,
  focused,
}: {
  rows: ResourceRow[];
  servicePending: PendingLifecycle | null;
  onRunServiceAction: RunServiceAction;
  onDatabaseDeleted: (id: string) => void;
  onKeyValueDeleted: (id: string) => void;
  unread: UnreadResources;
  /** The families its rows depend on that have not answered. */
  unknown: ProjectResourceFamily[];
  focused: boolean;
}) {
  const { t } = useTranslations();
  const titleId = useId();
  const name = t("environments.unassignedOption");
  const sectionRef = useFocusedSection(focused, rows.length);
  const [filter, setFilter] = useSectionFilter(UNASSIGNED_SECTION);
  const [query, setQuery] = useState("");
  const counts = environmentFilterCounts(
    countsWithUnknown(countProjectResources(rows), unknown),
  );
  // Project-only rows are services, Postgres and Key Value: never env groups.
  const visibleRows =
    filter === "envgroups"
      ? []
      : rows.filter((row) => matchesResourceQuery(row, query));
  return (
    <section
      aria-labelledby={titleId}
      ref={sectionRef}
      data-focused={focused ? "" : undefined}
      className={cn(focusedSectionClass(focused), "space-y-4")}
    >
      <EnvironmentSectionHeader
        titleId={titleId}
        title={<span className="min-w-0 truncate leading-snug">{name}</span>}
        description={
          <p className="text-sm text-muted-foreground">
            {t("environments.unassignedDescription")}
          </p>
        }
        switcher={
          <EnvironmentResourceSwitcher
            filter={filter}
            counts={counts}
            onChange={setFilter}
          />
        }
        actions={null}
      />
      <EnvironmentResourceSearch
        environmentName={name}
        query={query}
        onChange={setQuery}
      />
      <UnreadResourcesNotice unread={unread} families={unknown} />
      {visibleRows.length > 0 ? (
        <ResourceTable
          rows={visibleRows}
          servicePending={servicePending}
          onRunServiceAction={onRunServiceAction}
          onDatabaseDeleted={onDatabaseDeleted}
          onKeyValueDeleted={onKeyValueDeleted}
          projectMetadata
        />
      ) : unknown.length > 0 ? null : rows.length > 0 &&
        filter !== "envgroups" ? (
        <NoMatchingResources />
      ) : (
        <p className="py-6 text-center text-sm text-muted-foreground">
          {t(
            filter === "envgroups"
              ? "environments.noEnvGroups"
              : "environments.cardEmpty",
          )}
        </p>
      )}
    </section>
  );
}

const NO_CATALOG: ProjectResourceCatalog = {
  pending: [],
  failed: [],
  retry: async () => undefined,
};
