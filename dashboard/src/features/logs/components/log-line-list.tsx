import { useLayoutEffect, useRef, useState } from "react";
import { ArrowDown } from "lucide-react";
import { Button } from "@/common/components/ui/button.tsx";
import { useTranslations } from "@/common/hooks/use-translations";
import type { LogLine } from "../types";

// How close to the bottom (px) still counts as "pinned" — a small slack so a
// sub-pixel scroll position or a wrapped final line doesn't unpin autoscroll.
const PIN_THRESHOLD = 24;

// Height of the scroll viewport; the list fills it and scrolls internally.
const VIEWPORT_HEIGHT = 520;

interface LogLineListProps {
  lines: LogLine[];
}

/**
 * The monospace log line list (Render's layout: timestamp · [instance] ·
 * message, newest at the bottom). Autoscrolls to the newest line while the user
 * is pinned to the bottom; scrolling up releases the pin (so reading history
 * isn't yanked away) and surfaces a "jump to latest" affordance.
 */
export function LogLineList({ lines }: LogLineListProps) {
  const { t } = useTranslations();
  const viewportRef = useRef<HTMLDivElement>(null);
  const [pinned, setPinned] = useState(true);

  const scrollToBottom = () => {
    const el = viewportRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  };

  // Recompute the pin from the scroll position; releasing it while the user
  // reads up, restoring it the moment they return to the bottom.
  const onScroll = () => {
    const el = viewportRef.current;
    if (!el) return;
    const distance = el.scrollHeight - el.scrollTop - el.clientHeight;
    setPinned(distance <= PIN_THRESHOLD);
  };

  // useLayoutEffect so the scroll lands before paint — no visible jump as new
  // lines append.
  useLayoutEffect(() => {
    if (pinned) scrollToBottom();
  }, [lines, pinned]);

  return (
    <div className="relative">
      <div
        ref={viewportRef}
        onScroll={onScroll}
        style={{ height: VIEWPORT_HEIGHT }}
        className="overflow-auto rounded-md border bg-muted/30 font-mono text-xs leading-relaxed"
      >
        <div className="min-w-full p-3">
          {lines.map((line) => (
            <div
              key={line.key}
              className="flex gap-3 whitespace-pre-wrap break-words px-1 py-0.5 hover:bg-muted/60"
            >
              <span className="shrink-0 tabular-nums text-muted-foreground">
                {line.time}
              </span>
              {line.instance ? (
                <span className="shrink-0 text-muted-foreground/70">
                  [{line.instance}]
                </span>
              ) : null}
              <span className="min-w-0 flex-1 text-foreground">
                {line.message}
              </span>
            </div>
          ))}
        </div>
      </div>

      {!pinned ? (
        <Button
          size="sm"
          variant="secondary"
          className="absolute bottom-3 left-1/2 -translate-x-1/2 shadow-md"
          onClick={() => {
            setPinned(true);
            scrollToBottom();
          }}
        >
          <ArrowDown className="h-4 w-4" />
          {t("logs.jumpToLatest")}
        </Button>
      ) : null}
    </div>
  );
}
