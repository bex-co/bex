import type { ComponentProps, ReactNode } from "react";
import { BrainIcon } from "lucide-react";
import { cn } from "@/common/lib/utils/utils.ts";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/common/components/ai-elements/collapsible.tsx";

// The agent's chain-of-thought (AI Elements `Reasoning`) — the Devin "Thought
// for Xs" disclosure. Collapsed by default so a long transcript stays scannable;
// the caller can force it open while the reasoning part is still streaming.

export function Reasoning({
  defaultOpen = false,
  className,
  children,
  ...props
}: Omit<ComponentProps<typeof Collapsible>, "open"> & {
  defaultOpen?: boolean;
}) {
  return (
    <Collapsible
      defaultOpen={defaultOpen}
      className={cn("bg-muted/30", className)}
      {...props}
    >
      {children}
    </Collapsible>
  );
}

export function ReasoningTrigger({
  children,
  ...props
}: ComponentProps<typeof CollapsibleTrigger>) {
  return (
    <CollapsibleTrigger {...props}>
      <BrainIcon
        aria-hidden
        className="text-muted-foreground size-4 shrink-0"
      />
      <span className="text-muted-foreground">{children}</span>
    </CollapsibleTrigger>
  );
}

export function ReasoningContent({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <CollapsibleContent
      className={cn(
        "text-muted-foreground whitespace-pre-wrap break-words",
        className,
      )}
    >
      {children}
    </CollapsibleContent>
  );
}
