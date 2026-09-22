import { useCallback, useState } from "react";
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

/** The dialog's four tabs, which are also its four independent draft keys. */
type ResourceKind = "services" | "databases" | "keyvalues" | "envgroups";

export interface ManageResourcesDialogProps {
  environment: EnvironmentView;
  /** Candidate resources — the whole workspace's, since assigning auto-joins the project. */
  services: ServiceView[];
  databases: DatabaseView[];
  keyValues: KeyValueView[];
  open: boolean;
  onOpenChange: (open: boolean) => void;
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
}: {
  environment: EnvironmentView;
  services: ServiceView[];
  databases: DatabaseView[];
  keyValues: KeyValueView[];
  onClose: () => void;
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
          <ResourceChecklist
            items={services.map((s) => ({
              id: s.id,
              name: s.name,
              badge: <ServiceStatusBadge service={s} />,
            }))}
            checked={drafts.services}
            onToggle={(id, next) => toggle("services", id, next)}
            busy={servicesBusyId === environment.id}
            emptyLabel={t("environments.manageNoServices")}
            onSave={(ids) => setServices(environment.id, environment.name, ids)}
            onClose={onClose}
          />
        </TabsContent>
        <TabsContent value="databases">
          <ResourceChecklist
            items={databases.map((d) => ({
              id: d.id,
              name: d.name,
              badge: <DatabaseStatusBadge database={d} />,
            }))}
            checked={drafts.databases}
            onToggle={(id, next) => toggle("databases", id, next)}
            busy={databasesBusyId === environment.id}
            emptyLabel={t("environments.manageNoDatabases")}
            onSave={(ids) =>
              setDatabases(environment.id, environment.name, ids)
            }
            onClose={onClose}
          />
        </TabsContent>
        <TabsContent value="keyvalues">
          <ResourceChecklist
            items={keyValues.map((k) => ({
              id: k.id,
              name: k.name,
              badge: <KeyValueStatusBadge keyValue={k} />,
            }))}
            checked={drafts.keyvalues}
            onToggle={(id, next) => toggle("keyvalues", id, next)}
            busy={keyValuesBusyId === environment.id}
            emptyLabel={t("environments.manageNoKeyValues")}
            onSave={(ids) =>
              setKeyValues(environment.id, environment.name, ids)
            }
            onClose={onClose}
          />
        </TabsContent>
        <TabsContent value="envgroups">
          <ResourceChecklist
            items={envGroups.map((group) => ({
              id: group.id,
              name: group.name,
              badge: null,
            }))}
            checked={drafts.envgroups}
            onToggle={(id, next) => toggle("envgroups", id, next)}
            busy={envGroupsBusyId === environment.id}
            emptyLabel={t("environments.manageNoEnvGroups")}
            onSave={(ids) =>
              setEnvGroups(environment.id, environment.name, ids)
            }
            onClose={onClose}
          />
        </TabsContent>
      </Tabs>
    </>
  );
}
