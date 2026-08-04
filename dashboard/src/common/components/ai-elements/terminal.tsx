import type { ComponentProps } from "react";
import { TerminalIcon } from "lucide-react";
import { cn } from "@/common/lib/utils/utils.ts";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/common/components/ai-elements/collapsible.tsx";

// A terminal/command block (the ACP `terminal` update + captured output). Not a
// named AI Elements primitive, but the Devin "Worked" transcript renders shell
// activity as its own collapsible group alongside Reasoning/Task/Tool, so it
// follows the same disclosure shape here.

export function Terminal({
  defaultOpen = false,
  className,
  children,
  ...props
}: Omit<ComponentProps<typeof Collapsible>, "open"> & {
  defaultOpen?: boolean;
}) {
  return (
    <Collapsible defaultOpen={defaultOpen} className={className} {...props}>
      {children}
    </Collapsible>
  );
}

export function TerminalTrigger({
  children,
  ...props
}: ComponentProps<typeof CollapsibleTrigger>) {
  return (
    <CollapsibleTrigger {...props}>
      <TerminalIcon
        aria-hidden
        className="text-muted-foreground size-4 shrink-0"
      />
      <span className="min-w-0 flex-1 truncate">{children}</span>
    </CollapsibleTrigger>
  );
}

export function TerminalContent({
  children,
  className,
}: {
  children: string;
  className?: string;
}) {
  return (
    <CollapsibleContent className={cn("p-0", className)}>
      <pre className="overflow-x-auto rounded-b-md bg-[#0a0a0a] p-3 font-mono text-xs leading-relaxed text-[#e5e5e5]">
        {children}
      </pre>
    </CollapsibleContent>
  );
}
