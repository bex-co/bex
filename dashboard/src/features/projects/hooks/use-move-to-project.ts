import { useCallback, useState } from "react";
import { useApolloClient, useMutation } from "@apollo/client/react";
import { useRouter } from "@tanstack/react-router";
import { toast } from "sonner";
import {
  ProjectsDocument,
  SetProjectServicesDocument,
  SetProjectDatabasesDocument,
  SetProjectKeyValuesDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import {
  useProjects,
  type ProjectView,
} from "@/features/projects/hooks/use-projects";

export type ProjectResourceKind = "service" | "database" | "keyvalue";

function idsOf(kind: ProjectResourceKind, project: ProjectView): string[] {
  if (kind === "service") return project.serviceIds;
  if (kind === "database") return project.databaseIds;
  return project.keyValueIds;
}

export interface UseMoveToProjectResult {
  /** Every project in the current workspace (for the "Move to project" picker). */
  projects: ProjectView[];
  /** The project currently holding resourceId, or null when ungrouped. */
  currentProjectId: (resourceId: string) => string | null;
  /** Assign resourceId to targetProjectId, unassigning it from any prior project first. */
  moveTo: (
    resourceId: string,
    resourceName: string,
    targetProjectId: string,
  ) => Promise<boolean>;
  /** Unassign resourceId from whichever project currently holds it. */
  removeFromProject: (
    resourceId: string,
    resourceName: string,
  ) => Promise<boolean>;
  /** The resource id currently being moved, or null. */
  busyId: string | null;
}

/**
 * Drives the "Move to project" row action shared by services/databases/key-value
 * (w1/m31 extension — databases/key-value grouping added alongside the unified
 * dashboard Projects page). A project's membership per resource kind is a
 * full-replace list (setProjectServices/Databases/KeyValues), so moving a
 * resource computes the old and new project's full member lists client-side
 * and fires up to two mutations, mirroring how the backend's own
 * SetServices/SetDatabases/SetKeyValues diff against a wanted set.
 */
export function useMoveToProject(
  kind: ProjectResourceKind,
): UseMoveToProjectResult {
  const { t } = useTranslations();
  const client = useApolloClient();
  const router = useRouter();
  const { projects } = useProjects();
  const [setServices] = useMutation(SetProjectServicesDocument);
  const [setDatabases] = useMutation(SetProjectDatabasesDocument);
  const [setKeyValues] = useMutation(SetProjectKeyValuesDocument);
  const [busyId, setBusyId] = useState<string | null>(null);

  const runSet = useCallback(
    (projectId: string, ids: string[]) => {
      if (kind === "service") {
        return setServices({ variables: { id: projectId, serviceIds: ids } });
      }
      if (kind === "database") {
        return setDatabases({
          variables: { id: projectId, databaseIds: ids },
        });
      }
      return setKeyValues({ variables: { id: projectId, keyValueIds: ids } });
    },
    [kind, setServices, setDatabases, setKeyValues],
  );

  const currentProjectId = useCallback(
    (resourceId: string) =>
      projects.find((p) => idsOf(kind, p).includes(resourceId))?.id ?? null,
    [projects, kind],
  );

  // Post-move refresh. Selecting a menu item closes the dropdown, which
  // unmounts the menu (and this hook) and aborts any query the unmounted
  // instance still had in flight — so the refresh goes through the client and
  // router singletons (both lifecycle-independent), not a component-scoped
  // refetch, and is fire-and-forget: the mutations already succeeded and
  // updated the cache, so a refresh failure must never be reported as a failed
  // move. Two surfaces render these row actions, each fed differently:
  //   - the overview page reads the live `Projects` watcher (useProjects), so
  //     client.refetchQueries({ Projects }) updates it; but
  //   - the project-detail page's membership table is derived from its route
  //     loader's snapshot of the *singular* `Project` query (network-only),
  //     which refetching `Projects` neither refreshes nor re-runs — so we also
  //     router.invalidate() to re-run that loader (matching the same page's own
  //     refetchAll on database/key-value deletion). Without it a removed
  //     resource lingers in the detail table until a manual reload.
  const refreshProjects = useCallback(() => {
    client
      .refetchQueries({ include: [ProjectsDocument] })
      .catch(() => undefined);
    router.invalidate().catch(() => undefined);
  }, [client, router]);

  const moveTo = useCallback(
    async (
      resourceId: string,
      resourceName: string,
      targetProjectId: string,
    ) => {
      const from = projects.find((p) => idsOf(kind, p).includes(resourceId));
      const to = projects.find((p) => p.id === targetProjectId);
      if (!to || from?.id === to.id) return false;
      setBusyId(resourceId);
      try {
        if (from) {
          await runSet(
            from.id,
            idsOf(kind, from).filter((id) => id !== resourceId),
          );
        }
        await runSet(to.id, [...idsOf(kind, to), resourceId]);
      } catch {
        toast.error(t("projects.moveError", { name: resourceName }));
        return false;
      } finally {
        setBusyId(null);
      }
      toast.success(
        t("projects.moveSuccess", { name: resourceName, project: to.name }),
      );
      refreshProjects();
      return true;
    },
    [projects, kind, runSet, refreshProjects, t],
  );

  const removeFromProject = useCallback(
    async (resourceId: string, resourceName: string) => {
      const from = projects.find((p) => idsOf(kind, p).includes(resourceId));
      if (!from) return false;
      setBusyId(resourceId);
      try {
        await runSet(
          from.id,
          idsOf(kind, from).filter((id) => id !== resourceId),
        );
      } catch {
        toast.error(t("projects.removeError", { name: resourceName }));
        return false;
      } finally {
        setBusyId(null);
      }
      toast.success(t("projects.removeSuccess", { name: resourceName }));
      refreshProjects();
      return true;
    },
    [projects, kind, runSet, refreshProjects, t],
  );

  return { projects, currentProjectId, moveTo, removeFromProject, busyId };
}
