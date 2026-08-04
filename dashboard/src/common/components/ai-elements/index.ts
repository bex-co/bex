// Vendored AI Elements (shadcn registry, https://ai-sdk.dev/elements) adapted
// for this repo: plain React 19 + Tailwind 4, no Next.js-isms, reusing the
// repo's own primitives (MarkdownRenderer, CodeBlock, Badge) instead of pulling
// streamdown/shiki/radix-collapsible. Hook-agnostic and presentational — the
// agent-sessions conversation column composes them over `useChat`.

export {
  Collapsible,
  CollapsibleTrigger,
  CollapsibleContent,
} from "@/common/components/ai-elements/collapsible.tsx";
export {
  Conversation,
  ConversationContent,
} from "@/common/components/ai-elements/conversation.tsx";
export {
  Message,
  MessageContent,
  type MessageRole,
} from "@/common/components/ai-elements/message.tsx";
export { Response } from "@/common/components/ai-elements/response.tsx";
export {
  Reasoning,
  ReasoningTrigger,
  ReasoningContent,
} from "@/common/components/ai-elements/reasoning.tsx";
export {
  Task,
  TaskTrigger,
  TaskContent,
  TaskItem,
  type TaskItemStatus,
} from "@/common/components/ai-elements/task.tsx";
export {
  Tool,
  ToolHeader,
  ToolContent,
  ToolInput,
  ToolOutput,
  type ToolState,
} from "@/common/components/ai-elements/tool.tsx";
export {
  Terminal,
  TerminalTrigger,
  TerminalContent,
} from "@/common/components/ai-elements/terminal.tsx";
