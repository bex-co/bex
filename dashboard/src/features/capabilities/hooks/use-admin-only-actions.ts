// The verbs a protected environment keeps to Admins on a service, a datastore
// or an environment group (render.com/docs/projects): revealing a Postgres's
// or Key Value's connection info, changing its inbound IP rules, and viewing,
// changing or deleting an environment group's variables and secret files
// (w4/m594), and deleting a service, a Postgres or a Key Value (w4/724). A
// Developer reads Render's "This action is limited to admins." on each control
// before the click, decided by bex-api's own rule (core.RestrictedDecision)
// through the resource's action projection.

import { useCallback } from "react";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { useAdminOnlyProjection } from "@/features/capabilities/hooks/use-resource-actions";
import {
  projectionReason,
  type ResourceActionId,
} from "@/features/capabilities/lib/resource-actions";

export type AdminOnlyResource =
  "service" | "database" | "keyValue" | "envGroup";

/**
 * Why `action` on resource `id` cannot run now (still checking, refused with
 * its reason, or the check unavailable), or undefined when it may. A null id
 * asks nothing and blocks nothing.
 */
export function useAdminOnlyActions(
  kind: AdminOnlyResource,
  id: string | null,
): (action: ResourceActionId) => string | undefined {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const projection = useAdminOnlyProjection(kind, id);
  return useCallback(
    (action: ResourceActionId) =>
      id === null
        ? undefined
        : projectionReason(projection, currentWorkspaceId, id, action, t),
    [projection, currentWorkspaceId, id, t],
  );
}
