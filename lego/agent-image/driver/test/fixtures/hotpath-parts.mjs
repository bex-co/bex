export const representativeParts = [
  { type: "start" },
  { type: "text-delta", delta: "hello" },
  { type: "reasoning", text: "thinking" },
  { type: "tool-input-start", toolCallId: "t1", toolName: "bash" },
  { type: "tool-input-delta", toolCallId: "t1", inputTextDelta: "{}" },
  { type: "data-acp-plan", data: { steps: ["inspect"] } },
  { type: "data-acp-diff", data: { path: "a.ts" } },
  { type: "data-acp-terminal", data: { output: "ok" } },
  { type: "finish" },
];
