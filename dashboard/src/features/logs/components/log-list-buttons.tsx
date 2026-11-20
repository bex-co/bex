import { MoveDown, MoveUp, RefreshCw, X } from "lucide-react";
import { Button } from "@/common/components/ui/button.tsx";
import { useTranslations } from "@/common/hooks/use-translations";
import { cn } from "@/common/lib/utils/utils.ts";
import { heldLinesCount } from "../lib/log-rows";

// The floating buttons over a log list (w3/m528), placed as Render's
// `LogViewerFloatingButton` places them: at the newest edge (the bottom
// ascending, the top descending), Return at the panel's right and the
// new-log buttons centred.
const EDGE = { asc: "bottom-4", desc: "top-4" } as const;
const CENTRED = "absolute left-1/2 z-20 -translate-x-1/2";

/** Render's Return to Bottom (Return to Top descending): back to the newest
 *  line, icon-only at the panel's right edge. */
export function ReturnButton({
  descending,
  onClick,
}: {
  descending: boolean;
  onClick: () => void;
}) {
  const { t } = useTranslations();
  const label = descending ? t("logs.returnToTop") : t("logs.returnToBottom");
  const Icon = descending ? MoveUp : MoveDown;
  return (
    <Button
      size="icon-sm"
      variant="secondary"
      aria-label={label}
      title={label}
      className={cn(
        "absolute right-6 z-20 shadow-md",
        EDGE[descending ? "desc" : "asc"],
      )}
      onClick={onClick}
    >
      <Icon className="size-4" />
    </Button>
  );
}

/** Render's `BufferedLogsButton`: the lines a scrolled-away live tail held
 *  back, released by a click. */
export function HeldLinesButton({
  count,
  descending,
  onClick,
}: {
  count: number;
  descending: boolean;
  onClick: () => void;
}) {
  const { t } = useTranslations();
  const shown = heldLinesCount(count);
  const Icon = descending ? MoveUp : MoveDown;
  return (
    <Button
      size="sm"
      variant="secondary"
      className={cn(CENTRED, "shadow-md", EDGE[descending ? "desc" : "asc"])}
      onClick={onClick}
    >
      <Icon className="size-4" />
      {shown === null
        ? t("logs.newLogs")
        : t("logs.newLogsCount", { count: shown })}
    </Button>
  );
}

/** Render's `NewLogsAvailableButton`: a paused explorer's tail has a new
 *  line. The button turns Live on; Dismiss hides it. */
export function NewLogsInLiveTailButton({
  descending,
  onSwitch,
  onDismiss,
}: {
  descending: boolean;
  onSwitch: () => void;
  onDismiss: () => void;
}) {
  const { t } = useTranslations();
  return (
    <div
      className={cn(
        CENTRED,
        "flex items-stretch rounded-md shadow-md backdrop-blur-md",
        EDGE[descending ? "desc" : "asc"],
      )}
    >
      <Button
        size="sm"
        variant="secondary"
        className="rounded-r-none"
        title={t("logs.switchToLiveTail")}
        onClick={onSwitch}
      >
        <RefreshCw className="size-4" />
        {t("logs.newLogsInLiveTail")}
      </Button>
      <Button
        size="icon-sm"
        variant="secondary"
        className="rounded-l-none border-l border-border/60"
        aria-label={t("logs.dismiss")}
        title={t("logs.dismiss")}
        onClick={onDismiss}
      >
        <X className="size-4" />
      </Button>
    </div>
  );
}
