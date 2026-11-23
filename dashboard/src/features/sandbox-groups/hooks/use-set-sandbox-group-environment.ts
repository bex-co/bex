import { useCallback, useState } from "react";
import { useApolloClient, useMutation } from "@apollo/client/react";
import { toast } from "sonner";
import {
  SandboxGroupsDocument,
  SetSandboxGroupEnvironmentDocument,
} from "@/graphql/definitions";
import { useTranslations } from "@/common/hooks/use-translations";
import { placementErrorMessage } from "@/features/environments/lib/placement";
import { useWorkspace } from "@/features/workspaces/context/hooks";

/** One sandbox group whose membership of an environment changes. */
export interface SandboxGroupMove {
  id: string;
  name: string;
  /** True puts the group in the environment, false takes it out of it. */
  into: boolean;
}

export interface UseSetSandboxGroupEnvironmentResult {
  /**
   * Puts each group in `environment` (or, with `into: false`, takes it out of
   * the one it is in) through bex-api's `setSandboxGroupEnvironment`, Render's
   * environment-resources move (w22/m603). The groups are written at once and
   * each settles on its own: resolves the ids of the groups that moved, so a
   * caller keeps the others selected or drafted for a retry.
   */
  setGroupsEnvironment: (
    environment: { id: string; name: string },
    moves: SandboxGroupMove[],
  ) => Promise<Set<string>>;
  /** The environment id currently being written, or null. */
  busyId: string | null;
}

/**
 * The sandbox-group writer of an environment's bulk Move and of its Manage
 * resources dialog. A group is in at most one environment, so putting it in
 * one moves it out of any other; a protected environment's move is an Admin's,
 * and a raced refusal reads as the same localized reason the disabled control
 * shows (`placementErrorMessage`).
 *
 * The answer carries the group's new `environmentId`, which updates the cached
 * group every list shows; the workspace's groups are then re-read once, for a
 * change the answer does not show. A failed re-read is not a failed move.
 */
export function useSetSandboxGroupEnvironment(): UseSetSandboxGroupEnvironmentResult {
  const { t } = useTranslations();
  const client = useApolloClient();
  const { currentWorkspaceId } = useWorkspace();
  const [mutate] = useMutation(SetSandboxGroupEnvironmentDocument);
  const [busyId, setBusyId] = useState<string | null>(null);

  const setGroupsEnvironment = useCallback(
    async (
      environment: { id: string; name: string },
      moves: SandboxGroupMove[],
    ) => {
      const moved = new Set<string>();
      if (moves.length === 0) return moved;
      setBusyId(environment.id);
      try {
        await Promise.all(
          moves.map(async (move) => {
            try {
              await mutate({
                variables: {
                  id: move.id,
                  environmentId: move.into ? environment.id : null,
                  ...(currentWorkspaceId
                    ? { ownerId: currentWorkspaceId }
                    : {}),
                },
              });
              moved.add(move.id);
            } catch (err) {
              toast.error(
                placementErrorMessage(
                  err,
                  t("sandboxGroups.environmentUpdateError", {
                    group: move.name,
                  }),
                  t,
                ),
              );
            }
          }),
        );
        if (moved.size > 0) {
          toast.success(
            t("sandboxGroups.environmentUpdated", { name: environment.name }),
          );
          try {
            await client.refetchQueries({ include: [SandboxGroupsDocument] });
          } catch {
            // The moves stand; the cached groups already carry them.
          }
        }
        return moved;
      } finally {
        setBusyId(null);
      }
    },
    [client, currentWorkspaceId, mutate, t],
  );

  return { setGroupsEnvironment, busyId };
}
