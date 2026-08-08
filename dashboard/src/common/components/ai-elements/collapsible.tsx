import {
  createContext,
  useContext,
  useId,
  useState,
  type ComponentProps,
  type ReactNode,
} from "react";
import { ChevronRight } from "lucide-react";
import { cn } from "@/common/lib/utils/utils.ts";

// A tiny headless disclosure primitive the AI Elements (Reasoning/Task/Tool)
// build on. AI Elements upstream leans on shadcn's Radix `Collapsible`; that
// package isn't vendored here, and a plain React state toggle is both
// SSR-safe (no `window`) and dependency-free — which is all these transcript
// groups need. Controlled or uncontrolled via `open`/`defaultOpen`.

interface CollapsibleContextValue {
  open: boolean;
  toggle: () => void;
  contentId: string;
}

const CollapsibleContext = createContext<CollapsibleContextValue | null>(null);

function useCollapsible(): CollapsibleContextValue {
  const ctx = useContext(CollapsibleContext);
  if (!ctx) {
    throw new Error(
      "Collapsible subcomponents must be used within <Collapsible>",
    );
  }
  return ctx;
}

export function Collapsible({
  open: controlledOpen,
  defaultOpen = false,
  onOpenChange,
  className,
  children,
  ...props
}: Omit<ComponentProps<"div">, "onChange"> & {
  open?: boolean;
  defaultOpen?: boolean;
  onOpenChange?: (open: boolean) => void;
}) {
  const [uncontrolled, setUncontrolled] = useState(defaultOpen);
  const isControlled = controlledOpen !== undefined;
  const open = isControlled ? controlledOpen : uncontrolled;
  const contentId = useId();

  const toggle = () => {
    const next = !open;
    if (!isControlled) setUncontrolled(next);
    onOpenChange?.(next);
  };

  return (
    <CollapsibleContext.Provider value={{ open, toggle, contentId }}>
      <div className={cn("rounded-md border", className)} {...props}>
        {children}
      </div>
    </CollapsibleContext.Provider>
  );
}

export function CollapsibleTrigger({
  className,
  children,
  ...props
}: ComponentProps<"button">) {
  const { open, toggle, contentId } = useCollapsible();
  return (
    <button
      type="button"
      aria-expanded={open}
      aria-controls={contentId}
      onClick={toggle}
      className={cn(
        "flex w-full items-center gap-2 px-2 py-1 text-left text-xs font-medium",
        "hover:bg-muted/50 transition-colors",
        className,
      )}
      {...props}
    >
      <ChevronRight
        aria-hidden
        className={cn(
          "text-muted-foreground size-4 shrink-0 transition-transform",
          open && "rotate-90",
        )}
      />
      {children}
    </button>
  );
}

export function CollapsibleContent({
  className,
  children,
  ...props
}: ComponentProps<"div"> & { children: ReactNode }) {
  const { open, contentId } = useCollapsible();
  if (!open) return null;
  return (
    <div
      id={contentId}
      className={cn("border-t px-2 py-1.5 text-xs", className)}
      {...props}
    >
      {children}
    </div>
  );
}
