import { useCallback, useMemo, useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/common/components/ui/dialog";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/common/components/ui/tabs";
import { useTranslations } from "@/common/hooks/use-translations";
import { UnreadResourcesNotice } from "@/features/environments/components/unread-resources-notice";
import {
  unreadFamilies,
  type UnreadResources,
} from "@/features/environments/lib/unread-resources";
import type { ProjectResourceFamily } from "@/features/projects/lib/resource-filter";
import type { EnvironmentView } from "@/features/environments/hooks/use-environments";
import { useSetEnvironmentServices } from "@/features/environments/hooks/use-set-environment-services";
import { useSetEnvironmentDatabases } from "@/features/environments/hooks/use-set-environment-databases";
import { useSetEnvironmentKeyValues } from "@/features/environments/hooks/use-set-environment-keyvalues";
import { useSetEnvironmentEnvGroups } from "@/features/environments/hooks/use-set-environment-env-groups";
import { ResourceChecklist } from "@/features/environments/components/resource-checklist";
import { ServiceStatusBadge } from "@/features/services/components/service-status-badge";
import { DatabaseStatusBadge } from "@/features/databases/components/database-status-badge";
import { KeyValueStatusBadge } from "@/features/keyvalue/components/key-value-status-badge";
import type { ServiceView } from "@/features/services/types";
import type { DatabaseView } from "@/features/databases/types";
import type { KeyValueView } from "@/features/keyvalue/types";
import { useEnvGroups } from "@/features/env-groups/hooks/use-env-groups";
import { useEnvGroupScopeIndex } from "@/features/env-groups/hooks/use-env-group-scope-index";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import {
  aclBearing,
  placementIndex,
  restrictionMarkers,
} from "@/features/environments/lib/placement";
import type { ResourceChecklistItem } from "@/features/environments/components/resource-checklist";
import { toast } from "sonner";
import { environmentNameIn } from "@/features/env-groups/lib/scope";
import {
  strandedGroupsForDestination,
  strandedPromptItems,
} from "@/features/env-groups/lib/stranded";
import {
  StrandedGroupsConfirmDialog,
  type StrandedGroupPromptItem,
} from "@/features/environments/components/stranded-groups-confirm-dialog";

/** The dialog's four tabs, which are also its four independent draft keys. */
type ResourceKind = ProjectResourceFamily;

export interface ManageResourcesDialogProps {
  environment: EnvironmentView;
  /** Candidate resources — the whole workspace's, since assigning auto-joins the project. */
  services: ServiceView[];
  databases: DatabaseView[];
  keyValues: KeyValueView[];
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Resource lists that have not answered; their tabs offer no Save (w4/m227). */
  unread?: UnreadResources;
}

/**
 * The environment's "Manage resources" dialog (w6/m20 extension — closes the
 * Environments/Projects asymmetry: an environment now groups services,
 * databases, AND key-value instances, not just services). One tab per
 * resource kind, each a `ResourceChecklist` full-replacing that kind's
 * membership via its own `setEnvironment*` mutation — assigning any of them
 * also auto-joins the environment's parent project (docs/ADR032-environments.md).
 */
export function ManageResourcesDialog({
  environment,
  services,
  databases,
  keyValues,
  open,
  onOpenChange,
  unread,
}: ManageResourcesDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        {/* Radix unmounts Content's children on close, so this form remounts
            each open — its four drafts seed fresh from the environment's
            current members without a sync effect. That remount is exactly why
            the drafts belong on the form and not on each tab's checklist: the
            form survives tab changes, a checklist does not (w4/133). */}
        <ManageResourcesForm
          environment={environment}
          services={services}
          databases={databases}
          keyValues={keyValues}
          unread={unread}
          onClose={() => onOpenChange(false)}
        />
      </DialogContent>
    </Dialog>
  );
}

function ManageResourcesForm({
  environment,
  services,
  databases,
  keyValues,
  onClose,
  unread,
}: {
  environment: EnvironmentView;
  services: ServiceView[];
  databases: DatabaseView[];
  keyValues: KeyValueView[];
  onClose: () => void;
  unread?: UnreadResources;
}) {
  const { t } = useTranslations();
  const { setServices, busyId: servicesBusyId } = useSetEnvironmentServices();
  const { setDatabases, busyId: databasesBusyId } =
    useSetEnvironmentDatabases();
  const { setKeyValues, busyId: keyValuesBusyId } =
    useSetEnvironmentKeyValues();
  const { setEnvGroups, busyId: envGroupsBusyId } =
    useSetEnvironmentEnvGroups();
  // The query is scoped to the workspace switcher's current selection, so a
  // cross-workspace group is never offered as an assignment candidate.
  const { groups: envGroups } = useEnvGroups();

  // The four drafts live HERE, not in each ResourceChecklist, because this
  // component stays mounted across tab changes while Radix unmounts inactive
  // TabsContent — which is what used to discard an unsaved selection the moment
  // the user looked at another resource kind (w4/133).
  //
  // Seeded once per dialog open (lazy initial state, never a sync effect): the
  // Dialog remounts this form on each open, so reopening reflects the latest
  // persisted membership, while a background refetch handing down a new array
  // instance cannot stomp a draft in progress.
  const [drafts, setDrafts] = useState<Record<ResourceKind, Set<string>>>(
    () => ({
      services: new Set(environment.serviceIds),
      databases: new Set(environment.databaseIds),
      keyvalues: new Set(environment.keyValueIds),
      envgroups: new Set(environment.envGroupIds),
    }),
  );

  const toggle = useCallback(
    (kind: ResourceKind, id: string, next: boolean) => {
      setDrafts((prev) => {
        const nextSet = new Set(prev[kind]);
        if (next) nextSet.add(id);
        else nextSet.delete(id);
        return { ...prev, [kind]: nextSet };
      });
    },
    [],
  );

  // Each candidate's current environment, when it is another one: checking it
  // moves it out of there, which the row and the footer say before Save. An
  // ACL-bearing source needs an admin (w4/m232), so a confirmed non-admin
  // cannot check it, and the row says why rather than Save meeting a 403.
  const scopeIndex = useEnvGroupScopeIndex();
  const { environments: workspaceEnvironments } = scopeIndex;

  // w4/m238: saving the services tab moves services into this environment
  // (checked) or out of every environment (unchecked). Either can strand a
  // linked group scoped elsewhere — confirm before the single write.
  const [strandedPrompt, setStrandedPrompt] = useState<{
    items: StrandedGroupPromptItem[];
    proceed: () => void;
    cancel: () => void;
  } | null>(null);

  function strandedItemsFor(
    serviceIds: string[],
    destinationEnvironmentId: string | null,
  ): StrandedGroupPromptItem[] {
    return strandedPromptItems(
      strandedGroupsForDestination(envGroups, serviceIds, destinationEnvironmentId),
      (group) =>
        group.environmentId == null
          ? undefined
          : environmentNameIn(scopeIndex, group.environmentId),
    );
  }

  function saveServices(ids: string[]): Promise<boolean> {
    const current = new Set(environment.serviceIds);
    const wanted = new Set(ids);
    const added = ids.filter((id) => !current.has(id));
    const removed = environment.serviceIds.filter((id) => !wanted.has(id));
    if (added.length === 0 && removed.length === 0) {
      return setServices(environment.id, environment.name, ids);
    }
    if (!scopeIndex.ready) {
      toast.error(t("environments.strandedScopeUnknown"));
      return Promise.resolve(false);
    }
    const byId = new Map<string, StrandedGroupPromptItem>();
    for (const item of [
      ...strandedItemsFor(added, environment.id),
      ...strandedItemsFor(removed, null),
    ]) {
      byId.set(item.id, item);
    }
    if (byId.size === 0) {
      return setServices(environment.id, environment.name, ids);
    }
    return new Promise<boolean>((resolve) => {
      setStrandedPrompt({
        items: [...byId.values()],
        proceed: () => {
          setStrandedPrompt(null);
          void setServices(environment.id, environment.name, ids).then(resolve);
        },
        cancel: () => {
          setStrandedPrompt(null);
          resolve(false);
        },
      });
    });
  }
  const environmentOf = useMemo(
    () => placementIndex(workspaceEnvironments),
    [workspaceEnvironments],
  );
  const { loaded: capsLoaded, canManage } = useCapabilities();
  const placed = (
    kind: ResourceKind,
    item: ResourceChecklistItem,
  ): ResourceChecklistItem => {
    const source = environmentOf(kind, item.id);
    if (!source || source.id === environment.id) return item;
    const guarded =
      capsLoaded &&
      !canManage &&
      aclBearing(source) &&
      !drafts[kind].has(item.id);
    return {
      ...item,
      location: t("environments.manageResourceIn", {
        environment: source.name,
      }),
      markers: restrictionMarkers(source).map((key) => t(key)),
      disabledReason: guarded
        ? t("environments.manageNeedsAdminToMove", { environment: source.name })
        : undefined,
    };
  };
  const moveSummary = (kind: ResourceKind, items: ResourceChecklistItem[]) => {
    // Keyed by id: environments in different projects may share a name.
    const bySource = new Map<string, { name: string; resources: string[] }>();
    for (const item of items) {
      const source = environmentOf(kind, item.id);
      if (!source || source.id === environment.id || !drafts[kind].has(item.id))
        continue;
      const entry = bySource.get(source.id) ?? {
        name: source.name,
        resources: [],
      };
      entry.resources.push(item.name);
      bySource.set(source.id, entry);
    }
    return [...bySource.values()].map(({ name, resources }) =>
      t("environments.manageMovesOut", {
        environment: name,
        resources: resources.join(", "),
      }),
    );
  };
  const checklist = (kind: ResourceKind, items: ResourceChecklistItem[]) => {
    const shown = items.map((item) => placed(kind, item));
    return { items: shown, moveSummary: moveSummary(kind, shown) };
  };

  // A tab whose list has not answered shows why instead of an empty
  // checklist, and offers no Save over an unread list (w4/m227).
  const unreadKind = (kind: ResourceKind) =>
    unread != null && unreadFamilies(unread).includes(kind);

  return (
    <>
      <DialogHeader>
        <DialogTitle>
          {t("environments.manageTitle", { name: environment.name })}
        </DialogTitle>
        <DialogDescription>
          {t("environments.manageDescription")}
        </DialogDescription>
      </DialogHeader>

      <Tabs defaultValue="services">
        <TabsList className="grid w-full grid-cols-4">
          <TabsTrigger value="services">
            {t("environments.tabServices")}
          </TabsTrigger>
          <TabsTrigger value="databases">
            {t("environments.tabDatabases")}
          </TabsTrigger>
          <TabsTrigger value="keyvalues">
            {t("environments.tabKeyValues")}
          </TabsTrigger>
          <TabsTrigger value="envgroups">
            {t("environments.tabEnvGroups")}
          </TabsTrigger>
        </TabsList>
        <TabsContent value="services">
          {unreadKind("services") ? (
            <UnreadResourcesNotice unread={unread!} families={["services"]} />
          ) : (
            <ResourceChecklist
              {...checklist(
                "services",
                services.map((s) => ({
                  id: s.id,
                  name: s.name,
                  badge: <ServiceStatusBadge service={s} />,
                })),
              )}
              checked={drafts.services}
              onToggle={(id, next) => toggle("services", id, next)}
              busy={servicesBusyId === environment.id}
              emptyLabel={t("environments.manageNoServices")}
              onSave={(ids) => saveServices(ids)}
              onClose={onClose}
            />
          )}
        </TabsContent>
        <TabsContent value="databases">
          {unreadKind("databases") ? (
            <UnreadResourcesNotice unread={unread!} families={["databases"]} />
          ) : (
            <ResourceChecklist
              {...checklist(
                "databases",
                databases.map((d) => ({
                  id: d.id,
                  name: d.name,
                  badge: <DatabaseStatusBadge database={d} />,
                })),
              )}
              checked={drafts.databases}
              onToggle={(id, next) => toggle("databases", id, next)}
              busy={databasesBusyId === environment.id}
              emptyLabel={t("environments.manageNoDatabases")}
              onSave={(ids) =>
                setDatabases(environment.id, environment.name, ids)
              }
              onClose={onClose}
            />
          )}
        </TabsContent>
        <TabsContent value="keyvalues">
          {unreadKind("keyvalues") ? (
            <UnreadResourcesNotice unread={unread!} families={["keyvalues"]} />
          ) : (
            <ResourceChecklist
              {...checklist(
                "keyvalues",
                keyValues.map((k) => ({
                  id: k.id,
                  name: k.name,
                  badge: <KeyValueStatusBadge keyValue={k} />,
                })),
              )}
              checked={drafts.keyvalues}
              onToggle={(id, next) => toggle("keyvalues", id, next)}
              busy={keyValuesBusyId === environment.id}
              emptyLabel={t("environments.manageNoKeyValues")}
              onSave={(ids) =>
                setKeyValues(environment.id, environment.name, ids)
              }
              onClose={onClose}
            />
          )}
        </TabsContent>
        <TabsContent value="envgroups">
          {unreadKind("envgroups") ? (
            <UnreadResourcesNotice unread={unread!} families={["envgroups"]} />
          ) : (
            <ResourceChecklist
              {...checklist(
                "envgroups",
                envGroups.map((group) => ({
                  id: group.id,
                  name: group.name,
                  badge: null,
                })),
              )}
              checked={drafts.envgroups}
              onToggle={(id, next) => toggle("envgroups", id, next)}
              busy={envGroupsBusyId === environment.id}
              emptyLabel={t("environments.manageNoEnvGroups")}
              onSave={(ids) =>
                setEnvGroups(environment.id, environment.name, ids)
              }
              onClose={onClose}
            />
          )}
        </TabsContent>
      </Tabs>
      <StrandedGroupsConfirmDialog
        open={strandedPrompt != null}
        onOpenChange={(next) => {
          if (!next) strandedPrompt?.cancel();
        }}
        items={strandedPrompt?.items ?? []}
        onConfirm={() => strandedPrompt?.proceed()}
      />
    </>
  );
}
