import { useEffect, useState } from "react";
import { useQuery } from "@apollo/client/react";
import type { UIMessage } from "ai";
import { MobileAgentSessionConversationDocument } from "@/generated-graphql";
import { prepareConversation } from "./history";

export type ConversationSeedState =
  | { status: "loading"; messages: UIMessage[]; cursor: -1; error: null }
  | {
      status: "ready";
      messages: UIMessage[];
      cursor: number;
      error: Error | null;
    };

/** Load the durable turn ledger before useChat opens its cursor-based GET. */
export function useConversationSeed(sessionId: string): ConversationSeedState {
  const { data, loading, error } = useQuery(
    MobileAgentSessionConversationDocument,
    {
      variables: { id: sessionId },
      fetchPolicy: "network-only",
      errorPolicy: "all",
    },
  );
  const [state, setState] = useState<ConversationSeedState>({
    status: "loading",
    messages: [],
    cursor: -1,
    error: null,
  });

  useEffect(() => {
    let current = true;
    const conversation = data?.agentSessionConversation;
    if (loading && !conversation && !error) return () => undefined;

    if (!conversation) {
      // An older/unavailable history surface can still use the gateway's full
      // replay from -1. Keep the degradation visible to the caller while the
      // phase-1 metadata/evidence view remains available.
      setState({
        status: "ready",
        messages: [],
        cursor: -1,
        error: error ? toError(error) : null,
      });
      return () => undefined;
    }
    if (conversation.sessionId !== sessionId) {
      setState({
        status: "ready",
        messages: [],
        cursor: -1,
        error: new Error("agent conversation session mismatch"),
      });
      return () => undefined;
    }

    void prepareConversation(conversation)
      .then((prepared) => {
        if (!current) return;
        setState({
          status: "ready",
          messages: prepared.messages,
          cursor: prepared.cursor,
          error: error ? toError(error) : null,
        });
      })
      .catch((prepareError: unknown) => {
        if (!current) return;
        setState({
          status: "ready",
          messages: [],
          cursor: -1,
          error: toError(prepareError),
        });
      });
    return () => {
      current = false;
    };
  }, [data?.agentSessionConversation, error, loading, sessionId]);

  return state;
}

function toError(value: unknown): Error {
  return value instanceof Error ? value : new Error("conversation unavailable");
}
