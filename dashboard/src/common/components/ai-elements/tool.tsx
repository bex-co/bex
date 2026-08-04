import type { ComponentProps, ReactNode } from "react";
import { WrenchIcon } from "lucide-react";
import { cn } from "@/common/lib/utils/utils.ts";
import { Badge } from "@/common/components/ui/badge";
import { CodeBlock } from "@/common/components/code-block";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/common/components/ai-elements/collapsible.tsx";

// A single tool invocation (AI Elements `Tool`). The header names the tool and
// reflects its lifecycle state; the collapsed body shows the input and, once
// available, the output or error. Rendered from the AI SDK `tool-*` UI parts.

export type ToolState =
  | "input-streaming"
  | "input-available"
  | "output-available"
  | "output-error"
  | string;

export function Tool({
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

function stateVariant(
  state: ToolState,
): "secondary" | "success" | "destructive" {
  if (state === "output-available") return "success";
  if (state === "output-error") return "destructive";
  return "secondary";
}

export function ToolHeader({
  name,
  state,
  stateLabel,
}: {
  name: string;
  state: ToolState;
  /** Human-readable, i18n'd state label (falls back to the raw state). */
  stateLabel?: string;
}) {
  return (
    <CollapsibleTrigger>
      <WrenchIcon
        aria-hidden
        className="text-muted-foreground size-4 shrink-0"
      />
      <span className="min-w-0 flex-1 truncate font-mono text-xs">{name}</span>
      <Badge variant={stateVariant(state)} className="shrink-0">
        {stateLabel ?? state}
      </Badge>
    </CollapsibleTrigger>
  );
}

export function ToolContent({
  className,
  ...props
}: ComponentProps<typeof CollapsibleContent>) {
  return (
    <CollapsibleContent className={cn("space-y-2", className)} {...props} />
  );
}

function Section({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="space-y-1">
      <p className="text-muted-foreground text-xs font-medium uppercase tracking-wide">
        {label}
      </p>
      {children}
    </div>
  );
}

export function ToolInput({ label, input }: { label: string; input: unknown }) {
  if (input === undefined || input === null) return null;
  return (
    <Section label={label}>
      <CodeBlock code={stringify(input)} language="json" />
    </Section>
  );
}

export function ToolOutput({
  label,
  errorLabel,
  output,
  errorText,
}: {
  label: string;
  errorLabel: string;
  output?: unknown;
  errorText?: string | null;
}) {
  if (errorText) {
    return (
      <Section label={errorLabel}>
        <p className="text-destructive whitespace-pre-wrap break-words text-xs">
          {errorText}
        </p>
      </Section>
    );
  }
  if (output === undefined || output === null) return null;
  return (
    <Section label={label}>
      <CodeBlock code={stringify(output)} language="json" />
    </Section>
  );
}

function stringify(value: unknown): string {
  if (typeof value === "string") return value;
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}
