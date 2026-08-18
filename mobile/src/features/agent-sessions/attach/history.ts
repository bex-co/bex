import { readUIMessageStream, type UIMessage, type UIMessageChunk } from "ai";

export type DurableConversation = {
  cursor: number;
  turns: Array<{
    turn: number;
    idempotencyKey: string;
    prompt: string;
    assistantParts: string[];
  }>;
};

export type PreparedConversation = {
  cursor: number;
  messages: UIMessage[];
};

/**
 * Rebuild the complete useChat seed from the durable turn ledger. The control
 * plane owns user prompts and verbatim AI SDK chunks, so a fresh process needs
 * no local transcript cache and cannot invent or duplicate a turn.
 */
export async function prepareConversation(
  conversation: DurableConversation,
): Promise<PreparedConversation> {
  if (!Number.isSafeInteger(conversation.cursor) || conversation.cursor < -1) {
    throw new Error("agent conversation returned an invalid cursor");
  }
  const messages: UIMessage[] = [];
  let previousTurn = 0;
  for (const turn of conversation.turns) {
    if (
      !Number.isSafeInteger(turn.turn) ||
      turn.turn <= previousTurn ||
      !turn.idempotencyKey ||
      typeof turn.prompt !== "string"
    ) {
      throw new Error("agent conversation returned an invalid turn");
    }
    previousTurn = turn.turn;
    messages.push({
      id: `user-${turn.turn}-${turn.idempotencyKey}`,
      role: "user",
      parts: [{ type: "text", text: turn.prompt }],
    });
    const assistant = await assembleAssistant(turn.assistantParts);
    if (assistant) messages.push(assistant);
  }
  return { cursor: conversation.cursor, messages };
}

async function assembleAssistant(
  serializedParts: string[],
): Promise<UIMessage | undefined> {
  if (serializedParts.length === 0) return undefined;
  const chunks = serializedParts.map(parseChunk);
  const stream = new ReadableStream<UIMessageChunk>({
    start(controller) {
      for (const chunk of chunks) controller.enqueue(chunk);
      controller.close();
    },
  });
  let assistant: UIMessage | undefined;
  for await (const state of readUIMessageStream({ stream })) {
    assistant = state;
  }
  return assistant;
}

function parseChunk(raw: string): UIMessageChunk {
  let value: unknown;
  try {
    value = JSON.parse(raw);
  } catch {
    throw new Error("agent conversation returned an invalid assistant part");
  }
  if (!value || typeof value !== "object" || !("type" in value)) {
    throw new Error("agent conversation returned an invalid assistant part");
  }
  return value as UIMessageChunk;
}
