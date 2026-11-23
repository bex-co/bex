import { useEffect, useId, useRef } from "react";
import {
  Copy,
  ExternalLink,
  Filter,
  ScrollText,
  SquareTerminal,
  X,
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
import { useTranslations } from "@/common/hooks/use-translations";

/**
 * Actions for one charted service instance (w4/717; Render's chart action card
 * in `metrics-*.js` / `src-*.js`): Filter to instance, SSH into instance (with
 * the log row's running/stopped rules), View instance logs and Copy instance ID.
 */
export interface ChartInstanceActions {
  /** Shell target for this instance, or undefined when the service's Shell
   *  cannot open (Free, suspended, cron, or SSH gateway unconfigured). */
  shell?: (instanceId: string) => { href: string | null };
  /** Logs tab URL pre-filtered to this instance. */
  logsHref: (instanceId: string) => string;
  /** Whether this instance is currently selected in the Instances filter. */
  isFiltered: (instanceId: string) => boolean;
  /** Toggles filtering the card to this instance; omitted when there is only
   *  one instance and no active filter. */
  onToggleFilter?: (instanceId: string) => void;
  /** Copies the full instance ID to the clipboard. */
  onCopyId: (instanceId: string) => void;
  /** Compact display name for the header and tooltip hint. */
  formatLabel?: (instanceId: string) => string;
}

const MENU_ITEM_CLASS =
  "flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-left text-xs text-popover-foreground transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:bg-accent focus-visible:text-accent-foreground focus-visible:outline-none";

const MENU_ITEM_DISABLED_CLASS =
  "flex w-full cursor-not-allowed items-center gap-2 rounded-sm px-2 py-1.5 text-left text-xs text-muted-foreground opacity-50";

/**
 * Render's floating chart instance action card: opens when the reader clicks a
 * Memory or CPU series (or presses Enter on a focused chart point). Closes on
 * Escape, an outside click, or its Close button.
 */
export function ChartInstanceMenu({
  instanceId,
  color,
  xPercent,
  yPx,
  flipLeft,
  actions,
  onClose,
}: {
  instanceId: string;
  color: string;
  /** Horizontal anchor inside the chart container, 0..100. */
  xPercent: number;
  /** Vertical anchor inside the chart container, in pixels. */
  yPx: number;
  /** Flip to the left of the point when anchored in the right half. */
  flipLeft: boolean;
  actions: ChartInstanceActions;
  onClose: () => void;
}) {
  const { t } = useTranslations();
  const reasonId = useId();
  const menuRef = useRef<HTMLElement>(null);

  useEffect(() => {
    const onMouseDown = (event: MouseEvent) => {
      if (
        event.target instanceof Node &&
        !menuRef.current?.contains(event.target)
      ) {
        onClose();
      }
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.stopPropagation();
        event.preventDefault();
        onClose();
      }
    };
    document.addEventListener("mousedown", onMouseDown);
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("mousedown", onMouseDown);
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [onClose]);

  const displayName = actions.formatLabel?.(instanceId) ?? instanceId;
  const filtered = actions.isFiltered(instanceId);
  const shell = actions.shell ? actions.shell(instanceId) : null;
  const logsHref = actions.logsHref(instanceId);

  return (
    <section
      ref={menuRef}
      aria-label={t("metrics.instanceActionsMenu", { instance: displayName })}
      className="absolute z-20 w-60 max-w-[calc(100%-0.5rem)] rounded-md border bg-popover text-popover-foreground shadow-md"
      style={{
        top: `${Math.max(4, yPx)}px`,
        ...(flipLeft
          ? { right: `${Math.max(0, 100 - xPercent)}%` }
          : { left: `${Math.min(100, Math.max(0, xPercent))}%` }),
      }}
    >
      <header className="flex items-center justify-between gap-2 border-b bg-muted/40 py-1.5 pr-1.5 pl-3">
        <div className="flex min-w-0 items-center gap-2 text-xs font-medium">
          <span
            className="inline-block size-2.5 shrink-0 rounded-full"
            style={{ backgroundColor: color }}
            aria-hidden="true"
          />
          <span className="truncate font-mono" title={instanceId}>
            {displayName}
          </span>
        </div>
        <button
          type="button"
          onClick={onClose}
          aria-label={t("common.close")}
          className="flex size-6 shrink-0 items-center justify-center rounded-sm text-muted-foreground hover:bg-accent hover:text-accent-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
        >
          <X className="size-3.5" aria-hidden="true" />
        </button>
      </header>
      <ul className="space-y-0.5 p-1">
        {actions.onToggleFilter ? (
          <li>
            <button
              type="button"
              className={MENU_ITEM_CLASS}
              onClick={() => {
                actions.onToggleFilter?.(instanceId);
                onClose();
              }}
            >
              <Filter
                className="size-3.5 shrink-0 text-muted-foreground"
                aria-hidden="true"
              />
              <span className="flex-1">
                {filtered
                  ? t("metrics.resetInstanceFilter")
                  : t("metrics.filterToInstance")}
              </span>
            </button>
          </li>
        ) : null}
        {shell?.href ? (
          <li>
            <a
              href={shell.href}
              target="_blank"
              rel="noopener"
              className={MENU_ITEM_CLASS}
            >
              <SquareTerminal
                className="size-3.5 shrink-0 text-muted-foreground"
                aria-hidden="true"
              />
              <span className="flex-1">{t("logs.sshIntoInstance")}</span>
              <ExternalLink
                className="size-3.5 shrink-0 text-muted-foreground"
                aria-hidden="true"
              />
            </a>
          </li>
        ) : shell ? (
          <li>
            <Tooltip>
              <TooltipTrigger asChild>
                <div>
                  <button
                    type="button"
                    disabled
                    aria-describedby={reasonId}
                    className={MENU_ITEM_DISABLED_CLASS}
                  >
                    <SquareTerminal
                      className="size-3.5 shrink-0"
                      aria-hidden="true"
                    />
                    <span className="flex-1">{t("logs.sshIntoInstance")}</span>
                    <ExternalLink
                      className="size-3.5 shrink-0"
                      aria-hidden="true"
                    />
                  </button>
                  <span id={reasonId} className="sr-only">
                    {t("logs.sshInstanceNotRunning")}
                  </span>
                </div>
              </TooltipTrigger>
              <TooltipContent className="max-w-64">
                {t("logs.sshInstanceNotRunning")}
              </TooltipContent>
            </Tooltip>
          </li>
        ) : null}
        <li>
          <a
            href={logsHref}
            target="_blank"
            rel="noopener"
            className={MENU_ITEM_CLASS}
          >
            <ScrollText
              className="size-3.5 shrink-0 text-muted-foreground"
              aria-hidden="true"
            />
            <span className="flex-1">{t("metrics.viewInstanceLogs")}</span>
            <ExternalLink
              className="size-3.5 shrink-0 text-muted-foreground"
              aria-hidden="true"
            />
          </a>
        </li>
        <li>
          <button
            type="button"
            className={MENU_ITEM_CLASS}
            onClick={() => {
              actions.onCopyId(instanceId);
              onClose();
            }}
          >
            <Copy
              className="size-3.5 shrink-0 text-muted-foreground"
              aria-hidden="true"
            />
            <span className="flex-1">{t("metrics.copyInstanceId")}</span>
          </button>
        </li>
      </ul>
    </section>
  );
}

/**
 * Interactive legend pill for a per-instance chart series: opens the same
 * instance actions via a Radix DropdownMenu so keyboard and touch users can
 * reach SSH into instance without hovering a 2px line.
 */
export function ChartInstanceLegendTrigger({
  instanceId,
  color,
  actions,
}: {
  instanceId: string;
  color: string;
  actions: ChartInstanceActions;
}) {
  const { t } = useTranslations();
  const reasonId = useId();
  const displayName = actions.formatLabel?.(instanceId) ?? instanceId;
  const filtered = actions.isFiltered(instanceId);
  const shell = actions.shell ? actions.shell(instanceId) : null;
  const logsHref = actions.logsHref(instanceId);

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          title={instanceId}
          aria-label={t("metrics.instanceActionsMenu", {
            instance: displayName,
          })}
          className="flex cursor-pointer items-center gap-1.5 rounded-sm px-1 -mx-1 py-0.5 text-xs text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none data-[state=open]:bg-muted data-[state=open]:text-foreground"
        >
          <span
            className="inline-block size-2 rounded-full"
            style={{ backgroundColor: color }}
            aria-hidden="true"
          />
          <span>{displayName}</span>
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-56 font-sans">
        {actions.onToggleFilter ? (
          <DropdownMenuItem
            onSelect={() => actions.onToggleFilter?.(instanceId)}
          >
            <Filter />
            <span className="flex-1">
              {filtered
                ? t("metrics.resetInstanceFilter")
                : t("metrics.filterToInstance")}
            </span>
          </DropdownMenuItem>
        ) : null}
        {shell?.href ? (
          <DropdownMenuItem asChild>
            <a href={shell.href} target="_blank" rel="noopener">
              <SquareTerminal />
              <span className="flex-1">{t("logs.sshIntoInstance")}</span>
              <ExternalLink aria-hidden="true" />
            </a>
          </DropdownMenuItem>
        ) : shell ? (
          <Tooltip>
            <TooltipTrigger asChild>
              <div>
                <DropdownMenuItem disabled aria-describedby={reasonId}>
                  <SquareTerminal />
                  <span className="flex-1">{t("logs.sshIntoInstance")}</span>
                  <ExternalLink aria-hidden="true" />
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
        <DropdownMenuItem asChild>
          <a href={logsHref} target="_blank" rel="noopener">
            <ScrollText />
            <span className="flex-1">{t("metrics.viewInstanceLogs")}</span>
            <ExternalLink aria-hidden="true" />
          </a>
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={() => actions.onCopyId(instanceId)}>
          <Copy />
          <span className="flex-1">{t("metrics.copyInstanceId")}</span>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
