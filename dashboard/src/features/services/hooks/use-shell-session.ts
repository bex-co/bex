import { useCallback } from "react";
import { useMutation } from "@apollo/client/react";
import { CreateShellSessionDocument } from "@/graphql/definitions";

export interface ShellSession {
  /** The signed exec ticket to present to the gateway WebSocket. */
  ticket: string;
  /** The browser-reachable gateway WebSocket origin (e.g. wss://ssh.bex.co/shell). */
  url: string;
  /** RFC3339 expiry of the ticket. */
  expiresAt: string;
}

/** Thrown when bex-api reports the browser-shell transport is unconfigured (503). */
export class ShellUnavailableError extends Error {}

/**
 * Mints a Browser Web Shell exec ticket via bex-api's `createShellSession`
 * mutation (w2/m55, docs/ADR035-ssh.md § Browser Web Shell). The terminal opens
 * the returned gateway WebSocket with the ticket. The mutation runs behind the
 * dashboard's Kratos session; bex-api authorizes `can_operate` before minting.
 */
export function useShellSession() {
  const [mutate] = useMutation(CreateShellSessionDocument);

  const createShellSession = useCallback(
    async (id: string, instanceId?: string): Promise<ShellSession> => {
      try {
        const res = await mutate({
          variables: { id, instanceId: instanceId ?? null },
          fetchPolicy: "no-cache",
        });
        const session = res.data?.createShellSession;
        if (!session?.ticket || !session.url) {
          throw new Error("shell session response was empty");
        }
        return {
          ticket: session.ticket,
          url: session.url,
          expiresAt: session.expiresAt ?? "",
        };
      } catch (err) {
        // The backend returns ErrShellUnavailable (503, "…not configured") when
        // BEX_SHELL_TICKET_SECRET/BEX_SHELL_WS_URL are unset — a distinct,
        // actionable state the terminal surfaces differently from a transient
        // failure.
        if (err instanceof Error && /not configured/i.test(err.message)) {
          throw new ShellUnavailableError(err.message);
        }
        throw err;
      }
    },
    [mutate],
  );

  return { createShellSession };
}
