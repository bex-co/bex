import { useState, type ReactNode } from "react";
import { Check, Copy } from "lucide-react";

import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/common/components/ui/tooltip";
import { useCopyToClipboard } from "@/common/hooks/use-copy-to-clipboard";
import { useTranslations } from "@/common/hooks/use-translations";
import { formatInstantDetails, formatInstantGap } from "@/common/lib/format";
import { cn } from "@/common/lib/utils/utils";

interface InstantTooltipProps {
  /** RFC3339/ISO instant behind the visible text. */
  value: string;
  /** The visible rendering — usually elapsed time, "Deployed 2 hours ago". */
  children: ReactNode;
  className?: string;
  /**
   * Hold the tooltip open (a log list's keyboard row cursor on this row,
   * w3/m509); hovering or focusing still opens it while this is false.
   */
  open?: boolean;
  /** Mount already open, as hovered (a log row's time mounts its tooltip
   *  under the pointer); the pointer leaving closes it as usual. */
  initiallyOpen?: boolean;
  /**
   * The trigger's Tab stop. A log row's time is left out of the Tab order
   * (-1), as one stop per line made the log hundreds of stops long; the
   * list's row cursor opens it instead.
   */
  tabIndex?: number;
  /** The line before a log line's, in time order: the card's From previous
   *  (w4/716). */
  previous?: string;
  /** When a deploy's log began (the deploy's creation): a log line's From
   *  start, on a deploy's log only (w4/716). */
  startedAt?: string;
}

/**
 * Render's timestamp card (`TimestampTooltipContent`, w4/716): hovering (or
 * focusing) a time reveals the exact instant — the viewer's zone and clock,
 * UTC unless that is the viewer's zone, and Unix milliseconds — and on a log
 * line the gap since the line before (From previous) and since a deploy's
 * start (From start). Each value has its own copy button, writing Render's
 * string, and stays selectable. Hoverable content (Radix's default) is what
 * lets the pointer travel into the tooltip to reach them without closing it.
 *
 * The trigger is a `<time>` carrying the machine-readable instant. The visible
 * text is the caller's and may be elapsed-time text, which drifts between the
 * SSR pass and hydration — the same divergence `RelativeAge` suppresses
 * (w6/m102). The detail rows only mount when the tooltip opens, so the local
 * reading is never computed into the hydrated markup (the w6/m107 rule).
 */
export function InstantTooltip({
  value,
  children,
  className,
  open,
  initiallyOpen = false,
  tabIndex = 0,
  previous,
  startedAt,
}: InstantTooltipProps) {
  const [triggerOpen, setTriggerOpen] = useState(initiallyOpen);
  return (
    <Tooltip
      {...(open === undefined
        ? {}
        : { open: open || triggerOpen, onOpenChange: setTriggerOpen })}
    >
      <TooltipTrigger asChild>
        <time
          dateTime={value}
          // Focusable by default, so keyboard users can reach its text.
          tabIndex={tabIndex}
          className={cn(
            "cursor-default rounded-sm underline decoration-dotted underline-offset-2 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none",
            className,
          )}
          suppressHydrationWarning
        >
          {children}
        </time>
      </TooltipTrigger>
      <TooltipContent className="select-text px-3 py-2 text-left">
        <InstantDetails
          value={value}
          previous={previous}
          startedAt={startedAt}
        />
      </TooltipContent>
    </Tooltip>
  );
}

interface InstantRowProps {
  label: string;
  /** What the row shows. */
  text: string;
  /** What its copy button writes (Render's string for the row). */
  copy: string;
}

function InstantDetails({
  value,
  previous,
  startedAt,
}: {
  value: string;
  previous?: string;
  startedAt?: string;
}) {
  const { t, i18n } = useTranslations();
  const language = i18n.language;
  const details = formatInstantDetails(value, language);
  if (!details) return null;
  // Render's rows, in its order; each gap only where the card has one.
  const rows: InstantRowProps[] = [
    {
      label: details.zone ?? t("common.timeLocal"),
      text: details.local,
      copy: details.localCopy,
    },
  ];
  if (details.utc !== null) {
    rows.push({
      label: t("common.timeUtc"),
      text: details.utc,
      copy: details.utcCopy,
    });
  }
  rows.push({
    label: t("common.timeUnix"),
    text: details.unix,
    copy: details.unix,
  });
  const gaps: [string, string | undefined][] = [
    [t("common.timeFromPrevious"), previous],
    [t("common.timeFromStart"), startedAt],
  ];
  for (const [label, since] of gaps) {
    const gap = since ? formatInstantGap(value, since, language) : null;
    if (gap) rows.push({ label, text: gap, copy: gap });
  }
  return (
    <dl className="grid grid-cols-[auto_1fr] items-center gap-x-5 gap-y-1 font-mono whitespace-nowrap">
      {rows.map((row) => (
        <InstantRow key={row.label} {...row} />
      ))}
    </dl>
  );
}

function InstantRow({ label, text, copy }: InstantRowProps) {
  const { t } = useTranslations();
  const { copied, copy: write } = useCopyToClipboard({
    successText: t("common.timeCopied"),
    errorText: t("common.timeCopyFailed"),
  });
  return (
    <>
      <dt className="text-background/70">
        {t("common.timeRowLabel", { label })}
      </dt>
      <dd className="flex items-center justify-between gap-x-2">
        <span className="tabular-nums select-all">{text}</span>
        <button
          type="button"
          aria-label={t("common.copyField", { field: label })}
          onClick={() => void write(copy)}
          // A 28px hit area around the 20px button, within the card's rows.
          className="relative -my-0.5 inline-flex size-5 shrink-0 items-center justify-center rounded-sm text-background/70 after:absolute after:-inset-1 after:content-[''] hover:bg-background/15 hover:text-background focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
        >
          {copied ? (
            <Check aria-hidden className="size-3" />
          ) : (
            <Copy aria-hidden className="size-3" />
          )}
        </button>
      </dd>
    </>
  );
}
