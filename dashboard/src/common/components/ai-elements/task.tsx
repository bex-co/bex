import type { ComponentProps, ReactNode } from "react";
import {
  CheckIcon,
  CircleIcon,
  LoaderIcon,
  ListChecksIcon,
} from "lucide-react";
import { cn } from "@/common/lib/utils/utils.ts";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/common/components/ai-elements/collapsible.tsx";

// The agent's plan (AI Elements `Task`) — the ACP `plan` update, rendered as
// the Devin "Worked" checklist. Each entry carries a status the icon reflects.

export type TaskItemStatus = "pending" | "in_progress" | "completed" | string;

export function Task({
  defaultOpen = true,
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

export function TaskTrigger({
  title,
  ...props
}: ComponentProps<typeof CollapsibleTrigger> & { title: ReactNode }) {
  return (
    <CollapsibleTrigger {...props}>
      <ListChecksIcon
        aria-hidden
        className="text-muted-foreground size-4 shrink-0"
      />
      <span>{title}</span>
    </CollapsibleTrigger>
  );
}

export function TaskContent({
  className,
  ...props
}: ComponentProps<typeof CollapsibleContent>) {
  return (
    <CollapsibleContent className={cn("space-y-0.5", className)} {...props} />
  );
}

export function TaskItem({
  status = "pending",
  className,
  children,
}: {
  status?: TaskItemStatus;
  className?: string;
  children: ReactNode;
}) {
  const Icon =
    status === "completed"
      ? CheckIcon
      : status === "in_progress"
        ? LoaderIcon
        : CircleIcon;
  return (
    <div
      className={cn("flex items-start gap-1.5 text-xs leading-5", className)}
    >
      <Icon
        aria-hidden
        className={cn(
          "mt-0.5 size-3 shrink-0",
          status === "completed"
            ? "text-emerald-600 dark:text-emerald-400"
            : status === "in_progress"
              ? "text-amber-600 dark:text-amber-400"
              : "text-muted-foreground",
        )}
      />
      <span
        className={cn(
          "min-w-0 break-words",
          status === "completed" && "text-muted-foreground line-through",
        )}
      >
        {children}
      </span>
    </div>
  );
}
