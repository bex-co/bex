import { useCallback, useState } from "react";
import type { ApolloClient } from "@apollo/client";
import { useApolloClient, useMutation } from "@apollo/client/react";
import { useRouter } from "@tanstack/react-router";
import { toast } from "sonner";
import {
  EnvironmentsDocument,
  ProjectDocument,
  ProjectsDocument,
  SetProjectServicesDocument,
  SetProjectDatabasesDocument,
  SetProjectKeyValuesDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { mutationErrorMessage } from "@/common/lib/graphql-error";
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
  /** Assign resourceId to targetProjectId; the server unassigns it from any prior project. */
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
 * Re-read the source project's Environments after a move or remove (w4/m178).
 *
 * A selected Environment's rows and counts come from its own EnvironmentsDocument
 * member arrays, which neither the membership mutation nor a Project read
 * touches, so without this the moved resource lingers until the next poll.
 * Same ordering as the rename refresh (use-rename-project): settle the source
 * project's in-flight list reads first (a pre-write poll would otherwise land
 * after the fresh read and restore the old membership), each on its own since
 * the aggregate rejects early, then re-read without joining an older request.
 * Only the source project's lists are touched.
 */
async function refreshSourceEnvironments(
  client: ApolloClient,
  projectId: string,
) {
  const lists = client.refetchQueries({
    include: [EnvironmentsDocument],
    // Only join reads already in flight; an idle watcher would just duplicate
    // the fresh read below.
    onQueryUpdated: (query) =>
      query.variables.projectId === projectId &&
      query.getCurrentResult().loading,
  });
  lists.catch(() => undefined); // each result is settled below
  await Promise.allSettled(lists.results);
  await client.query({
    query: EnvironmentsDocument,
    variables: { projectId },
    fetchPolicy: "network-only",
    context: { queryDeduplication: false },
  });
}

/**
 * Drives the "Move to project" row action shared by services/databases/key-value
 * (w1/m31 extension — databases/key-value grouping added alongside the unified
 * dashboard Projects page). A project's membership per resource kind is a
 * full-replace list (setProjectServices/Databases/KeyValues), so a move computes
 * the target project's full member list client-side, mirroring how the
 * backend's own SetServices/SetDatabases/SetKeyValues diff against a wanted
 * set.
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

  // Post-move refresh. Selecting a menu item unmounts this hook, so the
  // refresh goes through the client and router singletons and is
  // fire-and-forget: the mutation already succeeded, so a refresh failure must
  // never read as a failed move (the next poll re-reads; nothing re-writes).
  // The overview reads the live `Projects` watcher; the project page renders
  // its route loader's `Project` snapshot, which router.invalidate() re-runs
  // cache-first for a retained match. A cross-project move's mutation returns
  // only the target, and no `Projects` watcher is guaranteed to be mounted
  // (the sidebar is lazy and closes on mobile), so re-read the source
  // network-only and invalidate only after it settles. Remove's mutation
  // returns the source itself, so it skips that read. Either way the source's
  // Environments are re-read too (w4/m178).
  const refreshSource = useCallback(
    (
      sourceProjectId: string | undefined,
      { sourceProjectStale }: { sourceProjectStale: boolean },
    ) => {
      const refetches: Promise<unknown>[] = [
        client.refetchQueries({ include: [ProjectsDocument] }),
      ];
      if (sourceProjectId && sourceProjectStale) {
        refetches.push(
          client.query({
            query: ProjectDocument,
            variables: { id: sourceProjectId },
            fetchPolicy: "network-only",
          }),
        );
      }
      void Promise.allSettled(refetches).then(() =>
        router.invalidate().catch(() => undefined),
      );
      if (sourceProjectId) {
        refreshSourceEnvironments(client, sourceProjectId).catch(
          () => undefined,
        );
      }
    },
    [client, router],
  );

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
        // One mutation, not two. Membership is single-valued on the server —
        // a service's apps.project_id column, a database/key-value's single
        // core.LabelProject label — so assigning the resource to the target
        // already unassigns it from its old project; a second call clearing
        // the source is pure redundancy. It was also the bug: the pair ran
        // remove-then-add with no rollback, so a failure on the add left the
        // resource in NEITHER project while the toast said the move failed
        // (w6/036). One call avoids that preliminary detach; the server
        // validates the requested set before applying it. The stale source
        // list is reconciled by refreshSource below.
        await runSet(to.id, [...idsOf(kind, to), resourceId]);
      } catch (err) {
        toast.error(
          mutationErrorMessage(
            err,
            t("projects.moveError", { name: resourceName }),
          ),
        );
        return false;
      } finally {
        setBusyId(null);
      }
      toast.success(
        t("projects.moveSuccess", { name: resourceName, project: to.name }),
      );
      refreshSource(from?.id, { sourceProjectStale: true });
      return true;
    },
    [projects, kind, runSet, refreshSource, t],
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
      } catch (err) {
        toast.error(
          mutationErrorMessage(
            err,
            t("projects.removeError", { name: resourceName }),
          ),
        );
        return false;
      } finally {
        setBusyId(null);
      }
      toast.success(t("projects.removeSuccess", { name: resourceName }));
      refreshSource(from.id, { sourceProjectStale: false });
      return true;
    },
    [projects, kind, runSet, refreshSource, t],
  );

  return { projects, currentProjectId, moveTo, removeFromProject, busyId };
}
