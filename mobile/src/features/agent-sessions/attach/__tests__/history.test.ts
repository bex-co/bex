import type { UIMessageChunk } from "ai";
import { prepareConversation } from "../history";

function serializedAssistant(text: string): string[] {
  const chunks: UIMessageChunk[] = [
    { type: "start", messageId: "assistant-1" },
    { type: "text-start", id: "text-1" },
    { type: "text-delta", id: "text-1", delta: text },
    { type: "text-end", id: "text-1" },
    { type: "finish" },
  ];
  return chunks.map((chunk) => JSON.stringify(chunk));
}

describe("durable agent conversation history", () => {
  it("reconstructs ordered user and assistant messages with the durable cursor", async () => {
    const prepared = await prepareConversation({
      cursor: 14,
      turns: [
        {
          turn: 1,
          idempotencyKey: "initial",
          prompt: "Fix the failing test",
          assistantParts: serializedAssistant("I fixed it."),
        },
        {
          turn: 2,
          idempotencyKey: "follow-up",
          prompt: "Run the full suite",
          assistantParts: [],
        },
      ],
    });

    expect(prepared.cursor).toBe(14);
    expect(prepared.messages.map((message) => message.role)).toEqual([
      "user",
      "assistant",
      "user",
    ]);
    expect(prepared.messages[0].id).toBe("user-1-initial");
    expect(prepared.messages[2].id).toBe("user-2-follow-up");
    expect(prepared.messages[1].parts.length).toBe(1);
    expect(prepared.messages[1].parts[0].type).toBe("text");
    expect(
      prepared.messages[1].parts[0].type === "text"
        ? prepared.messages[1].parts[0].text
        : "",
    ).toBe("I fixed it.");
  });

  it("rejects non-monotonic turns instead of inventing an order", async () => {
    let message = "";
    try {
      await prepareConversation({
        cursor: 1,
        turns: [
          {
            turn: 2,
            idempotencyKey: "two",
            prompt: "two",
            assistantParts: [],
          },
          {
            turn: 1,
            idempotencyKey: "one",
            prompt: "one",
            assistantParts: [],
          },
        ],
      });
    } catch (error) {
      message = error instanceof Error ? error.message : String(error);
    }
    expect(message).toBe("agent conversation returned an invalid turn");
  });
});
