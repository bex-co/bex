import type { ReactNode } from "react";
import { Link } from "@tanstack/react-router";
import {
  Copy,
  Ellipsis,
  Eraser,
  Maximize2,
  Minimize2,
  Settings,
  WrapText,
} from "lucide-react";
import { Button } from "@/common/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "@/common/components/ui/dropdown-menu";
import { useIsMac } from "@/common/hooks/use-is-mac";
import { useIsMobile } from "@/common/hooks/use-mobile";
import { useTranslations } from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils.ts";
import type { LogExplorer } from "../hooks/use-log-explorer";
import { LOG_DISPLAYS, type LogDisplay } from "../lib/log-display";
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

const DISPLAY_LABEL_KEYS = {
  "expand-all": "logs.expandEverything",
  "expand-errors": "logs.expandErrors",
  "collapse-all": "logs.collapseEverything",
} as const satisfies Record<LogDisplay, string>;

/**
 * Render's Options (…) menu: Copy logs (`⌘/Ctrl ⇧ C`), Order by,
 * Display (w3/088), and Theme Settings, a link to account Settings ▸
 * Appearance (w3/m528). Display is Render's submenu, `Display: <choice>`
 * over Display settings' Expand everything, Expand errors and Collapse
 * everything; on a phone, where a side flyout has no room, its choices sit
 * in the menu itself, as the account menu's theme does (w4/302).
 * `children` are an explorer's own options, listed first (the deploy page's
 * range and wrap/timestamp toggles), and `phoneMaximizeItem` adds Maximize
 * as an item on a phone, for a toolbar with no room there for its own
 * button.
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
  const isMobile = useIsMobile();
  const { order, setOrder, display, setDisplay, maximized, setMaximized } =
    explorer;
  const displayChoices = (
    <DropdownMenuRadioGroup
      value={display}
      onValueChange={(value) => setDisplay(value as LogDisplay)}
    >
      {LOG_DISPLAYS.map((choice) => (
        <DropdownMenuRadioItem key={choice} value={choice}>
          {t(DISPLAY_LABEL_KEYS[choice])}
        </DropdownMenuRadioItem>
      ))}
    </DropdownMenuRadioGroup>
  );
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
      <DropdownMenuContent align="end" className="w-70">
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
        {isMobile ? (
          <DropdownMenuGroup aria-label={t("logs.displaySettings")}>
            <DropdownMenuLabel>{t("logs.displaySettings")}</DropdownMenuLabel>
            {displayChoices}
          </DropdownMenuGroup>
        ) : (
          <DropdownMenuSub>
            <DropdownMenuSubTrigger>
              <WrapText />
              <span className="min-w-0 truncate">
                {t("logs.display")}{" "}
                <span className="font-semibold">
                  {t(DISPLAY_LABEL_KEYS[display])}
                </span>
              </span>
            </DropdownMenuSubTrigger>
            <DropdownMenuSubContent>
              <DropdownMenuLabel>{t("logs.displaySettings")}</DropdownMenuLabel>
              {displayChoices}
            </DropdownMenuSubContent>
          </DropdownMenuSub>
        )}
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
        <DropdownMenuSeparator />
        <DropdownMenuItem asChild>
          <Link to="/settings" hash="appearance">
            <Settings />
            {t("logs.themeSettings")}
          </Link>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
