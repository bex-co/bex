import type { ReactNode } from "react";
import { Copy, Ellipsis, Eraser, Maximize2, Minimize2 } from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu";
import { useIsMac } from "@/common/hooks/use-is-mac";
import { useTranslations } from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils.ts";
import type { LogExplorer } from "../hooks/use-log-explorer";
import type { LogOrder } from "../lib/log-rows";
import type { LogPanelFill } from "../hooks/use-log-panel-height";
import { LogPanelState } from "./log-panel-state";
import { LOG_ICON_BUTTON_CLASS } from "./log-toolbar-layout";

/**
 * An explorer's Maximize and Options buttons, in its toolbar's own wrapper.
 * `phoneMaximize` moves Maximize into Options on a phone, for a toolbar with
 * no room there for its own button.
 */
export function LogExplorerActions({
  explorer,
  canCopy,
  phoneMaximize = false,
  className,
}: {
  explorer: LogExplorer;
  /** False while no line is shown. */
  canCopy: boolean;
  phoneMaximize?: boolean;
  className: string;
}) {
  return (
    <div className={className}>
      <MaximizeButton
        explorer={explorer}
        className={cn(LOG_ICON_BUTTON_CLASS, phoneMaximize && "max-sm:hidden")}
      />
      <LogOptionsMenu
        explorer={explorer}
        canCopy={canCopy}
        phoneMaximizeItem={phoneMaximize}
        triggerClassName={LOG_ICON_BUTTON_CLASS}
      />
    </div>
  );
}

/** Where a live tail's lines were before `Shift+L` cleared them. */
export function LogClearedState({
  fillViewport,
  onRestore,
}: {
  fillViewport: LogPanelFill;
  onRestore: () => void;
}) {
  const { t } = useTranslations();
  return (
    <LogPanelState
      fillViewport={fillViewport}
      icon={Eraser}
      title={t("logs.clearedTitle")}
      body={t("logs.clearedBody")}
      action={
        <Button variant="outline" size="sm" onClick={onRestore}>
          {t("logs.clearedRestore")}
        </Button>
      }
    />
  );
}

/** Render's Maximize button: full screen, or back (`M`, or `Esc` to leave). */
export function MaximizeButton({
  explorer,
  className,
}: {
  explorer: LogExplorer;
  className?: string;
}) {
  const { t } = useTranslations();
  const { maximized, setMaximized } = explorer;
  return (
    <Button
      variant="outline"
      size="icon-sm"
      className={className}
      aria-label={maximized ? t("logs.minimize") : t("logs.maximize")}
      aria-keyshortcuts="M"
      title={maximized ? t("logs.minimizeHint") : t("logs.maximizeHint")}
      onClick={() => setMaximized((value) => !value)}
    >
      {maximized ? (
        <Minimize2 className="h-4 w-4" />
      ) : (
        <Maximize2 className="h-4 w-4" />
      )}
    </Button>
  );
}

/**
 * Render's Options (…) menu: Copy logs (`⌘/Ctrl ⇧ C`), Order by, and
 * Display ▸ Expand everything. `children` are an explorer's own options,
 * listed first (the deploy page's range and wrap/timestamp toggles), and
 * `phoneMaximizeItem` adds Maximize as an item on a phone, for a toolbar
 * with no room there for its own button.
 */
export function LogOptionsMenu({
  explorer,
  canCopy,
  phoneMaximizeItem = false,
  triggerClassName,
  children,
}: {
  explorer: LogExplorer;
  /** False while no line is shown. */
  canCopy: boolean;
  phoneMaximizeItem?: boolean;
  triggerClassName?: string;
  children?: ReactNode;
}) {
  const { t } = useTranslations();
  const shortcut = useIsMac() ? "⌘⇧C" : "Ctrl+Shift+C";
  const { order, setOrder, expandAll, setExpandAll, maximized, setMaximized } =
    explorer;
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="outline"
          size="icon-sm"
          className={triggerClassName}
          aria-label={t("logs.options")}
          title={t("logs.options")}
        >
          <Ellipsis className="h-4 w-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-56">
        {children}
        {children ? <DropdownMenuSeparator /> : null}
        <DropdownMenuItem disabled={!canCopy} onSelect={explorer.copy}>
          <Copy />
          {t("logs.copyLogs")}
          <DropdownMenuShortcut>{shortcut}</DropdownMenuShortcut>
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuLabel>{t("logs.orderBy")}</DropdownMenuLabel>
        <DropdownMenuRadioGroup
          value={order}
          onValueChange={(value) => setOrder(value as LogOrder)}
        >
          <DropdownMenuRadioItem value="asc">
            {t("logs.orderAscending")}
          </DropdownMenuRadioItem>
          <DropdownMenuRadioItem value="desc">
            {t("logs.orderDescending")}
          </DropdownMenuRadioItem>
        </DropdownMenuRadioGroup>
        <DropdownMenuSeparator />
        <DropdownMenuLabel>{t("logs.display")}</DropdownMenuLabel>
        <DropdownMenuCheckboxItem
          checked={expandAll}
          onCheckedChange={(value) => setExpandAll(value === true)}
        >
          {t("logs.expandEverything")}
        </DropdownMenuCheckboxItem>
        {phoneMaximizeItem ? (
          <>
            <DropdownMenuSeparator className="sm:hidden" />
            <DropdownMenuItem
              className="sm:hidden"
              onSelect={() => setMaximized((value) => !value)}
            >
              {maximized ? <Minimize2 /> : <Maximize2 />}
              {maximized ? t("logs.minimize") : t("logs.maximize")}
            </DropdownMenuItem>
          </>
        ) : null}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
