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
import { Badge } from "@/common/components/ui/badge";
import { useTranslations } from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils";
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
import type { SandboxGroupItem } from "@/features/sandbox-groups/hooks/use-sandbox-groups";
import { useSetSandboxGroupEnvironment } from "@/features/sandbox-groups/hooks/use-set-sandbox-group-environment";
import { inEnvironment } from "@/features/environments/lib/link-id";
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
  type CoMovingGroups,
} from "@/features/env-groups/lib/stranded";
import {
  StrandedGroupsConfirmDialog,
  type StrandedGroupPromptItem,
} from "@/features/environments/components/stranded-groups-confirm-dialog";

/**
 * The dialog's tabs, which are also its independent draft keys: the catalog's
 * four families, and the workspace's sandbox groups when it has any (w22/m603).
 */
type ResourceKind = ProjectResourceFamily | "sandboxgroups";

/** Tab order, which is also Save's write order. */
const catalogKinds: ResourceKind[] = [
  "services",
  "databases",
  "keyvalues",
  "envgroups",
];
const withSandboxGroups: ResourceKind[] = [...catalogKinds, "sandboxgroups"];

export interface ManageResourcesDialogProps {
  environment: EnvironmentView;
  /** Candidate resources — the whole workspace's, since assigning auto-joins the project. */
  services: ServiceView[];
  databases: DatabaseView[];
  keyValues: KeyValueView[];
  /**
   * The workspace's sandbox groups (w22/m603). Their tab shows once there is
   * one, so a workspace without any (or whose bex-api has no sandboxes) sees
   * the dialog as it is without them.
   */
  sandboxGroups?: SandboxGroupItem[];
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Resource lists that have not answered; their tabs offer no Save (w4/m227). */
  unread?: UnreadResources;
}

/**
 * The environment's "Manage resources" dialog (w6/m20 extension — closes the
 * Environments/Projects asymmetry: an environment now groups services,
 * databases, AND key-value instances, not just services). One tab per
 * resource kind; Save full-replaces each changed kind's membership via its
 * own `setEnvironment*` mutation — assigning any of them also auto-joins the
 * environment's parent project (docs/ADR032-environments.md).
 */
export function ManageResourcesDialog({
  environment,
  services,
  databases,
  keyValues,
  sandboxGroups = NO_SANDBOX_GROUPS,
  open,
  onOpenChange,
  unread,
}: ManageResourcesDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        {/* Radix unmounts Content's children on close, so this form remounts
            each open — its drafts seed fresh from the environment's
            current members without a sync effect. That remount is exactly why
            the drafts belong on the form and not on each tab's checklist: the
            form survives tab changes, a checklist does not (w4/133). */}
        <ManageResourcesForm
          environment={environment}
          services={services}
          databases={databases}
          keyValues={keyValues}
          sandboxGroups={sandboxGroups}
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
  sandboxGroups,
  onClose,
  unread,
}: {
  environment: EnvironmentView;
  services: ServiceView[];
  databases: DatabaseView[];
  keyValues: KeyValueView[];
  sandboxGroups: SandboxGroupItem[];
  onClose: () => void;
  unread?: UnreadResources;
}) {
  const { t } = useTranslations();
  const { setServices } = useSetEnvironmentServices();
  const { setDatabases } = useSetEnvironmentDatabases();
  const { setKeyValues } = useSetEnvironmentKeyValues();
  const { setEnvGroups } = useSetEnvironmentEnvGroups();
  const { setGroupsEnvironment } = useSetSandboxGroupEnvironment();
  // The query is scoped to the workspace switcher's current selection, so a
  // cross-workspace group is never offered as an assignment candidate.
  const { groups: envGroups } = useEnvGroups();

  // The drafts live HERE, not in each ResourceChecklist, because this
  // component stays mounted across tab changes while Radix unmounts inactive
  // TabsContent — which is what used to discard an unsaved selection the moment
  // the user looked at another resource kind (w4/133).
  //
  // Seeded once per dialog open (lazy initial state, never a sync effect): the
  // Dialog remounts this form on each open, so reopening reflects the latest
  // persisted membership, while a background refetch handing down a new array
  // instance cannot stomp a draft in progress.
  //
  // A sandbox group names its environment rather than being named by it, so
  // this environment's groups are the ones whose environmentId is this one.
  const sandboxGroupIds = sandboxGroups
    .filter((group) => inEnvironment(group.environmentId, environment.id))
    .map((group) => group.id);
  const [drafts, setDrafts] = useState<Record<ResourceKind, Set<string>>>(
    () => ({
      services: new Set(environment.serviceIds),
      databases: new Set(environment.databaseIds),
      keyvalues: new Set(environment.keyValueIds),
      envgroups: new Set(environment.envGroupIds),
      sandboxgroups: new Set(sandboxGroupIds),
    }),
  );
  // The sandbox groups tab shows once the workspace's groups have been read
  // and there is one, and then stays for this open. Groups read after the form
  // mounted seed their draft then, so a late read never drafts them out.
  const [groupsShown, setGroupsShown] = useState(sandboxGroups.length > 0);
  if (!groupsShown && sandboxGroups.length > 0) {
    setGroupsShown(true);
    setDrafts((prev) => ({ ...prev, sandboxgroups: new Set(sandboxGroupIds) }));
  }
  const resourceKinds = groupsShown ? withSandboxGroups : catalogKinds;

  const persisted: Record<ResourceKind, string[]> = {
    services: environment.serviceIds,
    databases: environment.databaseIds,
    keyvalues: environment.keyValueIds,
    envgroups: environment.envGroupIds,
    sandboxgroups: sandboxGroupIds,
  };
  const changed = (kind: ResourceKind) => {
    const draft = drafts[kind];
    const current = persisted[kind];
    return (
      draft.size !== current.length || current.some((id) => !draft.has(id))
    );
  };

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
    comoving?: CoMovingGroups,
  ): StrandedGroupPromptItem[] {
    return strandedPromptItems(
      strandedGroupsForDestination(
        envGroups,
        serviceIds,
        destinationEnvironmentId,
        comoving,
      ),
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
    // A group added on the Env groups tab moves here with them (w4/564).
    const addedGroups = [...drafts.envgroups].filter(
      (id) => !environment.envGroupIds.includes(id),
    );
    const byId = new Map<string, StrandedGroupPromptItem>();
    for (const item of [
      ...strandedItemsFor(added, environment.id, {
        groupIds: addedGroups,
        destinationServiceIds: ids,
      }),
      ...strandedItemsFor(removed, null),
    ]) {
      byId.set(item.id, item);
    }
    if (byId.size === 0) {
      return setServices(environment.id, environment.name, ids);
    }
    return new Promise<boolean>((resolve) => {
      // Confirming also closes the prompt, whose close handler cancels: the
      // first answer wins, or that cancel would report the confirmed write
      // as refused.
      let answered = false;
      const answer = (result: () => Promise<boolean> | boolean) => {
        if (answered) return;
        answered = true;
        setStrandedPrompt(null);
        void Promise.resolve(result()).then(resolve);
      };
      setStrandedPrompt({
        items: [...byId.values()],
        proceed: () =>
          answer(() => setServices(environment.id, environment.name, ids)),
        cancel: () => answer(() => false),
      });
    });
  }
  const placementOf = useMemo(
    () => placementIndex(workspaceEnvironments),
    [workspaceEnvironments],
  );
  // Where each candidate is now: a group names its own environment.
  const environmentOf = (kind: ResourceKind, id: string) => {
    if (kind !== "sandboxgroups") return placementOf(kind, id);
    const group = sandboxGroups.find((candidate) => candidate.id === id);
    return group
      ? workspaceEnvironments.find((e) =>
          inEnvironment(group.environmentId, e.id),
        )
      : undefined;
  };
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
    kind !== "sandboxgroups" &&
    unread != null &&
    unreadFamilies(unread).includes(kind);

  // One Save applies every tab's draft (w4/415): a selection the dialog keeps
  // visible across tabs must never be dropped under a success toast. Services
  // go first so their stranded-group confirmation runs before any write; the
  // first refusal stops the rest and keeps the dialog open on its drafts.
  const savers: Record<ResourceKind, (ids: string[]) => Promise<boolean>> = {
    services: saveServices,
    databases: (ids) => setDatabases(environment.id, environment.name, ids),
    keyvalues: (ids) => setKeyValues(environment.id, environment.name, ids),
    envgroups: (ids) => setEnvGroups(environment.id, environment.name, ids),
    sandboxgroups: saveSandboxGroups,
  };
  // A group moves on its own (Render's environment-resources move): a checked
  // one is put here, out of any other environment, and an unchecked one is
  // taken out of this one. Only groups whose membership changed are written;
  // one that was refused stays drafted, and a retry writes only it.
  async function saveSandboxGroups(ids: string[]): Promise<boolean> {
    const wanted = new Set(ids);
    const current = new Set(sandboxGroupIds);
    const nameOf = (id: string) =>
      sandboxGroups.find((group) => group.id === id)?.name ?? id;
    const moves = [
      ...ids
        .filter((id) => !current.has(id))
        .map((id) => ({ id, name: nameOf(id), into: true })),
      ...sandboxGroupIds
        .filter((id) => !wanted.has(id))
        .map((id) => ({ id, name: nameOf(id), into: false })),
    ];
    const moved = await setGroupsEnvironment(environment, moves);
    return moved.size === moves.length;
  }
  const [saving, setSaving] = useState(false);
  async function saveAll(): Promise<boolean> {
    setSaving(true);
    try {
      for (const kind of resourceKinds) {
        if (unreadKind(kind) || !changed(kind)) continue;
        if (!(await savers[kind]([...drafts[kind]]))) return false;
      }
      return true;
    } finally {
      setSaving(false);
    }
  }
  const tabLabels: Record<ResourceKind, string> = {
    services: t("environments.tabServices"),
    databases: t("environments.tabDatabases"),
    keyvalues: t("environments.tabKeyValues"),
    envgroups: t("environments.tabEnvGroups"),
    sandboxgroups: t("environments.tabSandboxGroups"),
  };

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
        {/* Five equal tabs cannot hold "Sandbox Groups" in this dialog at any
            width, so with that tab they sit on two rows, three over two. */}
        <TabsList
          className={
            groupsShown
              ? "grid h-auto w-full grid-cols-6 gap-y-[3px]"
              : "grid w-full grid-cols-4"
          }
        >
          {resourceKinds.map((kind, index) => {
            const unsavedId = changed(kind)
              ? `manage-unsaved-${kind}`
              : undefined;
            return (
              <TabsTrigger
                key={kind}
                value={kind}
                aria-describedby={unsavedId}
                className={
                  groupsShown
                    ? cn("h-[29px]", index < 3 ? "col-span-2" : "col-span-3")
                    : undefined
                }
              >
                {tabLabels[kind]}
                {unsavedId && (
                  <>
                    <span
                      aria-hidden
                      className="ml-1.5 inline-block size-1.5 rounded-full bg-amber-500"
                    />
                    <span id={unsavedId} hidden>
                      {t("environments.manageUnsaved")}
                    </span>
                  </>
                )}
              </TabsTrigger>
            );
          })}
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
              busy={saving}
              emptyLabel={t("environments.manageNoServices")}
              onSave={saveAll}
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
              busy={saving}
              emptyLabel={t("environments.manageNoDatabases")}
              onSave={saveAll}
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
              busy={saving}
              emptyLabel={t("environments.manageNoKeyValues")}
              onSave={saveAll}
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
              busy={saving}
              emptyLabel={t("environments.manageNoEnvGroups")}
              onSave={saveAll}
              onClose={onClose}
            />
          )}
        </TabsContent>
        {groupsShown ? (
          <TabsContent value="sandboxgroups">
            <ResourceChecklist
              {...checklist(
                "sandboxgroups",
                sandboxGroups.map((group) => ({
                  id: group.id,
                  name: group.name,
                  badge: group.isDefault ? (
                    <Badge variant="secondary">
                      {t("sandboxGroups.defaultBadge")}
                    </Badge>
                  ) : null,
                })),
              )}
              checked={drafts.sandboxgroups}
              onToggle={(id, next) => toggle("sandboxgroups", id, next)}
              busy={saving}
              emptyLabel={t("environments.manageNoSandboxGroups")}
              onSave={saveAll}
              onClose={onClose}
            />
          </TabsContent>
        ) : null}
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

const NO_SANDBOX_GROUPS: SandboxGroupItem[] = [];
