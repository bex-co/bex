import { useId, type CSSProperties } from "react";
import {
  Copy,
  Ellipsis,
  ExternalLink,
  SquareTerminal,
  TextSearch,
} from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/common/components/ui/tooltip";
import { cn } from "@/common/lib/utils/utils.ts";
import type { LogLine } from "../types";

type Translate = (key: string) => string;

/** Where the row menu's SSH into instance goes for a line: the Shell on its
 *  instance, or `null` when that instance is not running (the item is then
 *  disabled with Render's reason). */
export type LogLineShellTarget = { href: string | null };

/**
 * The row menu's items beyond Copy log line, as an explorer offers them
 * (w3/m528). Each is absent where the explorer has none: View in context
 * only while a search or filter narrows it, SSH into instance only on a
 * service whose Shell can open.
 */
export interface LogRowMenuItems {
  /** View in context: the explorer, unfiltered, 10 minutes either side of
   *  the line, with the line marked; undefined for a line without a time. */
  contextHref?: (line: LogLine) => string | undefined;
  /** SSH into instance: null for a line that offers none (no instance). */
  shell?: (line: LogLine) => LogLineShellTarget | null;
}

/** Marks the menu's content, so the list keeps its row cursor while the
 *  keyboard's focus is in the menu. */
export const LOG_ROW_MENU_ATTRIBUTE = "data-log-row-menu";

/**
 * Render's row menu (`LogLineOverflowMenu`): a "…" button at the row's top
 * right with Copy log line, View in context and SSH into instance. A blank
 * line with neither of the last two has no menu. The list mounts it only on
 * the hovered row, the keyboard's row, or the row whose menu is open.
 */
export function LogRowMenu({
  line,
  text,
  items,
  open,
  onOpenChange,
  onCopy,
  onCloseAutoFocus,
  style,
  className,
  t,
}: {
  line: LogLine;
  /** Copy log line's text (`logLineText`). */
  text: string;
  items: LogRowMenuItems | undefined;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCopy: (text: string) => void;
  onCloseAutoFocus: (event: Event) => void;
  style?: CSSProperties;
  className?: string;
  t: Translate;
}) {
  const reasonId = useId();
  const blank = text.trim() === "";
  const contextHref = items?.contextHref?.(line);
  const shell = items?.shell?.(line) ?? null;
  if (blank && !contextHref && !shell) return null;
  return (
    <div className={cn("absolute top-0 z-10", className)} style={style}>
      <DropdownMenu open={open} onOpenChange={onOpenChange}>
        <DropdownMenuTrigger asChild>
          <button
            type="button"
            // Out of the Tab order, like the row's instance tag: the focused
            // list's Shift+F10 opens the menu of the line under its cursor.
            tabIndex={-1}
            data-touch-exempt=""
            aria-label={t("logs.lineMenu")}
            title={t("logs.lineMenu")}
            className="relative flex size-6 items-center justify-center rounded-sm border bg-background text-muted-foreground shadow-xs hover:bg-muted hover:text-foreground data-[state=open]:bg-muted data-[state=open]:text-foreground max-lg:after:absolute max-lg:after:-inset-2.5 max-lg:after:content-['']"
          >
            <Ellipsis className="size-3.5" />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent
          align="end"
          className="w-56 font-sans"
          {...{ [LOG_ROW_MENU_ATTRIBUTE]: "" }}
          onCloseAutoFocus={onCloseAutoFocus}
        >
          <DropdownMenuItem disabled={blank} onSelect={() => onCopy(text)}>
            <Copy />
            {t("logs.copyLogLine")}
          </DropdownMenuItem>
          {contextHref ? (
            <DropdownMenuItem asChild>
              <a href={contextHref} target="_blank" rel="noopener">
                <TextSearch />
                <span className="flex-1">{t("logs.viewInContext")}</span>
                <ExternalLink aria-hidden />
              </a>
            </DropdownMenuItem>
          ) : null}
          {shell?.href ? (
            <DropdownMenuItem asChild>
              <a href={shell.href} target="_blank" rel="noopener">
                <SquareTerminal />
                <span className="flex-1">{t("logs.sshIntoInstance")}</span>
                <ExternalLink aria-hidden />
              </a>
            </DropdownMenuItem>
          ) : shell ? (
            // A disabled item takes no pointer events, so its reason hangs
            // on a wrapper, as Render's tooltip does.
            <Tooltip>
              <TooltipTrigger asChild>
                <div>
                  <DropdownMenuItem disabled aria-describedby={reasonId}>
                    <SquareTerminal />
                    <span className="flex-1">{t("logs.sshIntoInstance")}</span>
                    <ExternalLink aria-hidden />
                  </DropdownMenuItem>
                  <span id={reasonId} className="sr-only">
                    {t("logs.sshInstanceNotRunning")}
                  </span>
                </div>
              </TooltipTrigger>
              <TooltipContent className="max-w-64">
                {t("logs.sshInstanceNotRunning")}
              </TooltipContent>
            </Tooltip>
          ) : null}
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}
