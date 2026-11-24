import { useCallback } from "react";
import {
  ShellTerminal,
  type ShellTerminalCopy,
} from "@/common/components/shell-terminal";
import { useTranslations } from "@/common/hooks/use-translations";
import { adminOnlyErrorMessage } from "@/features/capabilities/lib/admin-only";
import {
  ShellUnavailableError,
  useShellSession,
} from "@/features/services/hooks/use-shell-session";

/** A service's Shell words (w4/m352): instances, and why a session ended. */
const SERVICE_SHELL_COPY: ShellTerminalCopy = {
  connecting: "services.shellConnecting",
  connected: "services.shellConnected",
  closed: "services.shellClosed",
  errorStatus: "services.shellErrorStatus",
  ended: "services.shellEnded",
  reconnect: "services.shellReconnect",
  errorGeneric: "services.shellErrorGeneric",
  endReasons: {
    revoked: "services.shellEndRevoked",
    suspended: "services.shellEndSuspended",
    ineligible: "services.shellEndIneligible",
    instance_gone: "services.shellEndInstanceGone",
    timeout: "services.shellEndTimeout",
    unverified: "services.shellEndUnverified",
  },
};

/**
 * A service's Browser Web Shell terminal (w2/m55, docs/ADR035-ssh.md
 * § Browser Web Shell): the shared `ShellTerminal` over the service's
 * `createShellSession` ticket, on one instance or any ready one.
 */
export function WebShellTerminal({
  serviceId,
  instanceId,
}: {
  serviceId: string;
  instanceId?: string;
}) {
  const { t } = useTranslations();
  const { createShellSession } = useShellSession();
  const mint = useCallback(
    () => createShellSession(serviceId, instanceId),
    [createShellSession, serviceId, instanceId],
  );
  // A refusal names its own blocker (suspended, Free, a first deploy not yet
  // live; w4/506): show that sentence, not generic copy (w4/m352). A protected
  // environment's refusal of a non-Admin reads as Render's admin-only reason
  // (w4/728).
  const mintErrorMessage = useCallback(
    (err: unknown) =>
      err instanceof ShellUnavailableError
        ? t("services.shellErrorUnavailable")
        : adminOnlyErrorMessage(err, t("services.shellErrorGeneric"), t),
    [t],
  );
  return (
    <ShellTerminal
      mint={mint}
      mintErrorMessage={mintErrorMessage}
      copy={SERVICE_SHELL_COPY}
    />
  );
}
