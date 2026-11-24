import { useCallback } from "react";
import { useMutation } from "@apollo/client/react";
import { CreateAgentSessionShellSessionDocument } from "@/graphql/definitions";
import { hasGraphQLErrorCode } from "@/common/lib/graphql-error";
import type { ShellTicket } from "@/common/components/shell-terminal";
import { ShellUnavailableError } from "@/features/services/hooks/use-shell-session";
import { toAgentSessionError } from "@/features/agent-sessions/lib/errors";
import { usePaymentRequiredGate } from "@/features/usage/context/payment-required-context";
import { useAgentSessionCapabilities } from "@/features/agent-sessions/hooks/use-agent-session-capabilities";
import type { AgentSessionView } from "@/features/agent-sessions/types";

/**
 * Whether this deployment serves the browser terminal (`terminalEnabled`),
 * so the session page offers its Terminal (w10/m615).
 */
export function useSessionTerminalOffered(
  session: Pick<AgentSessionView, "ownerId">,
): boolean {
  const { capabilities } = useAgentSessionCapabilities(session.ownerId);
  return Boolean(capabilities?.terminalEnabled);
}

/**
 * Mints the session page's Terminal ticket (w10/m615, docs/ADR054-open-in-zed.md
 * D10) through `createAgentSessionShellSession`. bex-api authorizes
 * can_view_sensitive on the session and starts a sandbox that closed as the
 * caller, as opening a sleeping Devin session's Terminal wakes it; the ticket
 * then reads `starting` and the gateway holds the WebSocket until the sandbox
 * is up. A start is metered sandbox compute, so a payment-method refusal goes
 * through the same interception dialog as resume's (ADR075 D7). A coded
 * refusal comes back as an `AgentSessionError`; an unconfigured browser
 * transport as `ShellUnavailableError`.
 */
export function useAgentSessionTerminal() {
  const paymentGate = usePaymentRequiredGate();
  const [mutate] = useMutation(CreateAgentSessionShellSessionDocument);

  const createTerminal = useCallback(
    async (id: string): Promise<ShellTicket> => {
      try {
        const res = await paymentGate.run(() =>
          mutate({ variables: { id }, fetchPolicy: "no-cache" }),
        );
        const session = res.data?.createAgentSessionShellSession;
        if (!session?.ticket || !session.url) {
          throw new Error("the Terminal's ticket response was empty");
        }
        return {
          ticket: session.ticket,
          url: session.url,
          starting: session.sandbox === "starting",
        };
      } catch (err) {
        if (
          err instanceof Error &&
          hasGraphQLErrorCode(err, "SHELL_UNAVAILABLE")
        ) {
          throw new ShellUnavailableError(err.message);
        }
        throw toAgentSessionError(err);
      }
    },
    [mutate, paymentGate],
  );

  return { createTerminal };
}
