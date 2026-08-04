import { useEffect, useRef, type ComponentProps } from "react";
import { cn } from "@/common/lib/utils/utils.ts";

// The scrolling transcript container (AI Elements `Conversation`). Upstream
// uses `use-stick-to-bottom`; a ref + effect that pins to the bottom whenever
// the child count grows keeps the live tail in view without the extra
// dependency, and degrades to a plain scroll region under SSR (the effect
// never runs on the server).

export function Conversation({ className, ...props }: ComponentProps<"div">) {
  return (
    <div
      className={cn("relative flex-1 overflow-y-auto", className)}
      role="log"
      aria-live="polite"
      {...props}
    />
  );
}

export function ConversationContent({
  className,
  children,
  ...props
}: ComponentProps<"div">) {
  const endRef = useRef<HTMLDivElement>(null);

  // Pin to the newest part as the stream appends. `children` identity changes
  // on every render useChat triggers, so scrolling on each is intentional; the
  // browser no-ops when already at the bottom.
  useEffect(() => {
    endRef.current?.scrollIntoView({ block: "end" });
  }, [children]);

  return (
    <div className={cn("flex flex-col gap-4 p-4", className)} {...props}>
      {children}
      <div ref={endRef} aria-hidden />
    </div>
  );
}
