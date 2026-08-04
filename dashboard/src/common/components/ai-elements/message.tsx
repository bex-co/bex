import type { ComponentProps } from "react";
import { cn } from "@/common/lib/utils/utils.ts";

// A single role-attributed turn (AI Elements `Message`). `from` aligns the
// block: the user's prompt sits to the right in a filled bubble, the agent's
// work fills the row to the left. Deliberately presentational — the caller
// composes Response/Reasoning/Task/Tool inside `MessageContent`.

export type MessageRole = "user" | "assistant" | "system";

export function Message({
  from,
  className,
  ...props
}: ComponentProps<"div"> & { from: MessageRole }) {
  return (
    <div
      data-role={from}
      className={cn(
        "flex w-full",
        from === "user" ? "justify-end" : "justify-start",
        className,
      )}
      {...props}
    />
  );
}

export function MessageContent({
  from = "assistant",
  className,
  ...props
}: ComponentProps<"div"> & { from?: MessageRole }) {
  return (
    <div
      className={cn(
        "flex min-w-0 flex-col gap-2 text-sm",
        from === "user"
          ? "bg-primary text-primary-foreground max-w-[80%] rounded-lg px-3 py-2"
          : "w-full",
        className,
      )}
      {...props}
    />
  );
}
