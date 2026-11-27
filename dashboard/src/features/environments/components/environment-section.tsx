import { useId, useMemo, useState } from "react";
import { Link } from "@tanstack/react-router";
import { ArrowRightLeft, LockKeyhole, Plus, Settings2 } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import { useTranslations } from "@/common/hooks/use-translations";
import { UnreadResourcesNotice } from "@/features/environments/components/unread-resources-notice";
import {
  NOTHING_UNREAD,
  environmentMemberIds,
  unreadFamilies,
  type UnreadResources,
} from "@/features/environments/lib/unread-resources";
import { ResourceTable } from "@/features/projects/components/resource-table";
import { BulkResourceToolbar } from "@/features/projects/components/bulk-resource-toolbar";
import { useResourceSelection } from "@/features/projects/hooks/use-resource-selection";
import {
  toDatabaseRow,
  toEnvGroupRow,
  toKeyValueRow,
  toSandboxGroupRow,
  toServiceRow,
} from "@/features/projects/hooks/use-grouped-resources";
import type { SandboxGroupItem } from "@/features/sandbox-groups/hooks/use-sandbox-groups";
import { inEnvironment } from "@/features/environments/lib/link-id";
import type { ResourceRow } from "@/features/projects/types";
import type { EnvironmentView } from "@/features/environments/hooks/use-environments";
import { ManageResourcesDialog } from "@/features/environments/components/manage-resources-dialog";
import { EnvironmentMenu } from "@/features/environments/components/environment-menu";
import { NoMatchingResources } from "@/features/environments/components/no-matching-resources";
import { EnvironmentSectionHeader } from "@/features/environments/components/environment-section-header";
import type { ServiceView } from "@/features/services/types";
import type {
  PendingLifecycle,
  RunServiceAction,
} from "@/features/services/hooks/use-service-lifecycle";
import type { DatabaseView } from "@/features/databases/types";
import type { KeyValueView } from "@/features/keyvalue/types";
import type { EnvGroupView } from "@/features/env-groups/types";
import {
  EnvironmentResourceSearch,
  EnvironmentResourceSwitcher,
} from "@/features/projects/components/project-resource-filters";
import {
  countProjectResources,
  countsWithUnknown,
  environmentFilterCounts,
  matchesResourceQuery,
} from "@/features/projects/lib/resource-filter";
import { useSectionFilter } from "@/features/environments/lib/section-filter";
import {
  focusedSectionClass,
  useFocusedSection,
} from "@/features/environments/hooks/use-focused-section";
import { useProtectedPlacement } from "@/features/environments/hooks/use-protected-placement";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { cn } from "@/common/lib/utils/utils";

export interface EnvironmentSectionProps {
  environment: EnvironmentView;
  /** The project's name, which the ••• menu's isolation dialog names. */
  projectName: string;
  /** All the workspace's services — used to resolve this env's rows and as assign candidates. */
  services: ServiceView[];
  /** All the workspace's databases — same role as `services` (w6/m20 extension). */
  databases: DatabaseView[];
  /** All the workspace's key-value instances — same role as `services` (w6/m20 extension). */
  keyValues: KeyValueView[];
  servicePending: PendingLifecycle | null;
  onRunServiceAction: RunServiceAction;
  onDatabaseDeleted: (id: string) => void;
  onKeyValueDeleted: (id: string) => void;
  envGroups?: EnvGroupView[];
  /**
   * The workspace's sandbox groups (w22/m603): those whose `environmentId` is
   * this environment are its rows, and all of them are Manage resources'
   * candidates. Empty when bex-api has no sandboxes, which leaves the card as
   * it is without them.
   */
  sandboxGroups?: SandboxGroupItem[];
  allEnvironments?: EnvironmentView[];
  /** Resource lists that have not answered (w4/m227). */
  unread?: UnreadResources;
  /**
   * Render's `/project/<id>/environment/<envId>` names this section: it
   * scrolls into view and is outlined while this holds (w1/m682).
   */
  focused?: boolean;
}

/**
 * One environment as a section of its project's page, as Render's project
 * page stacks them (w1/m682): the name as its h2 (with Render's protected
 * icon and "Protected environment"), the counted All / Services / Env Groups
 * switcher it remembers, its own search, the table of its services, Postgres,
 * Key Value and sandbox groups, the table of its env groups, and Render's
 * empty state. The right of the switcher row holds Render's ••• menu (w1/m683)
 * after bex's New Service and Manage resources, which stay until w1/m684 and
 * w1/m685 bring Render's New menu and Move Services modal
 * (docs/ADR032-environments.md).
 */
export function EnvironmentSection({
  environment,
  projectName,
  services,
  databases,
  keyValues,
  servicePending,
  onRunServiceAction,
  onDatabaseDeleted,
  onKeyValueDeleted,
  envGroups = [],
  sandboxGroups = NO_SANDBOX_GROUPS,
  allEnvironments = [environment],
  unread = NOTHING_UNREAD,
  focused = false,
}: EnvironmentSectionProps) {
  const { t } = useTranslations();
  const titleId = useId();
  const [filter, setFilter] = useSectionFilter(environment.id);
  const [query, setQuery] = useState("");

  const [assignOpen, setAssignOpen] = useState(false);

  const rows = useMemo((): ResourceRow[] => {
    const serviceById = new Map(services.map((s) => [s.id, s]));
    const databaseById = new Map(databases.map((d) => [d.id, d]));
    const keyValueById = new Map(keyValues.map((k) => [k.id, k]));
    const serviceRows = environment.serviceIds
      .map((id) => serviceById.get(id))
      .filter((s): s is ServiceView => s != null)
      .map(toServiceRow);
    const databaseRows = environment.databaseIds
      .map((id) => databaseById.get(id))
      .filter((d): d is DatabaseView => d != null)
      .map(toDatabaseRow);
    const keyValueRows = environment.keyValueIds
      .map((id) => keyValueById.get(id))
      .filter((k): k is KeyValueView => k != null)
      .map(toKeyValueRow);
    const envGroupById = new Map(envGroups.map((group) => [group.id, group]));
    const envGroupRows = environment.envGroupIds
      .map((id) => envGroupById.get(id))
      .filter((group): group is EnvGroupView => group != null)
      .map(toEnvGroupRow);
    // An environment's Render shape names no sandbox groups: each group names
    // its environment instead, and Render lists them after the other kinds.
    const sandboxGroupRows = sandboxGroups
      .filter((group) => inEnvironment(group.environmentId, environment.id))
      .map(toSandboxGroupRow);
    return [
      ...serviceRows,
      ...databaseRows,
      ...keyValueRows,
      ...envGroupRows,
      ...sandboxGroupRows,
    ];
  }, [
    environment.id,
    environment.serviceIds,
    environment.databaseIds,
    environment.keyValueIds,
    services,
    databases,
    keyValues,
    environment.envGroupIds,
    envGroups,
    sandboxGroups,
  ]);
  // Render keeps the env groups in a table of their own under the rest.
  const resourceRows = rows.filter((row) => row.kind !== "envgroup");
  const groupRows = rows.filter((row) => row.kind === "envgroup");
  const visibleResources = resourceRows.filter((row) =>
    matchesResourceQuery(row, query),
  );
  const visibleGroups = groupRows.filter((row) =>
    matchesResourceQuery(row, query),
  );
  // Judged against the whole membership, so a row merely hidden by the search
  // or the switcher stays selected (useResourceSelection).
  const selection = useResourceSelection(rows);
  // Families this environment has members of whose list has not answered:
  // their rows are unknown, so neither they nor the total can read as zero.
  const memberIds = environmentMemberIds(environment);
  const unknown = unreadFamilies(unread).filter(
    (family) => memberIds[family].length > 0,
  );
  const counts = environmentFilterCounts(
    countsWithUnknown(countProjectResources(rows), unknown),
  );
  const resourcesUnknown = unknown.some((family) => family !== "envgroups");
  const groupsUnknown = unknown.includes("envgroups");
  const moveTargets = allEnvironments.filter(
    (candidate) => candidate.id !== environment.id,
  );
  // Every change to a protected environment's membership is an Admin's move
  // (w3/m281): a non-Admin sees Manage resources and Move disabled with the
  // reason, and a protected destination disabled in the move picker.
  const placement = useProtectedPlacement();
  const placementReason = placement.restricts(environment)
    ? placement.reason
    : undefined;
  // Creating or moving a resource here needs can_create, and in a protected
  // environment an Admin (w3/m281): a Viewer reads the role's reason, a
  // Developer the admin-only one, never a 403 after the click.
  const capabilities = useCapabilities();
  const createReasonKey = capabilities.reasonKey("can_create");
  const placeReason =
    (createReasonKey ? t(createReasonKey) : undefined) ?? placementReason;
  // Render's empty state offers moving only when the workspace has something
  // to move.
  const workspaceHasResources =
    services.length +
      databases.length +
      keyValues.length +
      sandboxGroups.length >
    0;

  const showResources = filter !== "envgroups";
  const showGroups = filter !== "services";
  const rowsInFilter =
    (showResources ? resourceRows.length : 0) +
    (showGroups ? groupRows.length : 0);
  const visibleInFilter =
    (showResources ? visibleResources.length : 0) +
    (showGroups ? visibleGroups.length : 0);
  const noMatches = rowsInFilter > 0 && visibleInFilter === 0;

  const sectionRef = useFocusedSection(focused, rows.length);

  const tableProps = {
    servicePending,
    onRunServiceAction,
    onDatabaseDeleted,
    onKeyValueDeleted,
    selectedKeys: selection.selected,
    onSelectedKeysChange: selection.setSelected,
  };
  const isProtected = environment.protectedStatus === "protected";
  // One selection toolbar for both tables, over the first one shown.
  const toolbarOverResources = showResources && visibleResources.length > 0;
  const toolbar = (
    <BulkResourceToolbar
      selection={selection}
      moveTargets={moveTargets.map((target) => ({
        environment: target,
        label: target.name,
      }))}
      moveReason={placementReason}
      protectedEnvironment={isProtected}
    />
  );

  return (
    <section
      ref={sectionRef}
      aria-labelledby={titleId}
      data-focused={focused ? "" : undefined}
      className={cn(focusedSectionClass(focused), "space-y-4")}
    >
      <EnvironmentSectionHeader
        titleId={titleId}
        title={
          <>
            {/* leading-snug: the 28px line keeps descenders unclipped. */}
            <span
              className="min-w-0 truncate leading-snug"
              title={environment.name}
            >
              {environment.name}
            </span>
            {isProtected ? (
              <LockKeyhole
                aria-hidden="true"
                className="size-4 shrink-0 text-muted-foreground"
              />
            ) : null}
          </>
        }
        description={
          isProtected ? (
            <p className="text-sm text-muted-foreground">
              {t("environments.protectedDescription")}
            </p>
          ) : undefined
        }
        switcher={
          <EnvironmentResourceSwitcher
            filter={filter}
            counts={counts}
            onChange={setFilter}
          />
        }
        actions={
          <>
            <PermissionTooltip reason={placeReason}>
              {placeReason !== undefined ? (
                <Button variant="outline" size="sm" disabled>
                  <Plus />
                  {t("projects.newService")}
                </Button>
              ) : (
                <Button asChild variant="outline" size="sm">
                  <Link
                    to="/web/new"
                    search={{
                      projectId: environment.projectId,
                      environmentId: environment.id,
                    }}
                  >
                    <Plus />
                    {t("projects.newService")}
                  </Link>
                </Button>
              )}
            </PermissionTooltip>
            <PermissionTooltip reason={placementReason}>
              <Button
                variant="outline"
                size="sm"
                disabled={placementReason !== undefined}
                onClick={() => setAssignOpen(true)}
              >
                <Settings2 />
                {t("environments.manageButton")}
              </Button>
            </PermissionTooltip>
            <EnvironmentMenu
              environment={environment}
              projectName={projectName}
            />
          </>
        }
      />
      <EnvironmentResourceSearch
        environmentName={environment.name}
        query={query}
        onChange={setQuery}
      />
      <UnreadResourcesNotice unread={unread} families={unknown} />
      {showResources ? (
        resourceRows.length === 0 ? (
          resourcesUnknown ? null : (
            <EnvironmentEmptyState
              environment={environment}
              offerMove={workspaceHasResources}
              reason={placeReason}
              onMove={() => setAssignOpen(true)}
            />
          )
        ) : visibleResources.length > 0 ? (
          <div className="space-y-3">
            {toolbar}
            <ResourceTable
              rows={visibleResources}
              projectMetadata
              {...tableProps}
            />
          </div>
        ) : null
      ) : null}
      {filter === "envgroups" && groupRows.length === 0 && !groupsUnknown ? (
        <p className="py-6 text-center text-sm text-muted-foreground">
          {t("environments.noEnvGroups")}
        </p>
      ) : null}
      {showGroups && visibleGroups.length > 0 ? (
        <div className="space-y-3">
          {toolbarOverResources ? null : toolbar}
          <ResourceTable rows={visibleGroups} envGroupColumns {...tableProps} />
        </div>
      ) : null}
      {noMatches ? <NoMatchingResources /> : null}

      <ManageResourcesDialog
        environment={environment}
        services={services}
        databases={databases}
        keyValues={keyValues}
        sandboxGroups={sandboxGroups}
        open={assignOpen}
        onOpenChange={setAssignOpen}
        unread={unread}
      />
    </section>
  );
}

/**
 * Render's empty environment (w1/m682): "<env> is empty", a sentence that
 * offers moving only when the workspace has something to move, and Create new
 * service and Move existing services, disabled with the reason for a role
 * that cannot place a resource here. Move opens Manage resources until
 * w1/m685 brings Render's Move Services modal.
 */
function EnvironmentEmptyState({
  environment,
  offerMove,
  reason,
  onMove,
}: {
  environment: EnvironmentView;
  offerMove: boolean;
  reason: string | undefined;
  onMove: () => void;
}) {
  const { t } = useTranslations();
  return (
    <div className="space-y-4 rounded-lg border border-dashed px-6 py-10 text-center">
      <div className="space-y-1">
        <h3 className="font-semibold">
          {t("environments.emptyTitle", { name: environment.name })}
        </h3>
        <p className="text-sm text-muted-foreground">
          {t(
            offerMove
              ? "environments.emptyBodyCreateOrMove"
              : "environments.emptyBodyCreate",
          )}
        </p>
      </div>
      <div className="flex flex-col justify-center gap-2 sm:flex-row sm:gap-4">
        <PermissionTooltip reason={reason}>
          {reason !== undefined ? (
            <Button size="sm" disabled>
              <Plus />
              {t("environments.createNewService")}
            </Button>
          ) : (
            <Button asChild size="sm">
              <Link
                to="/web/new"
                search={{
                  projectId: environment.projectId,
                  environmentId: environment.id,
                }}
              >
                <Plus />
                {t("environments.createNewService")}
              </Link>
            </Button>
          )}
        </PermissionTooltip>
        {offerMove ? (
          <PermissionTooltip reason={reason}>
            <Button
              variant="outline"
              size="sm"
              disabled={reason !== undefined}
              onClick={onMove}
            >
              <ArrowRightLeft />
              {t("environments.moveExistingServices")}
            </Button>
          </PermissionTooltip>
        ) : null}
      </div>
    </div>
  );
}

const NO_SANDBOX_GROUPS: SandboxGroupItem[] = [];
