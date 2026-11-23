// Whether the caller may open a service's shell, in the browser or over SSH
// (w4/726). bex-api decides it with the verb's own rule: can_view_sensitive,
// and in a protected environment a workspace Admin only, as on Render
// (render.com/docs/projects). The Shell tab reads it before it mints a
// terminal ticket, so a Developer there sees Render's "This action is limited
// to admins." instead of a terminal that fails to connect.

import { useTranslations } from "@/common/hooks/use-translations";
import { useWorkspace } from "@/features/workspaces/context/hooks";
import { useServerActions } from "@/features/capabilities/hooks/use-resource-actions";
import {
  type ActionGate,
  gateAction,
  gateReason,
  resourceDecision,
} from "@/features/capabilities/lib/resource-actions";

export type ShellGate = {
  gate: ActionGate;
  /** Why the shell cannot open now, or undefined when it may. */
  reason: string | undefined;
};

/** The shell decision for `serviceId`; a null id asks nothing. */
export function useShellGate(serviceId: string | null): ShellGate {
  const { t } = useTranslations();
  const { currentWorkspaceId } = useWorkspace();
  const actions = useServerActions(serviceId);
  const gate = gateAction(
    actions.status === "ready" && serviceId !== null
      ? resourceDecision(
          actions.snapshot,
          currentWorkspaceId,
          serviceId,
          "shell",
        )
      : null,
    actions.status,
  );
  return { gate, reason: gateReason(gate, t) };
}
