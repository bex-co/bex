import { useMemo, useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { Layers, Search } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  Card,
  CardAction,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/common/components/ui/card";
import { Input } from "@/common/components/ui/input";
import { EnvGroupsTableSkeleton } from "@/features/environments/components/environment-settings-skeleton";
import { useTranslations } from "@/common/hooks/use-translations";
import { PermissionTooltip } from "@/features/capabilities/components/permission-tooltip";
import { NewEnvGroupDialog } from "@/features/env-groups/components/new-env-group-dialog";
import { useEnvGroups } from "@/features/env-groups/hooks/use-env-groups";
import { useEnvGroupScopeIndex } from "@/features/env-groups/hooks/use-env-group-scope-index";
import type { EnvironmentView } from "@/features/environments/hooks/use-environments";
import { NoMatchingResources } from "@/features/environments/components/no-matching-resources";
import { ResourceTable } from "@/features/projects/components/resource-table";
import { toEnvGroupRow } from "@/features/projects/hooks/use-grouped-resources";
import { matchesResourceQuery } from "@/features/projects/lib/resource-filter";
import { useServices } from "@/features/services/hooks/use-services";
import type { RunServiceAction } from "@/features/services/hooks/use-service-lifecycle";

const noop = () => undefined;
/** An env groups table runs no service action; the table still takes one. */
const noServiceAction: RunServiceAction = async () => ({ status: "error" });

/**
 * Render's Environment Groups card (w1/m683): the environment's env groups in
 * a searchable table (Render's Env Groups columns without Environment), and
 * New Env Group, which opens the create dialog with this environment chosen
 * until w1/m684 brings Render's create page. A role that cannot create here
 * sees New Env Group disabled with the reason.
 */
export function EnvironmentEnvGroupsCard({
  environment,
  createReason,
}: {
  environment: EnvironmentView;
  createReason?: string;
}) {
  const { t } = useTranslations();
  const navigate = useNavigate();
  const { groups, loading, error, refetch } = useEnvGroups({ poll: false });
  const [query, setQuery] = useState("");
  const [creating, setCreating] = useState(false);

  const rows = useMemo(() => {
    const members = new Set(environment.envGroupIds);
    return groups.filter((group) => members.has(group.id)).map(toEnvGroupRow);
  }, [groups, environment.envGroupIds]);
  const visible = rows.filter((row) => matchesResourceQuery(row, query));
  const pending =
    loading && rows.length === 0 && environment.envGroupIds.length > 0;

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("environments.envGroupsCardTitle")}</CardTitle>
        <CardAction>
          <PermissionTooltip reason={createReason}>
            <Button
              variant="outline"
              size="sm"
              disabled={createReason !== undefined}
              onClick={() => setCreating(true)}
            >
              <Layers />
              {t("environments.envGroupsNew")}
            </Button>
          </PermissionTooltip>
        </CardAction>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="relative">
          <Search
            aria-hidden="true"
            className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground"
          />
          <Input
            type="search"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder={t("environments.envGroupsSearch")}
            aria-label={t("environments.envGroupsSearch")}
            className="pl-9"
          />
        </div>
        {pending ? (
          <EnvGroupsTableSkeleton />
        ) : error && rows.length === 0 && environment.envGroupIds.length > 0 ? (
          <p className="text-sm text-destructive">
            {t("environments.resourcesUnavailable")}
          </p>
        ) : rows.length === 0 ? (
          <p className="py-6 text-center text-sm text-muted-foreground">
            {t("environments.envGroupsNone")}
          </p>
        ) : visible.length > 0 ? (
          <ResourceTable
            rows={visible}
            envGroupColumns
            servicePending={null}
            onRunServiceAction={noServiceAction}
            onDatabaseDeleted={noop}
            onKeyValueDeleted={noop}
          />
        ) : (
          <NoMatchingResources />
        )}
      </CardContent>
      {creating ? (
        <EnvGroupCreate
          environmentId={environment.id}
          onOpenChange={setCreating}
          refetch={refetch}
          onCreated={(groupId) =>
            void navigate({ to: "/env-groups/$groupId", params: { groupId } })
          }
        />
      ) : null}
    </Card>
  );
}

/**
 * The create dialog with its scope index and service list, read only once
 * someone opens it, as the Env Groups page wires it.
 */
function EnvGroupCreate({
  environmentId,
  onOpenChange,
  refetch,
  onCreated,
}: {
  environmentId: string;
  onOpenChange: (open: boolean) => void;
  refetch: () => Promise<unknown>;
  onCreated: (groupId: string) => void;
}) {
  const scope = useEnvGroupScopeIndex();
  const {
    services,
    loading: servicesLoading,
    error: servicesError,
    refetch: refetchServices,
  } = useServices({ poll: false });
  return (
    <NewEnvGroupDialog
      open
      onOpenChange={onOpenChange}
      refetch={refetch}
      services={services}
      servicesLoading={servicesLoading}
      servicesError={servicesError}
      environments={scope.environments}
      projects={scope.projects}
      serviceEnvironmentById={scope.serviceEnvironmentById}
      initialEnvironmentId={environmentId}
      scopeLoading={scope.loading}
      scopeReady={scope.ready}
      scopeError={scope.error}
      onRetry={() => {
        scope.retry();
        void refetchServices().catch(() => undefined);
      }}
      onCreated={onCreated}
    />
  );
}
