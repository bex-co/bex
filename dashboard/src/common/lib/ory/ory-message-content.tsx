import {
  messageTestId,
  type OryMessageContentProps,
} from "@ory/elements-react";
import { useOryMessageText } from "@/common/lib/ory/ory-message-text";
import { cn } from "@/common/lib/utils/utils.ts";

/**
 * Replaces Ory's `DefaultMessage` (`Message.Content`), which renders every
 * message Ory shows inline: a node's own messages under its field (the
 * settings guard's email-change refusal sits on `traits.email`) and a card's
 * flow-level messages. Same element, classes and test id as Ory's; only the
 * wording goes through `useOryMessageText`, so the settings guard's refusals
 * read in the dashboard's language (w7/m465).
 */
export function OryMessageContent({ message }: OryMessageContentProps) {
  const text = useOryMessageText(message);
  return (
    <span
      className={cn(
        "leading-normal",
        message.type === "error" &&
          "text-interface-foreground-validation-danger",
        message.type === "info" &&
          "text-interface-foreground-default-secondary",
        message.type === "success" &&
          "text-interface-foreground-validation-success",
      )}
      {...messageTestId(message)}
    >
      {text}
    </span>
  );
}
