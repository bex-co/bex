import type { ComponentProps } from "react";
import { cn } from "@/common/lib/utils/utils.ts";
import { useLogThemeClass } from "@/common/providers/theme-provider/hooks";
import { ThemeScope } from "@/common/providers/theme-provider/theme-scope";
import {
  LOG_MAXIMIZED_CLASS,
  LOG_THEMED_FRAME_CLASS,
} from "./log-toolbar-layout";

/**
 * Every log explorer's root (w3/m531): a service's Logs tab, a Postgres or
 * Key Value Logs tab, a deploy's log and a project's Application and Request
 * Logs views. Under Log Explorer Theme Light or Dark it keeps that theme,
 * in its high-contrast twin while High Contrast Mode is on, whatever the
 * dashboard's, and the menus and tooltips it opens follow it; under Match
 * Dashboard it follows the page. `maximized` is Render's Maximize.
 */
export function LogThemeFrame({
  maximized,
  className,
  ...props
}: ComponentProps<"div"> & { maximized: boolean }) {
  const theme = useLogThemeClass();
  return (
    <ThemeScope
      theme={theme}
      data-log-explorer=""
      className={cn(
        theme && LOG_THEMED_FRAME_CLASS,
        className,
        maximized && LOG_MAXIMIZED_CLASS,
      )}
      {...props}
    />
  );
}
