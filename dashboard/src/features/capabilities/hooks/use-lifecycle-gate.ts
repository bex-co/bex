// A datastore's Suspend and Resume, gated on its own projection
// (databaseActions / keyValueActions) and rechecked fresh before dispatch,
// the way the service surfaces gate on serverActions (w6/m143, w4/m159). In a
// protected environment only an Admin may run either, so a Developer reads
// Render's "This action is limited to admins." before the click (w4/m578).

import { useRef, useState } from "react";
import { toast } from "sonner";
import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { useBoundActionConfirm } from "@/features/capabilities/hooks/use-bound-action-confirm";
import { useDatastoreActions } from "@/features/capabilities/hooks/use-resource-actions";
import { projectionReason } from "@/features/capabilities/lib/resource-actions";

export type LifecycleVerb = "suspend" | "resume";

/**
 * `suspended` is the resource's state as rendered, so a change re-asks the
 * projection. A surface that offers neither verb passes `enabled: false` and
 * asks nothing.
 */
export function useLifecycleGate(
  kind: "database" | "keyValue",
  resourceId: string,
  suspended: boolean,
  enabled = true,
) {
  const { t } = useTranslations();
  const actions = useDatastoreActions(
    kind,
    enabled ? resourceId : null,
    suspended,
  );
  const { currentWorkspaceId } = useWorkspace();
  const { openConfirm, clearConfirm, recheckBeforeDispatch } =
    useBoundActionConfirm({ resourceId, kind });
  const inFlight = useRef(false);
  const [checking, setChecking] = useState(false);

  /** Why `verb` cannot run now, or undefined when it may. */
  const reason = (verb: LifecycleVerb) =>
    projectionReason(actions, currentWorkspaceId, resourceId, verb, t);

  /**
   * Runs `run` only once a fresh read of the projection still allows `verb`,
   * whatever the rendered decision says by now: a routine permission refresh
   * reads as checking for a moment, and must neither close a confirmation
   * nor swallow its click (w4/m159). Null when nothing was sent: a check was
   * already in flight, or the recheck refused and said why in a toast.
   */
  async function dispatch<T>(
    verb: LifecycleVerb,
    run: () => Promise<T>,
  ): Promise<T | null> {
    if (inFlight.current) return null;
    const binding = openConfirm(verb);
    if (!binding) {
      toast.error(t("capabilities.actionUnavailable"));
      return null;
    }
    inFlight.current = true;
    setChecking(true);
    try {
      const { ok } = await recheckBeforeDispatch(binding);
      if (!ok) return null;
      clearConfirm();
      return await run();
    } finally {
      inFlight.current = false;
      setChecking(false);
    }
  }

  return { reason, dispatch, checking };
}
