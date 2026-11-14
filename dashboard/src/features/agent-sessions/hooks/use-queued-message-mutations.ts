import { useCallback } from "react";
import { useApolloClient, useMutation } from "@apollo/client/react";
import {
  AgentSessionQueueDocument,
  DeleteQueuedAgentSessionMessageDocument,
  EditQueuedAgentSessionMessageDocument,
  MoveQueuedAgentSessionMessageDocument,
} from "@/graphql/definitions";
import { useAgentSessionMutations } from "@/features/agent-sessions/hooks/use-agent-session-mutations";
import {
  AgentSessionError,
  toAgentSessionError,
} from "@/features/agent-sessions/lib/errors";
import {
  toQueuedMessageView,
  toQueuedMessageViews,
} from "@/features/agent-sessions/lib/mapper";
import {
  patchCachedQueue,
  withEditedMessage,
  withoutQueuedMessage,
} from "@/features/agent-sessions/lib/queue-cache";
import type { QueuedMessageView } from "@/features/agent-sessions/types";

/** What "Send now" did: Stop turn ended a running turn, or none was running. */
export interface SendNowResult {
  stopped: boolean;
}

export interface UseQueuedMessageMutationsResult {
  /** Replaces a pending message's prompt; the turn then runs as the editor. */
  edit: (messageId: string, prompt: string) => Promise<QueuedMessageView>;
  /** Removes a pending message so it never runs. */
  remove: (messageId: string) => Promise<void>;
  /** Puts a message at a 1-based place in the queue; resolves the new order. */
  move: (messageId: string, position: number) => Promise<QueuedMessageView[]>;
  /**
   * Runs a message next, now (ADR047 D12): Stop turn ends the running turn,
   * then the message moves to the front, where the dispatcher takes it as soon
   * as the turn settles. With no turn running only the move is needed; a
   * message already first needs no move. `index` is its 0-based place now.
   */
  sendNow: (messageId: string, index: number) => Promise<SendNowResult>;
}

function hasCode(err: unknown, ...codes: string[]): boolean {
  return err instanceof AgentSessionError && codes.includes(err.code);
}

/**
 * Write wrappers over the queued-message verbs of one session (ADR047 D12).
 * Each edits the queue's cache from its answer, so the list moves at once, and
 * reads the queue again afterwards whether the verb worked or not: a refusal
 * such as AGENT_QUEUED_MESSAGE_DISPATCHED means the list on screen is stale.
 * Failures re-throw typed through `toAgentSessionError`.
 */
export function useQueuedMessageMutations(
  sessionId: string,
): UseQueuedMessageMutationsResult {
  const client = useApolloClient();
  const { interrupt } = useAgentSessionMutations();
  const [editMutation] = useMutation(EditQueuedAgentSessionMessageDocument);
  const [deleteMutation] = useMutation(DeleteQueuedAgentSessionMessageDocument);
  const [moveMutation] = useMutation(MoveQueuedAgentSessionMessageDocument);

  const refresh = useCallback(() => {
    void client
      .query({
        query: AgentSessionQueueDocument,
        variables: { id: sessionId },
        fetchPolicy: "network-only",
      })
      .catch(() => undefined);
  }, [client, sessionId]);

  // Every write types its failure and reads the queue again afterwards,
  // whether it worked or not.
  const guarded = useCallback(
    async <T>(write: () => Promise<T>): Promise<T> => {
      try {
        return await write();
      } catch (err) {
        throw toAgentSessionError(err);
      } finally {
        refresh();
      }
    },
    [refresh],
  );

  const edit = useCallback(
    (messageId: string, prompt: string) =>
      guarded(async () => {
        const res = await editMutation({
          variables: { id: sessionId, messageId, prompt },
          fetchPolicy: "no-cache",
        });
        const message = res.data?.editQueuedAgentSessionMessage;
        if (!message) throw new Error("editQueuedAgentSessionMessage failed");
        patchCachedQueue(client.cache, sessionId, (queue) =>
          withEditedMessage(queue, message),
        );
        return toQueuedMessageView(message);
      }),
    [client, editMutation, guarded, sessionId],
  );

  const remove = useCallback(
    (messageId: string) =>
      guarded(async () => {
        const res = await deleteMutation({
          variables: { id: sessionId, messageId },
          fetchPolicy: "no-cache",
        });
        if (res.data?.deleteQueuedAgentSessionMessage !== true) {
          throw new Error("deleteQueuedAgentSessionMessage did not confirm");
        }
        patchCachedQueue(client.cache, sessionId, (queue) =>
          withoutQueuedMessage(queue, messageId),
        );
      }),
    [client, deleteMutation, guarded, sessionId],
  );

  const move = useCallback(
    (messageId: string, position: number) =>
      guarded(async () => {
        const res = await moveMutation({
          variables: { id: sessionId, messageId, position },
          fetchPolicy: "no-cache",
        });
        const moved = res.data?.moveQueuedAgentSessionMessage;
        if (!moved) throw new Error("moveQueuedAgentSessionMessage failed");
        // The answer is the whole queue, in its new order.
        patchCachedQueue(client.cache, sessionId, () => moved);
        return toQueuedMessageViews(moved);
      }),
    [client, moveMutation, guarded, sessionId],
  );

  const sendNow = useCallback(
    async (messageId: string, index: number): Promise<SendNowResult> => {
      let stopped = false;
      try {
        await interrupt(sessionId);
        stopped = true;
      } catch (err) {
        // Idle: nothing to stop, the message only needs the front. Every other
        // refusal (the turn is still starting, canceled, no access) ends it
        // here, with the queue untouched.
        if (!hasCode(err, "AGENT_SESSION_NO_TURN_RUNNING")) throw err;
      }
      if (index > 0) {
        try {
          await move(messageId, 1);
        } catch (err) {
          // It started in the meantime: it is the turn that runs now.
          if (!hasCode(err, "AGENT_QUEUED_MESSAGE_DISPATCHED")) throw err;
        }
      } else {
        refresh();
      }
      return { stopped };
    },
    [interrupt, move, refresh, sessionId],
  );

  return { edit, remove, move, sendNow };
}
